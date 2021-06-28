package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecimal_ToString(t *testing.T) {
	f := Decimal{-9223372036854775808, 4, nil}
	f.String()
	assert.Equal(t, f.Err, errOverflow)
	f = Decimal{100, 0, nil}
	s := f.String()
	assert.Equal(t, s, "100")
}

func TestNewFixed(t *testing.T) {
	f, err := NewDecimalFromString("-323.49494", 12)
	assert.Equal(t, err, nil)
	assert.Equal(t, f.Value, int64(-323494940000000))
	_, err = NewDecimalFromString("-9223372036854775808", 0)
	assert.Equal(t, err, errOverflow)
	_, err = NewDecimalFromString("-323.49494", 40)
	assert.Equal(t, err, errOverflow)
	_, err = NewDecimalFromString("323.494.94", 10)
	assert.Equal(t, err, errMultiDecimalPoint)
}

func TestDecimal_Multiply(t *testing.T) {
	f1 := Decimal{-9223372036854775807, 4, nil}
	f2 := Decimal{-9223372036854775807, 4, nil}
	f1.Mul(&f2)
	assert.Equal(t, f1.Err, errOverflow)
}

func TestDecimal_Times(t *testing.T) {
	f1 := Decimal{-9223372036854775807, 4, nil}
	f1.MulInt(3)
	assert.Equal(t, f1.Err, errOverflow)
}

func TestDecimal_Marshal(t *testing.T) {
	f, err := UnmarshalDecimal((&Decimal{1230, 2, nil}).Marshal())
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, f.Scale)
	assert.Equal(t, nil, f.Err)
	assert.Equal(t, int64(1230), f.Value)
}

func TestDecimal_Compare(t *testing.T) {
	checkComp := func(a string, b string, result bool) {
		fa, err := NewDecimalFromString(a, -1)
		assert.Equal(t, err, nil)
		fb, err := NewDecimalFromString(b, -1)
		assert.Equal(t, err, nil)
		assert.Equal(t, fa.LessThan(fb), result)
	}

	// covers every branch in 'LessThan'
	// naive case
	checkComp("1.0", "999", true)
	// 1152921504606846976: 2**60
	checkComp("-1.00000001", "1152921504606846976", true)
	checkComp("-1.00000001", "-1152921504606846976", false)
	checkComp("-1152921504606846976", "-1.00000001", true)
	checkComp("0.00000000", "1152921504606846976", true)
	checkComp("0.00000000", "-1152921504606846976", false)
	checkComp("1152921504606846976", "-1.00000001", false)
	checkComp("1152921504606846976", "1.00000001", false)
	checkComp("1.00000001", "1152921504606846976", true)

}
