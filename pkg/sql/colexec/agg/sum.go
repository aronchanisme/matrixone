// Copyright 2022 Matrix Origin
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

package agg

import (
	"github.com/matrixorigin/matrixone/pkg/common/moerr"
	"github.com/matrixorigin/matrixone/pkg/container/nulls"
	"github.com/matrixorigin/matrixone/pkg/container/types"
)

// if input type is int8/int16/int32/int64, the return type is int64
// if input type is uint8/uint16/uint32/uint64, the return type is uint64
// f input type is float32/float64, the return type is float64
type ReturnTyp interface {
	uint64 | int64 | float64
}

type Sum[T1 Numeric, T2 ReturnTyp] struct {
}

type Decimal64Sum struct {
}

type Decimal128Sum struct {
}

var SumSupported = []types.T{
	types.T_uint8, types.T_uint16, types.T_uint32, types.T_uint64,
	types.T_int8, types.T_int16, types.T_int32, types.T_int64,
	types.T_float32, types.T_float64,
	types.T_decimal64, types.T_decimal128,
}

func SumReturnType(typs []types.Type) types.Type {
	switch typs[0].Oid {
	case types.T_float32, types.T_float64:
		return types.T_float64.ToType()
	case types.T_int8, types.T_int16, types.T_int32, types.T_int64:
		return types.T_int64.ToType()
	case types.T_uint8, types.T_uint16, types.T_uint32, types.T_uint64:
		return types.T_uint64.ToType()
	case types.T_decimal64:
		return types.New(types.T_decimal64, 18, typs[0].Scale)
	case types.T_decimal128:
		return types.New(types.T_decimal128, 38, typs[0].Scale)
	}
	panic(moerr.NewInternalErrorNoCtx("unsupport type '%v' for sum", typs[0]))
}

func NewSum[T1 Numeric, T2 ReturnTyp]() *Sum[T1, T2] {
	return &Sum[T1, T2]{}
}

func (s *Sum[T1, T2]) Grows(_ int) {
}

func (s *Sum[T1, T2]) Eval(vs []T2, err error) ([]T2, error) {
	return vs, nil
}

func (s *Sum[T1, T2]) Fill(_ int64, value T1, ov T2, z int64, isEmpty bool, isNull bool) (T2, bool, error) {
	if !isNull {
		return ov + T2(value)*T2(z), false, nil
	}
	return ov, isEmpty, nil
}

func (s *Sum[T1, T2]) Merge(_ int64, _ int64, x T2, y T2, xEmpty bool, yEmpty bool, _ any) (T2, bool, error) {
	if !yEmpty {
		if !xEmpty {
			return x + y, false, nil
		}
		return y, false, nil
	}
	return x, xEmpty, nil

}

func (s *Sum[T1, T2]) MarshalBinary() ([]byte, error) {
	return nil, nil
}

func (s *Sum[T1, T2]) UnmarshalBinary(data []byte) error {
	return nil
}

func NewD64Sum() *Decimal64Sum {
	return &Decimal64Sum{}
}

func (s *Decimal64Sum) Grows(_ int) {
}

func (s *Decimal64Sum) Eval(vs []types.Decimal64, err error) ([]types.Decimal64, error) {
	return vs, err
}

func (s *Decimal64Sum) Fill(_ int64, value types.Decimal64, ov types.Decimal64, z int64, isEmpty bool, isNull bool) (types.Decimal64, bool, error) {
	if !isNull {
		var err error
		value, err = value.Mul64(types.Decimal64(z))
		if err == nil {
			ov, err = ov.Add64(value)
		}
		return ov, false, err
	}
	return ov, isEmpty, nil
}

func (s *Decimal64Sum) Merge(_ int64, _ int64, x types.Decimal64, y types.Decimal64, xEmpty bool, yEmpty bool, _ any) (types.Decimal64, bool, error) {
	if !yEmpty {
		if !xEmpty {
			var err error
			x, err = x.Add64(y)
			return x, false, err
		}
		return y, false, nil
	}
	return x, xEmpty, nil
}

func (s *Decimal64Sum) BatchFill(rs, vs any, start, count int64, vps []uint64, nsp *nulls.Nulls) (err error) {
	rrs := rs.([]types.Decimal64)
	vvs := vs.([]types.Decimal64)

	for i := int64(0); i < count; i++ {
		if vps[i] == 0 {
			continue
		}
		rrs[vps[i]-1], _, err = rrs[vps[i]-1].Add(vvs[i+start], 0, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Decimal64Sum) MarshalBinary() ([]byte, error) {
	return nil, nil
}

func (s *Decimal64Sum) UnmarshalBinary(data []byte) error {
	return nil
}

func NewD128Sum() *Decimal128Sum {
	return &Decimal128Sum{}
}

func (s *Decimal128Sum) Grows(_ int) {
}

func (s *Decimal128Sum) Eval(vs []types.Decimal128, err error) ([]types.Decimal128, error) {
	return vs, err
}

func (s *Decimal128Sum) Fill(_ int64, value types.Decimal128, ov types.Decimal128, z int64, isEmpty bool, isNull bool) (types.Decimal128, bool, error) {
	if !isNull {
		var err error
		value, _, err = value.Mul(types.Decimal128{B0_63: uint64(z), B64_127: 0}, 0, 0)
		if err == nil {
			ov, err = ov.Add128(value)
		}
		return ov, false, err
	}
	return ov, isEmpty, nil
}

func (s *Decimal128Sum) Merge(_ int64, _ int64, x types.Decimal128, y types.Decimal128, xEmpty bool, yEmpty bool, _ any) (types.Decimal128, bool, error) {
	if !yEmpty {
		if !xEmpty {
			var err error
			x, err = x.Add128(y)
			return x, false, err
		}
		return y, false, nil
	}
	return x, xEmpty, nil
}

func (s *Decimal128Sum) BatchFill(rs, vs any, start, count int64, vps []uint64, nsp *nulls.Nulls) (err error) {
	rrs := rs.([]types.Decimal128)
	vvs := vs.([]types.Decimal128)

	for i := int64(0); i < count; i++ {
		if vps[i] == 0 {
			continue
		}
		rrs[vps[i]-1], _, err = rrs[vps[i]-1].Add(vvs[i+start], 0, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Decimal128Sum) MarshalBinary() ([]byte, error) {
	return nil, nil
}

func (s *Decimal128Sum) UnmarshalBinary(data []byte) error {
	return nil
}
