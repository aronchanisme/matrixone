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

package connector

import (
	"bytes"
	"context"
	"testing"

	"github.com/matrixorigin/matrixone/pkg/common/mpool"
	"github.com/matrixorigin/matrixone/pkg/container/batch"
	"github.com/matrixorigin/matrixone/pkg/container/types"
	"github.com/matrixorigin/matrixone/pkg/testutil"
	"github.com/matrixorigin/matrixone/pkg/vm/process"
	"github.com/stretchr/testify/require"
)

const (
	Rows = 10 // default rows
)

// add unit tests for cases
type connectorTestCase struct {
	arg    *Argument
	types  []types.Type
	proc   *process.Process
	cancel context.CancelFunc
}

var (
	tcs []connectorTestCase
)

func init() {
	tcs = []connectorTestCase{
		newTestCase(),
	}
}

func TestString(t *testing.T) {
	buf := new(bytes.Buffer)
	for _, tc := range tcs {
		String(tc.arg, buf)
	}
}

func TestPrepare(t *testing.T) {
	for _, tc := range tcs {
		err := Prepare(tc.proc, tc.arg)
		require.NoError(t, err)
	}
}

func TestConnector(t *testing.T) {
	for _, tc := range tcs {
		err := Prepare(tc.proc, tc.arg)
		require.NoError(t, err)
		bat := newBatch(t, tc.types, tc.proc, Rows)
		tc.proc.Reg.InputBatch = bat
		/*{
			for _, vec := range bat.Vecs {
				if vec.IsOriginal() {
					vec.FreeOriginal(tc.proc.Mp())
				}
			}
		}*/
		_, _ = Call(0, tc.proc, tc.arg, false, false)
		tc.proc.Reg.InputBatch = batch.EmptyBatch
		_, _ = Call(0, tc.proc, tc.arg, false, false)
		tc.proc.Reg.InputBatch = nil
		_, _ = Call(0, tc.proc, tc.arg, false, false)
		for len(tc.arg.Reg.Ch) > 0 {
			bat := <-tc.arg.Reg.Ch
			if bat == nil {
				break
			}
			if bat.IsEmpty() {
				continue
			}
			bat.Clean(tc.proc.Mp())
		}
		tc.arg.Free(tc.proc, false)
		tc.proc.FreeVectors()
		require.Equal(t, int64(0), tc.proc.Mp().CurrNB())
	}
}

func newTestCase() connectorTestCase {
	proc := testutil.NewProcessWithMPool(mpool.MustNewZero())
	proc.Reg.MergeReceivers = make([]*process.WaitRegister, 2)
	ctx, cancel := context.WithCancel(context.Background())
	return connectorTestCase{
		proc:  proc,
		types: []types.Type{types.T_int8.ToType()},
		arg: &Argument{
			Reg: &process.WaitRegister{
				Ctx: ctx,
				Ch:  make(chan *batch.Batch, 3),
			},
		},
		cancel: cancel,
	}

}

// create a new block based on the type information
func newBatch(t *testing.T, ts []types.Type, proc *process.Process, rows int64) *batch.Batch {
	return testutil.NewBatch(ts, false, int(rows), proc.Mp())
}
