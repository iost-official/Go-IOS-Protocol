package database

import (
	"encoding/binary"
	"errors"
	"strings"

	"github.com/iost-official/go-iost/v3/common"
)

// const prefixs
const (
	IntPrefix      = "i"
	StringPrefix   = "s"
	NilPrefix      = "n"
	BoolPrefix     = "b"
	JSONPrefix     = "j"
	FixPointPrefix = "f"
)

var (
	errTypeNotSupport = errors.New("type not support")
	errInvalidData    = errors.New("invalid data")
)

// SerializedJSON type of Serialized json
type SerializedJSON []byte

// Marshal marshal go types to value string
func Marshal(in interface{}, extras ...string) (string, error) {
	extra := ""
	if len(extras) > 0 {
		extra = extras[0]
	}
	switch in := in.(type) {
	case int64:
		return IntPrefix + int64ToRaw(in) + RAMOwnerSeparator + extra, nil
	case string:
		return StringPrefix + in + RAMOwnerSeparator + extra, nil
	case nil:
		return NilPrefix, nil
	case bool:
		return BoolPrefix + boolToString(in) + RAMOwnerSeparator + extra, nil
	case SerializedJSON:
		return JSONPrefix + string(in) + RAMOwnerSeparator + extra, nil
	case *common.Decimal:
		return FixPointPrefix + in.Marshal() + RAMOwnerSeparator + extra, nil
	}
	return "", errTypeNotSupport
}

// MustMarshal marshal go types to value string, panic on error
func MustMarshal(in interface{}, extra ...string) string {
	s, err := Marshal(in, extra...)
	if err != nil {
		panic(err)
	}
	return s
}

// unmarshalInner unmarshal value string to go types
func unmarshalInner(o string) interface{} {
	if len(o) < 1 {
		return errInvalidData
	}
	switch o[0:1] {
	case IntPrefix:
		return rawToInt64(o[1:])
	case StringPrefix:
		return o[1:]
	case NilPrefix:
		return nil
	case BoolPrefix:
		return o[1] == 't'
	case JSONPrefix:
		return SerializedJSON(o[1:])
	case MapKeysSeparator:
		// seems useless...
		return strings.Split(o, "@")[1:]
	case FixPointPrefix:
		ret, err := common.UnmarshalDecimal(o[1:])
		if err != nil {
			return err
		}
		return ret
	}
	return errInvalidData
}

// Unmarshal unmarshal value string to go types
func Unmarshal(o string) interface{} {
	idx := strings.LastIndex(o, RAMOwnerSeparator)
	if idx == -1 {
		return unmarshalInner(o)
	}
	return unmarshalInner(o[0:idx])
}

// UnmarshalWithExtra unmarshal value string to go types
func UnmarshalWithExtra(o string) (interface{}, string) {
	idx := strings.LastIndex(o, RAMOwnerSeparator)
	if idx == -1 {
		return unmarshalInner(o), ""
	}
	return unmarshalInner(o[0:idx]), o[idx+1:]
}

// MustUnmarshal  unmarshal value string to go types, panic on error
func MustUnmarshal(o string) interface{} {
	rtn := Unmarshal(o)
	if err, ok := rtn.(error); ok {
		panic(err.Error() + ":" + o)
	}
	return rtn
}

// MustUnmarshalWithExtra  unmarshal value string to go types, panic on error
func MustUnmarshalWithExtra(o string) (interface{}, string) {
	rtn, extra := UnmarshalWithExtra(o)
	if err, ok := rtn.(error); ok {
		panic(err.Error() + ":" + o)
	}
	return rtn, extra
}

func int64ToRaw(i int64) string {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(i))
	return string(b)
}

func rawToInt64(s string) int64 {
	b := []byte(s)
	return int64(binary.LittleEndian.Uint64(b))
}

func boolToString(i bool) string {
	if i {
		return "t"
	}
	return "f"
}
