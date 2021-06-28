package common

import (
	"encoding/binary"
	"errors"
	"math"
)

var errOverflow = errors.New("overflow error")
var errAbnormalChar = errors.New("abnormal char in amount")
var errAmountFormat = errors.New("amount format error")
var errDivideByZero = errors.New("divide by zero error")
var errMultiDecimalPoint = errors.New("multi decimal point error")

// Decimal implements decimal number for use of token balance
type Decimal struct {
	Value int64
	Scale int
	Err   error
}

// Float64 ...
func (f *Decimal) Float64() float64 {
	return float64(f.Value) / math.Pow10(f.Scale)
}

// IsZero checks whether the value is zero
func (f *Decimal) IsZero() bool {
	return f.Value == 0
}

// IsPositive ...
func (f *Decimal) IsPositive() bool {
	return f.Value > 0
}

// IsNegative ...
func (f *Decimal) IsNegative() bool {
	return f.Value < 0
}

// Equals check equal
func (f *Decimal) Equals(other *Decimal) bool {
	fpnNew, otherNew, err := RescalePair(f, other)
	if err != nil {
		// one number overflows, so they cannot be equal
		return false
	}
	return fpnNew.Value == otherNew.Value
}

// LessThan ...
func (f *Decimal) LessThan(other *Decimal) bool {
	fpnNew, otherNew, err := RescalePair(f, other)
	if err == nil {
		return fpnNew.Value < otherNew.Value
	}
	// so one number overflows
	if f.IsNegative() {
		if other.IsZero() || other.IsPositive() {
			return true
		}
		// they are both negative
		if f.Scale < other.Scale {
			// eg: "-50000" < "-1.00000001"
			return true
		}
		return false
	}
	if f.IsZero() {
		// other must be negative, since they cannot be both zero, or else UnifyDecimal will not return error
		return other.IsPositive()
	}
	// f is positive now
	if other.IsNegative() || other.IsZero() {
		return false
	}
	// they are both positive
	if f.Scale > other.Scale {
		// eg: "1.00000001" < "50000"
		return true
	}
	return false
}

// GreaterThan ...
func (f *Decimal) GreaterThan(other *Decimal) bool {
	return other.LessThan(f)
}

// Neg get negative number
func (f *Decimal) Neg() *Decimal {
	if multiplyOverflow(f.Value, -1) {
		f.Err = errOverflow
		return nil
	}
	return &Decimal{Value: -f.Value, Scale: f.Scale}
}

// Add ...
func (f *Decimal) Add(other *Decimal) *Decimal {
	fpnNew, otherNew, err := RescalePair(f, other)
	if err != nil {
		f.Err = err
		return nil
	}
	resultValue := fpnNew.Value + otherNew.Value
	if fpnNew.Value > 0 && otherNew.Value > 0 && resultValue < 0 {
		return nil
	}
	if fpnNew.Value < 0 && otherNew.Value < 0 && resultValue > 0 {
		return nil
	}
	return &Decimal{Value: resultValue, Scale: fpnNew.Scale}
}

// Sub ...
func (f *Decimal) Sub(other *Decimal) *Decimal {
	ret := other.Neg()
	if other.Err != nil {
		f.Err = other.Err
		return nil
	}
	return f.Add(ret)
}

// Mul ...
func (f *Decimal) Mul(other *Decimal) *Decimal {
	fpnNew := f.ShrinkScale()
	otherNew := other.ShrinkScale()
	if multiplyOverflow(fpnNew.Value, otherNew.Value) {
		f.Err = errOverflow
		return nil
	}
	return &Decimal{Value: fpnNew.Value * otherNew.Value, Scale: fpnNew.Scale + otherNew.Scale}
}

// MulInt multiply a int
func (f *Decimal) MulInt(i int64) *Decimal {
	if multiplyOverflow(f.Value, i) {
		f.Err = errOverflow
		return nil
	}
	return &Decimal{Value: f.Value * i, Scale: f.Scale}
}

// MulFloat multiply a float
func (f *Decimal) MulFloat(v float64) *Decimal {
	if multiplyOverflow(f.Value, int64(math.Floor(v))) || multiplyOverflow(f.Value, int64(math.Ceil(v))) {
		f.Err = errOverflow
		return nil
	}
	return &Decimal{Value: int64(math.Round(float64(f.Value) * v)), Scale: f.Scale}
}

// DivInt divide by a scalar
func (f *Decimal) DivInt(i int64) *Decimal {
	if i == 0 {
		f.Err = errDivideByZero
		return nil
	}
	return &Decimal{Value: f.Value / i, Scale: f.Scale}
}

func (f *Decimal) FloorInt() int64 {
	return f.Rescale(0).Value
}

// Rescale change scale to given scale, without changing its real value
func (f *Decimal) Rescale(targetScale int) *Decimal {
	value := f.Value
	scale := f.Scale
	for targetScale > scale {
		scale++
		if multiplyOverflow(value, 10) {
			return &Decimal{0, 0, errOverflow}
		}
		value *= 10
	}
	for targetScale < scale {
		scale--
		value /= 10
	}
	return &Decimal{Value: value, Scale: scale}
}

// ShrinkScale remove trailing 0s
func (f *Decimal) ShrinkScale() *Decimal {
	value := f.Value
	scale := f.Scale
	for value%10 == 0 && scale > 0 {
		value /= 10
		scale--
	}
	return &Decimal{Value: value, Scale: scale}
}

// RescalePair make two decimal number have same scale.
func RescalePair(a *Decimal, b *Decimal) (*Decimal, *Decimal, error) {
	if a.Scale < b.Scale {
		aChanged := a.Rescale(b.Scale)
		if aChanged.Err != nil {
			return nil, nil, aChanged.Err
		}
		return aChanged, b, nil
	}
	bChanged := b.Rescale(a.Scale)
	if bChanged.Err != nil {
		return nil, nil, bChanged.Err
	}
	return a, bChanged, nil
}

func NewDecimalFromInt(v int) *Decimal {
	return &Decimal{Value: int64(v), Scale: 0}
}

func NewDecimalFromIntWithScale(v int, s int) *Decimal {
	return (&Decimal{Value: int64(v), Scale: 0}).Rescale(s)
}

func NewDecimalExp10(i int) *Decimal {
	return &Decimal{
		Value: 1,
		Scale: -i,
	}
}

// NewDecimalFromString generate Decimal from string and scale, will truncate if scale is smaller. scale < 0 means auto detecting scale
func NewDecimalFromString(amount string, scale int) (*Decimal, error) {
	if len(amount) == 0 || amount[0] == '.' {
		return nil, errAmountFormat
	}
	if amount[0] == '-' {
		fpn, err := NewDecimalFromString(amount[1:], scale)
		if err != nil {
			return nil, err
		}
		return fpn.Neg(), fpn.Err
	}
	fpn := &Decimal{Value: 0, Scale: 0}
	decimalPointExists := false
	for i := 0; i < len(amount); i++ {
		if '0' <= amount[i] && amount[i] <= '9' {
			num := int64(amount[i] - '0')
			if multiplyOverflow(fpn.Value, 10) {
				return nil, errOverflow
			}
			fpn.Value = fpn.Value*10 + num
			if fpn.Value < 0 {
				return nil, errOverflow
			}
			if decimalPointExists {
				fpn.Scale++
				if scale >= 0 && fpn.Scale >= scale {
					break
				}
			}
		} else if amount[i] == '.' {
			if decimalPointExists {
				return nil, errMultiDecimalPoint
			}
			decimalPointExists = true
		} else {
			return nil, errAbnormalChar
		}
	}
	if scale >= 0 {
		fpn = fpn.Rescale(scale)
	}
	return fpn, fpn.Err
}

// ToStringWithFullScale convert to string with tailing 0s
func (f *Decimal) ToStringWithFullScale() string {
	if f.Value < 0 {
		ret := f.Neg()
		if f.Err != nil {
			return ""
		}
		str := ret.String()
		return "-" + str
	}
	val := f.Value
	str := make([]byte, 0)
	for val > 0 || len(str) <= f.Scale {
		str = append(str, byte('0'+val%10))
		val /= 10
	}
	rtn := make([]byte, 0)
	for i := len(str) - 1; i >= 0; i-- {
		if i+1 == f.Scale {
			rtn = append(rtn, '.')
		}
		rtn = append(rtn, str[i])
	}
	return string(rtn)
}

// String generate string of Decimal without post zero
func (f *Decimal) String() string {
	rtn := f.ToStringWithFullScale()
	if rtn == "" {
		return rtn
	}
	if f.Scale == 0 {
		return rtn
	}
	for rtn[len(rtn)-1] == '0' {
		rtn = rtn[0 : len(rtn)-1]
	}
	if rtn[len(rtn)-1] == '.' {
		rtn = rtn[0 : len(rtn)-1]
	}
	return rtn
}

func multiplyOverflow(a int64, b int64) bool {
	x := a * b
	if a != 0 && x/a != b {
		return true
	}
	return false
}

// Marshal ...
func (f *Decimal) Marshal() string {
	b1 := make([]byte, 8)
	binary.LittleEndian.PutUint64(b1, uint64(f.Value))
	b2 := make([]byte, 4)
	binary.LittleEndian.PutUint32(b2, uint32(f.Scale))
	return string(b1) + string(b2)
}

// UnmarshalDecimal unmarshal from string
func UnmarshalDecimal(s string) (*Decimal, error) {
	if len(s) != 8+4 {
		return &Decimal{Value: 0, Scale: 0}, errors.New("invalid length to unmarshal decimal number")
	}
	return &Decimal{Value: int64(binary.LittleEndian.Uint64([]byte(s[:8]))), Scale: int(int32(binary.LittleEndian.Uint32([]byte(s[8:]))))}, nil
}
