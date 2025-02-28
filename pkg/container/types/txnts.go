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

package types

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"sync/atomic"
	"time"

	"github.com/matrixorigin/matrixone/pkg/pb/timestamp"
	"github.com/matrixorigin/matrixone/pkg/txn/clock"
)

// Transaction ts contains a physical ts in higher 8 bytes
// and a logical in lower 4 bytes.  higher lower in little
// ending sense.
func (ts TS) Physical() int64 {
	return DecodeInt64(ts[4:12])
}
func (ts TS) Logical() uint32 {
	return DecodeUint32(ts[:4])
}

func (ts TS) IsEmpty() bool {
	return ts.Physical() == 0 && ts.Logical() == 0
}
func (ts TS) Equal(rhs TS) bool {
	return ts == rhs
}

// Compare physical first then logical.
func (ts TS) Compare(rhs TS) int {
	p1, p2 := ts.Physical(), rhs.Physical()
	if p1 < p2 {
		return -1
	}
	if p1 > p2 {
		return 1
	}
	l1, l2 := ts.Logical(), rhs.Logical()
	if l1 < l2 {
		return -1
	}
	if l1 == l2 {
		return 0
	}
	return 1
}

func (ts TS) Less(rhs TS) bool {
	return ts.Compare(rhs) < 0
}
func (ts TS) LessEq(rhs TS) bool {
	return ts.Compare(rhs) <= 0
}
func (ts TS) Greater(rhs TS) bool {
	return ts.Compare(rhs) > 0
}
func (ts TS) GreaterEq(rhs TS) bool {
	return ts.Compare(rhs) >= 0
}

// TODO::need to take "NodeID" account into
func TimestampToTS(ts timestamp.Timestamp) TS {
	return BuildTS(ts.PhysicalTime, ts.LogicalTime)
}

func (ts TS) ToTimestamp() timestamp.Timestamp {
	return timestamp.Timestamp{
		PhysicalTime: DecodeInt64(ts[4:12]),
		LogicalTime:  DecodeUint32(ts[:4])}
}

func BuildTS(p int64, l uint32) (ret TS) {
	copy(ret[4:12], EncodeInt64(&p))
	copy(ret[:4], EncodeUint32(&l))
	return
}

func MaxTs() TS {
	return BuildTS(math.MaxInt64, math.MaxUint32)
}

// Who use this function?
func (ts TS) Prev() TS {
	p, l := ts.Physical(), ts.Logical()
	if l == 0 {
		return BuildTS(p-1, math.MaxUint32)
	}
	return BuildTS(p, l-1)
}
func (ts TS) Next() TS {
	p, l := ts.Physical(), ts.Logical()
	if l == math.MaxUint32 {
		return BuildTS(p+1, 0)
	}
	return BuildTS(p, l+1)
}

func (ts TS) ToString() string {
	return fmt.Sprintf("%d-%d", ts.Physical(), ts.Logical())
}

func StringToTS(s string) (ts TS) {
	tmp := strings.Split(s, "-")
	if len(tmp) != 2 {
		panic("format of ts must be physical-logical")
	}

	pTime, err := strconv.ParseInt(tmp[0], 10, 64)
	if err != nil {
		panic("format of ts must be physical-logical, physical is not an integer")
	}

	lTime, err := strconv.ParseUint(tmp[1], 10, 32)
	if err != nil {
		panic("format of ts must be physical-logical, logical is not an uint32")
	}
	return BuildTS(pTime, uint32(lTime))
}

// XXX
// XXX The following code does not belong to types. TAE folks please fix.

// CompoundKeyType -- this is simply deadly wrong thing.
var CompoundKeyType Type

// Why this was in package types?
var SystemDBTS TS

func init() {
	CompoundKeyType = T_varchar.ToType()
	CompoundKeyType.Width = 100

	SystemDBTS = BuildTS(1, 0)
}

// var v T
func DefaultVal[T any]() T {
	var v T
	return v
}

// TAE test infra, should move out
var (
	//just for test
	GlobalTsAlloctor *TsAlloctor
)

func init() {
	GlobalTsAlloctor = NewTsAlloctor(NewMockHLCClock(1))
}

type TsAlloctor struct {
	clock clock.Clock
}

func NewTsAlloctor(clock clock.Clock) *TsAlloctor {
	return &TsAlloctor{clock: clock}
}

func (alloc *TsAlloctor) Alloc() TS {
	now, _ := alloc.clock.Now()
	var ts TS
	copy(ts[4:12], EncodeInt64(&now.PhysicalTime))
	copy(ts[:4], EncodeUint32(&now.LogicalTime))
	return ts
}

// TODO::will be removed
func (alloc *TsAlloctor) Get() TS {
	if mockClock, ok := alloc.clock.(*MockHLCClock); ok {
		var ts TS
		i64 := mockClock.Get().PhysicalTime
		copy(ts[4:12], EncodeInt64(&i64))
		//copy(ts[:4], EncodeUint32(mockClock.Get().LogicalTime))
		return ts
	}
	panic("HLCClock does not support Get()")
}

func (alloc *TsAlloctor) SetStart(start TS) {
	//if start.Greater(alloc.Get()) {
	alloc.clock.Update(timestamp.Timestamp{PhysicalTime: DecodeInt64(start[4:12]),
		LogicalTime: DecodeUint32(start[:4])})
	//}
}

func NextGlobalTsForTest() TS {
	return GlobalTsAlloctor.Alloc()
}

type MockHLCClock struct {
	pTime int64
	//always be 0
	//lTime     uint32
	maxOffset time.Duration
}

// Just for test , start >= 1
func NewMockHLCClock(start int64) *MockHLCClock {
	return &MockHLCClock{pTime: start}
}

func (c *MockHLCClock) Now() (timestamp.Timestamp, timestamp.Timestamp) {
	now := timestamp.Timestamp{
		PhysicalTime: atomic.AddInt64(&c.pTime, 1),
		//LogicalTime:  c.lTime,
	}
	return now, timestamp.Timestamp{PhysicalTime: now.PhysicalTime + int64(c.maxOffset)}
}

// TODO::will be removed
func (c *MockHLCClock) Get() timestamp.Timestamp {
	return timestamp.Timestamp{
		PhysicalTime: atomic.LoadInt64(&c.pTime),
		//LogicalTime:  c.lTime,
	}
}

func (c *MockHLCClock) Update(m timestamp.Timestamp) {
	atomic.StoreInt64(&c.pTime, m.PhysicalTime)
	//atomic.StoreUint32(&c.lTime, m.LogicalTime)
}

func (c *MockHLCClock) HasNetworkLatency() bool {
	return false
}

func (c *MockHLCClock) MaxOffset() time.Duration {
	return c.maxOffset
}

func (c *MockHLCClock) SetNodeID(id uint16) {
	// nothing to do.
}

func MockColTypes(colCnt int) (ct []Type) {
	for i := 0; i < colCnt; i++ {
		var typ Type
		switch i {
		case 0:
			typ = T_int8.ToType()
		case 1:
			typ = T_int16.ToType()
		case 2:
			typ = T_int32.ToType()
		case 3:
			typ = T_int64.ToType()
		case 4:
			typ = T_uint8.ToType()
		case 5:
			typ = T_uint16.ToType()
		case 6:
			typ = T_uint32.ToType()
		case 7:
			typ = T_uint64.ToType()
		case 8:
			typ = T_float32.ToType()
		case 9:
			typ = T_float64.ToType()
		case 10:
			typ = T_date.ToType()
		case 11:
			typ = T_datetime.ToType()
		case 12:
			typ = T_varchar.ToType()
		case 13:
			typ = T_char.ToType()
		case 14:
			typ = T_bool.ToType()
			typ.Width = 8
		case 15:
			typ = T_timestamp.ToType()
		case 16:
			typ = T_decimal64.ToType()
		case 17:
			typ = T_decimal128.ToType()
		case 18:
			typ = T_binary.ToType()
		case 19:
			typ = T_varbinary.ToType()
		case 20:
			typ = T_enum.ToType()
		}
		ct = append(ct, typ)
	}
	return
}

func CompareTSTSAligned(a, b TS) int {
	return bytes.Compare(a[:], b[:])
}
