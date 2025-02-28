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

package mergesort

import (
	"github.com/matrixorigin/matrixone/pkg/vm/engine/tae/containers"
)

func sort[T any](col containers.Vector, lessFunc LessFunc[T], n int) SortSlice[T] {
	dataWithIdx := NewSortSlice(n, lessFunc)
	if col.HasNull() {
		for i := 0; i < n; i++ {
			var item SortElem[T]
			if col.IsNull(i) {
				item = SortElem[T]{isNull: true, idx: int32(i)}
			} else {
				item = SortElem[T]{data: col.Get(i).(T), idx: int32(i)}
			}
			dataWithIdx.Append(item)
		}
	} else {
		for i := 0; i < n; i++ {
			dataWithIdx.Append(SortElem[T]{data: col.Get(i).(T), idx: int32(i)})
		}
	}
	sortUnstable(dataWithIdx)
	return dataWithIdx
}

func Sort[T any](col containers.Vector, lessFunc LessFunc[T], idx []int32) (ret containers.Vector) {
	dataWithIdx := sort(col, lessFunc, len(idx))

	// make col sorted
	for i, v := range dataWithIdx.AsSlice() {
		idx[i] = v.idx
		if v.isNull {
			col.Update(i, nil, true)
		} else {
			col.Update(i, v.data, false)
		}
	}

	ret = col
	return
}

func Merge[T any](
	col []containers.Vector,
	src *[]uint32,
	lessFunc LessFunc[T],
	fromLayout,
	toLayout []uint32,
	pool *containers.VectorPool,
) (ret []containers.Vector, mapping []uint32) {
	ret = make([]containers.Vector, len(toLayout))
	mapping = make([]uint32, len(*src))

	offset := make([]uint32, len(fromLayout))
	offset[0] = 0
	for i := 1; i < len(fromLayout); i++ {
		offset[i] = offset[i-1] + fromLayout[i-1]
	}

	for i := range toLayout {
		ret[i] = pool.GetVector(col[0].GetType())
	}

	nBlk := len(col)
	heap := NewHeapSlice(nBlk, lessFunc)

	for i := 0; i < nBlk; i++ {
		if col[i].IsNull(0) {
			heap.Append(HeapElem[T]{isNull: true, src: uint32(i), next: 1})
		} else {
			heap.Append(HeapElem[T]{data: col[i].Get(0).(T), src: uint32(i), next: 1})
		}
	}
	heapInit(heap)

	k := 0
	for i := 0; i < len(toLayout); i++ {
		for j := 0; j < int(toLayout[i]); j++ {
			top := heapPop(&heap)
			(*src)[k] = top.src
			if top.isNull {
				ret[i].Append(nil, true)
			} else {
				ret[i].Append(top.data, false)
			}
			mapping[offset[top.src]+top.next-1] = uint32(k)
			k++
			if int(top.next) < int(fromLayout[top.src]) {
				if col[top.src].IsNull(int(top.next)) {
					heapPush(&heap, HeapElem[T]{isNull: true, src: top.src, next: top.next + 1})
				} else {
					heapPush(&heap, HeapElem[T]{data: col[top.src].Get(int(top.next)).(T), src: top.src, next: top.next + 1})
				}
			}
		}
	}
	return
}

func Shuffle(
	col containers.Vector, idx []int32, pool *containers.VectorPool,
) containers.Vector {
	var err error
	ret := pool.GetVector(col.GetType())
	if err = ret.PreExtend(len(idx)); err != nil {
		panic(err)
	}
	retVec := ret.GetDownstreamVector()
	srcVec := col.GetDownstreamVector()

	if err = retVec.Union(srcVec, idx, ret.GetAllocator()); err != nil {
		panic(err)
	}

	col.Close()
	return ret
}

func Multiplex(
	col []containers.Vector, src []uint32, fromLayout, toLayout []uint32, pool *containers.VectorPool,
) (ret []containers.Vector) {
	ret = make([]containers.Vector, len(toLayout))
	cursors := make([]int, len(fromLayout))

	k := 0
	for i := 0; i < len(toLayout); i++ {
		ret[i] = pool.GetVector(col[0].GetType())
		ret[i].PreExtend(int(toLayout[i]))
		for j := 0; j < int(toLayout[i]); j++ {
			s := src[k]
			if col[s].IsNull(cursors[s]) {
				ret[i].Append(nil, true)
			} else {
				ret[i].Append(col[s].Get(cursors[s]), false)
			}
			cursors[s]++
			k++
		}
	}

	for _, v := range col {
		v.Close()
	}
	return
}

func ShuffleColumn(
	column []containers.Vector, sortedIdx []uint32, fromLayout, toLayout []uint32, pool *containers.VectorPool,
) (ret []containers.Vector) {
	ret = Multiplex(column, sortedIdx, fromLayout, toLayout, pool)
	return
}

func Reshape(
	column []containers.Vector, fromLayout, toLayout []uint32, pool *containers.VectorPool,
) (ret []containers.Vector) {
	ret = make([]containers.Vector, len(toLayout))
	fromIdx := 0
	fromOffset := 0
	for i := 0; i < len(toLayout); i++ {
		ret[i] = pool.GetVector(column[0].GetType())
		toOffset := 0
		for toOffset < int(toLayout[i]) {
			fromLeft := fromLayout[fromIdx] - uint32(fromOffset)
			if fromLeft == 0 {
				fromIdx++
				fromOffset = 0
				fromLeft = fromLayout[fromIdx]
			}
			length := 0
			if fromLeft < toLayout[i]-uint32(toOffset) {
				length = int(fromLeft)
			} else {
				length = int(toLayout[i]) - toOffset
			}
			cloned := column[fromIdx].CloneWindow(fromOffset, length)
			defer cloned.Close()
			ret[i].Extend(cloned)
			fromOffset += length
			toOffset += length
		}
	}
	for _, v := range column {
		v.Close()
	}
	return
}
