// Copyright 2021 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checkpoint

import (
	"context"
	"fmt"
	"github.com/matrixorigin/matrixone/pkg/container/batch"
	"sync"
	"time"

	"github.com/matrixorigin/matrixone/pkg/container/types"
	"github.com/matrixorigin/matrixone/pkg/logutil"
	"github.com/matrixorigin/matrixone/pkg/objectio"
	"github.com/matrixorigin/matrixone/pkg/pb/api"
	"github.com/matrixorigin/matrixone/pkg/vm/engine/tae/blockio"
	"github.com/matrixorigin/matrixone/pkg/vm/engine/tae/common"
	"github.com/matrixorigin/matrixone/pkg/vm/engine/tae/logtail"
)

type CheckpointEntry struct {
	sync.RWMutex
	start, end types.TS
	state      State
	entryType  EntryType
	cnLocation objectio.Location
	dnLocation objectio.Location
	lastPrint  time.Time
	version    uint32
}

func NewCheckpointEntry(start, end types.TS, typ EntryType) *CheckpointEntry {
	return &CheckpointEntry{
		start:     start,
		end:       end,
		state:     ST_Pending,
		entryType: typ,
		lastPrint: time.Now(),
		version:   logtail.CheckpointCurrentVersion,
	}
}

func (e *CheckpointEntry) SetPrintTime() {
	e.Lock()
	defer e.Unlock()
	e.lastPrint = time.Now()
}

func (e *CheckpointEntry) CheckPrintTime() bool {
	e.RLock()
	defer e.RUnlock()
	return time.Since(e.lastPrint) > 4*time.Minute
}

func (e *CheckpointEntry) GetStart() types.TS { return e.start }
func (e *CheckpointEntry) GetEnd() types.TS   { return e.end }
func (e *CheckpointEntry) GetState() State {
	e.RLock()
	defer e.RUnlock()
	return e.state
}
func (e *CheckpointEntry) IsCommitted() bool {
	e.RLock()
	defer e.RUnlock()
	return e.state == ST_Finished
}
func (e *CheckpointEntry) HasOverlap(from, to types.TS) bool {
	if e.start.Greater(to) || e.end.Less(from) {
		return false
	}
	return true
}
func (e *CheckpointEntry) LessEq(ts types.TS) bool {
	return e.end.LessEq(ts)
}
func (e *CheckpointEntry) SetLocation(cn, dn objectio.Location) {
	e.Lock()
	defer e.Unlock()
	e.cnLocation = cn
	e.dnLocation = dn
}

func (e *CheckpointEntry) GetLocation() objectio.Location {
	e.RLock()
	defer e.RUnlock()
	return e.cnLocation
}

func (e *CheckpointEntry) GetVersion() uint32 {
	return e.version
}

func (e *CheckpointEntry) SetState(state State) (ok bool) {
	e.Lock()
	defer e.Unlock()
	// entry is already finished
	if e.state == ST_Finished {
		return
	}
	// entry is already running
	if state == ST_Running && e.state == ST_Running {
		return
	}
	e.state = state
	ok = true
	return
}

func (e *CheckpointEntry) IsRunning() bool {
	e.RLock()
	defer e.RUnlock()
	return e.state == ST_Running
}
func (e *CheckpointEntry) IsPendding() bool {
	e.RLock()
	defer e.RUnlock()
	return e.state == ST_Pending
}
func (e *CheckpointEntry) IsFinished() bool {
	e.RLock()
	defer e.RUnlock()
	return e.state == ST_Finished
}

func (e *CheckpointEntry) IsIncremental() bool {
	return e.entryType == ET_Incremental
}

func (e *CheckpointEntry) String() string {
	t := "I"
	if !e.IsIncremental() {
		t = "G"
	}
	state := e.GetState()
	return fmt.Sprintf("CKP[%s][%v](%s->%s)", t, state, e.start.ToString(), e.end.ToString())
}

func (e *CheckpointEntry) Prefetch(
	ctx context.Context,
	fs *objectio.ObjectFS,
	data *logtail.CheckpointData,
) (err error) {
	if err = data.PrefetchFrom(
		ctx,
		e.version,
		fs.Service,
		e.dnLocation,
	); err != nil {
		return
	}
	return
}

func (e *CheckpointEntry) Read(
	ctx context.Context,
	fs *objectio.ObjectFS,
	data *logtail.CheckpointData,
) (err error) {
	reader, err := blockio.NewObjectReader(fs.Service, e.dnLocation)
	if err != nil {
		return
	}

	if err = data.ReadFrom(
		ctx,
		e.version,
		e.dnLocation,
		reader,
		fs.Service,
		common.DefaultAllocator,
	); err != nil {
		return
	}
	return
}

func (e *CheckpointEntry) PrefetchMetaIdx(
	ctx context.Context,
	fs *objectio.ObjectFS,
) (data *logtail.CheckpointData, err error) {
	data = logtail.NewCheckpointData()
	if err = data.PrefetchMeta(
		ctx,
		e.version,
		fs.Service,
		e.dnLocation,
	); err != nil {
		return
	}
	return
}

func (e *CheckpointEntry) ReadMetaIdx(
	ctx context.Context,
	fs *objectio.ObjectFS,
	data *logtail.CheckpointData,
) (err error) {
	reader, err := blockio.NewObjectReader(fs.Service, e.dnLocation)
	if err != nil {
		return
	}
	return data.ReadDNMetaBatch(ctx, e.version, e.dnLocation, reader)
}

func (e *CheckpointEntry) GetByTableID(ctx context.Context, fs *objectio.ObjectFS, tid uint64) (ins, del, cnIns, segDel *api.Batch, err error) {
	reader, err := blockio.NewObjectReader(fs.Service, e.cnLocation)
	if err != nil {
		return
	}
	data := logtail.NewCNCheckpointData()
	err = blockio.PrefetchMeta(fs.Service, e.cnLocation)
	if err != nil {
		return
	}

	err = data.PrefetchMetaIdx(ctx, e.version, logtail.GetMetaIdxesByVersion(e.version), e.cnLocation, fs.Service)
	if err != nil {
		return
	}
	err = data.InitMetaIdx(ctx, e.version, reader, e.cnLocation, common.DefaultAllocator)
	if err != nil {
		return
	}
	err = data.PrefetchMetaFrom(ctx, e.version, e.cnLocation, fs.Service, tid)
	if err != nil {
		return
	}
	err = data.PrefetchFrom(ctx, e.version, fs.Service, e.cnLocation, tid)
	if err != nil {
		return
	}
	var bats []*batch.Batch
	if bats, err = data.ReadFromData(ctx, tid, e.cnLocation, reader, e.version, common.DefaultAllocator); err != nil {
		return
	}
	ins, del, cnIns, segDel, err = data.GetTableDataFromBats(tid, bats)
	return
}

func (e *CheckpointEntry) GCMetadata(fs *objectio.ObjectFS) error {
	name := blockio.EncodeCheckpointMetadataFileName(CheckpointDir, PrefixMetadata, e.start, e.end)
	err := fs.Delete(name)
	logutil.Debugf("GC checkpoint metadata %v, err %v", e.String(), err)
	return err
}

func (e *CheckpointEntry) GCEntry(fs *objectio.ObjectFS) error {
	err := fs.Delete(e.cnLocation.Name().String())
	defer logutil.Debugf("GC checkpoint metadata %v, err %v", e.String(), err)
	return err
}
