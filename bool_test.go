package delta

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/benpate/rosetta/convert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBool(t *testing.T) {

	value := NewBool(false)

	require.False(t, value.Value())
	require.True(t, value.NotChanged())
	require.False(t, value.IsChanged())

	value.Set(true)

	require.True(t, value.Value())
	require.False(t, value.NotChanged())
	require.True(t, value.IsChanged())
}

// TestBool_Original verifies that the baseline survives a change to the current value
func TestBool_Original(t *testing.T) {

	value := NewBool(false)
	require.False(t, value.Original())

	value.Set(true)

	require.True(t, value.Value())
	require.False(t, value.Original(), "the baseline does not move when the value does")
}

// TestBool_OriginalAfterUnmarshal is the case Original() exists for
func TestBool_OriginalAfterUnmarshal(t *testing.T) {

	document, err := bson.Marshal(bson.M{"isActive": true})
	require.NoError(t, err)

	result := struct {
		IsActive Bool `bson:"isActive"`
	}{}

	require.NoError(t, bson.Unmarshal(document, &result))
	require.True(t, result.IsActive.Original())

	result.IsActive.Set(false)

	require.True(t, result.IsActive.Original(), "the value the database still holds")
	require.True(t, result.IsActive.IsChanged())
}

/******************************************
 * BSON format guard
 ******************************************/

// Bool must satisfy both *Value BSON interfaces. It holds only unexported fields
// and has no plain MarshalBSON to fall back on, so a type that stops satisfying
// them is encoded by the default struct codec instead: a stored `true` becomes an empty document, with no error anywhere.
var (
	_ bson.ValueMarshaler   = Bool{}
	_ bson.ValueUnmarshaler = &Bool{}
)

// TestBool_Reset verifies that Reset moves the baseline to the current value, as a save does
func TestBool_Reset(t *testing.T) {

	value := NewBool(false)
	value.Set(true)
	require.True(t, value.IsChanged())
	require.Equal(t, false, value.Original())

	value.Reset()

	require.Equal(t, true, value.Value(), "Reset does not change the value")
	require.Equal(t, true, value.Original())
	require.False(t, value.IsChanged())
	require.True(t, value.NotChanged())

	// A second Reset changes nothing
	value.Reset()
	require.Equal(t, true, value.Original())

	// The next change is measured from the new baseline
	value.Set(false)
	require.True(t, value.IsChanged())
	require.Equal(t, true, value.Original())
}

// TestBool_Clear verifies that Clear zeroes the current value and keeps the baseline
func TestBool_Clear(t *testing.T) {

	value := NewBool(true)

	value.Clear()

	require.Equal(t, false, value.Value())
	require.True(t, value.IsZero())
	require.Equal(t, true, value.Original(), "Clear does not move the baseline")
	require.True(t, value.IsChanged())

	// Clearing a value that is already zero changes nothing
	empty := NewBool(false)
	empty.Clear()
	require.True(t, empty.IsZero())
	require.True(t, empty.NotChanged())
}

/******************************************
 * Interface guards
 ******************************************/

// Bool satisfies the JSON and schema interfaces that its callers rely on
var (
	_ json.Marshaler                   = Bool{}
	_ json.Unmarshaler                 = &Bool{}
	_ interface{ GetValue() any }      = Bool{}
	_ interface{ SetValue(any) error } = &Bool{}
	_ interface{ IsZero() bool }       = Bool{}
)

/******************************************
 * Test helpers
 ******************************************/

// requireBool asserts every reader on a Bool against the expected current and baseline values
func requireBool(t *testing.T, value Bool, current bool, original bool) {
	t.Helper()

	require.Equal(t, current, value.Value(), "Value")
	require.Equal(t, current, value.Bool(), "Bool")
	require.Equal(t, current, value.IsTrue(), "IsTrue")
	require.Equal(t, !current, value.IsFalse(), "IsFalse")
	require.Equal(t, !current, value.IsZero(), "IsZero")
	require.Equal(t, any(current), value.GetValue(), "GetValue")
	require.Equal(t, current, *value.Pointer(), "Pointer")
	require.Equal(t, original, value.Original(), "Original")
	require.Equal(t, current != original, value.IsChanged(), "IsChanged")
	require.Equal(t, current == original, value.NotChanged(), "NotChanged")
}

// eachBoolPair calls the function once for every combination of two booleans
func eachBoolPair(t *testing.T, fn func(t *testing.T, first bool, second bool)) {
	t.Helper()

	for _, first := range []bool{false, true} {
		for _, second := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/%v", first, second), func(t *testing.T) {
				fn(t, first, second)
			})
		}
	}
}

// isWellFormedBSON returns TRUE if the data is a structurally valid BSON document
func isWellFormedBSON(data []byte) bool {

	// RULE: The header must match the data, because the driver's validator panics when it does not
	if len(data) < 5 {
		return false
	}

	if int(binary.LittleEndian.Uint32(data)) != len(data) {
		return false
	}

	return bson.Raw(data).Validate() == nil
}

/******************************************
 * Data Manipulation Methods
 ******************************************/

// TestBool_ZeroValue verifies that an undeclared Bool is false and unchanged
func TestBool_ZeroValue(t *testing.T) {

	var value Bool

	requireBool(t, value, false, false)
	require.Equal(t, NewBool(false), value)
}

// TestBool_New verifies that a new Bool reports its value as both current and baseline
func TestBool_New(t *testing.T) {
	requireBool(t, NewBool(false), false, false)
	requireBool(t, NewBool(true), true, true)
}

// TestBool_Set verifies every combination of baseline and new value
func TestBool_Set(t *testing.T) {

	eachBoolPair(t, func(t *testing.T, baseline bool, next bool) {
		value := NewBool(baseline)
		value.Set(next)
		requireBool(t, value, next, baseline)

		// Returning to the baseline undoes the change
		value.Set(baseline)
		requireBool(t, value, baseline, baseline)
	})

	t.Run("zero value", func(t *testing.T) {
		var value Bool
		value.Set(true)
		requireBool(t, value, true, false)
	})
}

// TestBool_Pointer verifies that the pointer aliases the current value and nothing else
func TestBool_Pointer(t *testing.T) {

	t.Run("a write through the pointer is a tracked change", func(t *testing.T) {
		value := NewBool(false)
		*value.Pointer() = true
		requireBool(t, value, true, false)
	})

	t.Run("the pointer follows later changes", func(t *testing.T) {
		value := NewBool(false)
		pointer := value.Pointer()

		value.Set(true)
		require.True(t, *pointer)

		value.Clear()
		require.False(t, *pointer)

		require.NoError(t, value.SetValue("true"))
		require.True(t, *pointer)

		require.Same(t, pointer, value.Pointer())
	})

	t.Run("the pointer survives a Reset", func(t *testing.T) {
		value := NewBool(false)
		pointer := value.Pointer()
		value.Set(true)
		value.Reset()

		*pointer = false
		requireBool(t, value, false, true)
	})

	t.Run("the pointer never reaches the baseline", func(t *testing.T) {
		value := NewBool(true)
		*value.Pointer() = false
		require.True(t, value.Original())
	})
}

// TestBool_Copy verifies that a copied Bool is independent of its source
func TestBool_Copy(t *testing.T) {

	source := NewBool(false)
	source.Set(true)

	duplicate := source
	duplicate.Reset()
	duplicate.Set(false)

	requireBool(t, source, true, false)
	requireBool(t, duplicate, false, true)
}

// TestBool_Equality verifies that == compares the baseline as well as the current value
func TestBool_Equality(t *testing.T) {

	first, second := NewBool(true), NewBool(true)
	require.True(t, first == second)
	require.False(t, first == NewBool(false))

	changed := NewBool(false)
	changed.Set(true)
	require.False(t, NewBool(true) == changed, "same current value, different baseline")

	changed.Reset()
	require.True(t, NewBool(true) == changed)
}

// TestBool_ResetAndClear verifies Reset and Clear from every starting state
func TestBool_ResetAndClear(t *testing.T) {

	t.Run("reset", func(t *testing.T) {
		eachBoolPair(t, func(t *testing.T, current bool, original bool) {
			value := Bool{current: current, original: original}
			value.Reset()
			requireBool(t, value, current, current)

			value.Reset()
			requireBool(t, value, current, current)
		})
	})

	t.Run("clear", func(t *testing.T) {
		eachBoolPair(t, func(t *testing.T, current bool, original bool) {
			value := Bool{current: current, original: original}
			value.Clear()
			requireBool(t, value, false, original)

			value.Clear()
			requireBool(t, value, false, original)
		})
	})

	t.Run("clear then reset makes false the baseline", func(t *testing.T) {
		value := NewBool(true)
		value.Clear()
		value.Reset()
		requireBool(t, value, false, false)
	})

	t.Run("zero value", func(t *testing.T) {
		var value Bool
		value.Reset()
		value.Clear()
		requireBool(t, value, false, false)
	})
}

/******************************************
 * Schema Interfaces
 ******************************************/

// TestBool_GetValue verifies that GetValue returns a plain bool holding the current value
func TestBool_GetValue(t *testing.T) {

	value := NewBool(false)
	value.Set(true)

	result, isBool := value.GetValue().(bool)
	require.True(t, isBool)
	require.True(t, result)
}

// TestBool_SetValue verifies the conversion of each kind of input, and that the baseline holds
func TestBool_SetValue(t *testing.T) {

	// check runs one input against both baselines, so no result can pass by coincidence
	check := func(name string, input any, want bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			for _, baseline := range []bool{false, true} {
				value := Bool{current: !want, original: baseline}
				require.NoError(t, value.SetValue(input))
				requireBool(t, value, want, baseline)
			}
		})
	}

	check("true", true, true)
	check("false", false, false)
	check("string true", "true", true)
	check("string false", "false", false)
	check("int 1", 1, true)
	check("int 0", 0, false)
	check("int64 1", int64(1), true)
	check("float 1", 1.0, true)
	check("float 0", 0.0, false)
	check("delta.Bool true", NewBool(true), true)
	check("delta.Bool false", NewBool(false), false)

	// Anything rosetta cannot read as a boolean is stored as false, without an error
	check("nil", nil, false)
	check("empty string", "", false)
	check("uppercase string", "TRUE", false)
	check("string 1", "1", false)
	check("string yes", "yes", false)
	check("int 2", 2, false)
	check("int -1", -1, false)
	check("float 0.5", 0.5, false)
	check("struct", struct{}{}, false)
	check("long slice", []bool{true, true}, false)
}

/******************************************
 * JSON Serialization
 ******************************************/

// TestBool_MarshalJSON verifies that a Bool encodes as a bare JSON boolean from its current value
func TestBool_MarshalJSON(t *testing.T) {

	eachBoolPair(t, func(t *testing.T, current bool, original bool) {
		value := Bool{current: current, original: original}
		want := fmt.Sprint(current)

		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, want, string(encoded))

		direct, err := value.MarshalJSON()
		require.NoError(t, err)
		require.Equal(t, want, string(direct))

		viaPointer, err := json.Marshal(&value)
		require.NoError(t, err)
		require.Equal(t, want, string(viaPointer))

		requireBool(t, value, current, original)
	})
}

// TestBool_UnmarshalJSON verifies decoding onto a Bool from every starting state
func TestBool_UnmarshalJSON(t *testing.T) {

	check := func(name string, input string, want bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			eachBoolPair(t, func(t *testing.T, current bool, original bool) {
				value := Bool{current: current, original: original}
				require.NoError(t, value.UnmarshalJSON([]byte(input)))
				requireBool(t, value, want, want)
			})
		})
	}

	checkErr := func(name string, input []byte) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			eachBoolPair(t, func(t *testing.T, current bool, original bool) {
				value := Bool{current: current, original: original}
				err := value.UnmarshalJSON(input)
				require.Error(t, err)
				require.ErrorContains(t, err, "delta.Bool.UnmarshalJSON")
				requireBool(t, value, current, original)
			})
		})
	}

	check("true", `true`, true)
	check("false", `false`, false)
	check("surrounding whitespace", " \n\ttrue ", true)

	check("null becomes false", `null`, false)

	checkErr("nil input", nil)
	checkErr("empty input", []byte{})
	checkErr("string true", []byte(`"true"`))
	checkErr("empty string", []byte(`""`))
	checkErr("number 1", []byte(`1`))
	checkErr("number 0", []byte(`0`))
	checkErr("object", []byte(`{}`))
	checkErr("array", []byte(`[true]`))
	checkErr("truncated literal", []byte(`tru`))
	checkErr("uppercase literal", []byte(`TRUE`))
	checkErr("two literals", []byte(`true false`))
	checkErr("trailing data", []byte(`truex`))
	checkErr("mixed-case null", []byte(`Null`))
}

// TestBool_JSONStruct verifies how a Bool behaves as a field of a larger document
func TestBool_JSONStruct(t *testing.T) {

	type wrapped struct {
		Flag Bool `json:"flag"`
	}
	type plain struct {
		Flag bool `json:"flag"`
	}

	t.Run("wire format matches a plain bool field", func(t *testing.T) {
		for _, flag := range []bool{false, true} {
			want, err := json.Marshal(plain{Flag: flag})
			require.NoError(t, err)

			got, err := json.Marshal(wrapped{Flag: NewBool(flag)})
			require.NoError(t, err)
			require.Equal(t, string(want), string(got))
		}
	})

	t.Run("round trip", func(t *testing.T) {
		eachBoolPair(t, func(t *testing.T, current bool, original bool) {
			encoded, err := json.Marshal(wrapped{Flag: Bool{current: current, original: original}})
			require.NoError(t, err)

			result := wrapped{}
			require.NoError(t, json.Unmarshal(encoded, &result))
			requireBool(t, result.Flag, current, current)
		})
	})

	t.Run("omitempty does not omit, omitzero does", func(t *testing.T) {
		record := struct {
			Empty Bool `json:"empty,omitempty"`
			Zero  Bool `json:"zero,omitzero"`
			Full  Bool `json:"full,omitzero"`
		}{Full: NewBool(true)}

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.JSONEq(t, `{"empty":false,"full":true}`, string(encoded))
	})

	t.Run("a missing field is left alone", func(t *testing.T) {
		record := wrapped{Flag: Bool{current: true, original: false}}
		require.NoError(t, json.Unmarshal([]byte(`{"other":true}`), &record))
		requireBool(t, record.Flag, true, false)
	})

	t.Run("a null field becomes false", func(t *testing.T) {
		record := wrapped{Flag: Bool{current: true, original: true}}
		require.NoError(t, json.Unmarshal([]byte(`{"flag":null}`), &record))
		requireBool(t, record.Flag, false, false)
	})

	t.Run("a wrong type fails and leaves the field alone", func(t *testing.T) {
		record := wrapped{Flag: Bool{current: true, original: false}}
		require.Error(t, json.Unmarshal([]byte(`{"flag":"true"}`), &record))
		requireBool(t, record.Flag, true, false)
	})

	t.Run("pointer field", func(t *testing.T) {
		record := struct {
			Flag *Bool `json:"flag"`
		}{}

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.Equal(t, `{"flag":null}`, string(encoded))

		require.NoError(t, json.Unmarshal([]byte(`{"flag":true}`), &record))
		require.NotNil(t, record.Flag)
		requireBool(t, *record.Flag, true, true)

		require.NoError(t, json.Unmarshal([]byte(`{"flag":null}`), &record))
		require.Nil(t, record.Flag)
	})

	t.Run("slice and map", func(t *testing.T) {
		record := struct {
			List  []Bool          `json:"list"`
			Index map[string]Bool `json:"index"`
		}{}

		require.NoError(t, json.Unmarshal([]byte(`{"list":[true,false],"index":{"k":true}}`), &record))
		require.Equal(t, []Bool{NewBool(true), NewBool(false)}, record.List)
		require.Equal(t, map[string]Bool{"k": NewBool(true)}, record.Index)

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.JSONEq(t, `{"list":[true,false],"index":{"k":true}}`, string(encoded))
	})
}

/******************************************
 * BSON Serialization
 ******************************************/

// TestBool_MarshalBSONValue verifies that a Bool encodes as a BSON boolean from its current value
func TestBool_MarshalBSONValue(t *testing.T) {

	eachBoolPair(t, func(t *testing.T, current bool, original bool) {
		value := Bool{current: current, original: original}

		wantType, wantData, err := bson.MarshalValue(current)
		require.NoError(t, err)

		gotType, gotData, err := value.MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, bson.TypeBoolean, gotType)
		require.Equal(t, wantType, gotType)
		require.Equal(t, wantData, gotData)

		requireBool(t, value, current, original)
	})

	t.Run("literal output", func(t *testing.T) {
		_, data, err := NewBool(true).MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, []byte{1}, data)

		_, data, err = NewBool(false).MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, []byte{0}, data)
	})
}

// TestBool_UnmarshalBSONValue verifies decoding onto a Bool from every starting state
func TestBool_UnmarshalBSONValue(t *testing.T) {

	check := func(name string, source any, want bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			bsonType, data, err := bson.MarshalValue(source)
			require.NoError(t, err)

			eachBoolPair(t, func(t *testing.T, current bool, original bool) {
				value := Bool{current: current, original: original}
				require.NoError(t, value.UnmarshalBSONValue(bsonType, data))
				requireBool(t, value, want, want)
			})
		})
	}

	checkErr := func(name string, bsonType bsontype.Type, data []byte) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			eachBoolPair(t, func(t *testing.T, current bool, original bool) {
				value := Bool{current: current, original: original}
				err := value.UnmarshalBSONValue(bsonType, data)
				require.Error(t, err)
				require.ErrorContains(t, err, "delta.Bool.UnmarshalBSONValue")
				requireBool(t, value, current, original)
			})
		})
	}

	checkErrFrom := func(name string, source any) {
		t.Helper()
		bsonType, data, err := bson.MarshalValue(source)
		require.NoError(t, err)
		checkErr(name, bsonType, data)
	}

	check("true", true, true)
	check("false", false, false)

	// The driver's bool decoder reads any non-zero number as true, and an absent value as false
	check("int32 zero", int32(0), false)
	check("int32 one", int32(1), true)
	check("int32 seven", int32(7), true)
	check("int32 negative", int32(-1), true)
	check("int64 zero", int64(0), false)
	check("int64 one", int64(1), true)
	check("double zero", 0.0, false)
	check("double fraction", 0.5, true)
	check("null", primitive.Null{}, false)
	check("undefined", primitive.Undefined{}, false)

	checkErrFrom("string", "true")
	checkErrFrom("empty string", "")
	checkErrFrom("document", bson.M{"a": true})
	checkErrFrom("array", bson.A{true})
	checkErrFrom("object id", primitive.NewObjectID())
	checkErrFrom("datetime", primitive.DateTime(1))
	checkErrFrom("binary", primitive.Binary{Data: []byte{1}})
	checkErrFrom("decimal128", primitive.NewDecimal128(0, 1))

	checkErr("nil data", bson.TypeBoolean, nil)
	checkErr("empty data", bson.TypeBoolean, []byte{})
	checkErr("byte other than zero or one", bson.TypeBoolean, []byte{2})
	checkErr("truncated int32", bson.TypeInt32, []byte{1, 0})
	checkErr("truncated int64", bson.TypeInt64, []byte{1, 0, 0, 0})
	checkErr("truncated double", bson.TypeDouble, []byte{1})
	checkErr("type zero", bsontype.Type(0), nil)
	checkErr("unknown type", bsontype.Type(0x42), []byte{1})
}

// TestBool_BSONDocument verifies how a Bool behaves as a field of a stored record
func TestBool_BSONDocument(t *testing.T) {

	type wrapped struct {
		Flag Bool `bson:"flag"`
	}
	type plain struct {
		Flag bool `bson:"flag"`
	}

	t.Run("wire format matches a plain bool field", func(t *testing.T) {
		for _, flag := range []bool{false, true} {
			want, err := bson.Marshal(plain{Flag: flag})
			require.NoError(t, err)

			got, err := bson.Marshal(wrapped{Flag: NewBool(flag)})
			require.NoError(t, err)
			require.Equal(t, want, got)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		eachBoolPair(t, func(t *testing.T, current bool, original bool) {
			document, err := bson.Marshal(wrapped{Flag: Bool{current: current, original: original}})
			require.NoError(t, err)

			result := wrapped{Flag: Bool{current: !current, original: original}}
			require.NoError(t, bson.Unmarshal(document, &result))
			requireBool(t, result.Flag, current, current)

			plainResult := plain{}
			require.NoError(t, bson.Unmarshal(document, &plainResult))
			require.Equal(t, current, plainResult.Flag)
		})
	})

	t.Run("omitempty drops a false value, as it does for a plain bool", func(t *testing.T) {
		record := struct {
			Empty   Bool `bson:"empty,omitempty"`
			Cleared Bool `bson:"cleared,omitempty"`
			Filled  Bool `bson:"filled,omitempty"`
		}{Cleared: NewBool(true)}

		record.Cleared.Clear()
		record.Filled.Set(true)

		document, err := bson.Marshal(record)
		require.NoError(t, err)

		result := bson.M{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, bson.M{"filled": true}, result)
	})

	t.Run("a missing field is left alone", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"other": true})
		require.NoError(t, err)

		result := wrapped{Flag: Bool{current: true, original: false}}
		require.NoError(t, bson.Unmarshal(document, &result))
		requireBool(t, result.Flag, true, false)
	})

	t.Run("a null field becomes false", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"flag": nil})
		require.NoError(t, err)

		result := wrapped{Flag: Bool{current: true, original: true}}
		require.NoError(t, bson.Unmarshal(document, &result))
		requireBool(t, result.Flag, false, false)
	})

	t.Run("a wrong type fails and leaves the field alone", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"flag": "true"})
		require.NoError(t, err)

		result := wrapped{Flag: Bool{current: true, original: false}}
		require.Error(t, bson.Unmarshal(document, &result))
		requireBool(t, result.Flag, true, false)
	})

	t.Run("pointer field", func(t *testing.T) {
		type pointered struct {
			Flag *Bool `bson:"flag"`
		}

		value := NewBool(true)
		document, err := bson.Marshal(pointered{Flag: &value})
		require.NoError(t, err)

		result := pointered{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.NotNil(t, result.Flag)
		requireBool(t, *result.Flag, true, true)
	})

	t.Run("save and reload cycle", func(t *testing.T) {

		// Load a record, edit it, save it, and Reset, as a service does
		document, err := bson.Marshal(plain{Flag: false})
		require.NoError(t, err)

		record := wrapped{}
		require.NoError(t, bson.Unmarshal(document, &record))

		record.Flag.Set(true)
		requireBool(t, record.Flag, true, false)

		saved, err := bson.Marshal(record)
		require.NoError(t, err)

		record.Flag.Reset()
		requireBool(t, record.Flag, true, true)

		reloaded := wrapped{}
		require.NoError(t, bson.Unmarshal(saved, &reloaded))
		require.Equal(t, record, reloaded)
	})
}

/******************************************
 * Fuzz Tests
 ******************************************/

// FuzzBool_Operations runs random sequences of mutations against a two-variable model
func FuzzBool_Operations(f *testing.F) {

	f.Add(false, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	f.Add(true, []byte{2, 3, 0, 3})
	f.Add(true, []byte{8, 8, 3, 8, 9, 10, 11})
	f.Add(false, []byte{})

	f.Fuzz(func(t *testing.T, initial bool, operations []byte) {

		value := NewBool(initial)
		current, original := initial, initial
		requireBool(t, value, current, original)

		for _, operation := range operations {

			switch operation % 12 {

			case 0:
				value.Set(true)
				current = true

			case 1:
				value.Set(false)
				current = false

			case 2:
				value.Clear()
				current = false

			case 3:
				value.Reset()
				original = current

			case 4:
				*value.Pointer() = true
				current = true

			case 5:
				*value.Pointer() = false
				current = false

			case 6:
				require.NoError(t, value.SetValue("true"))
				current = true

			case 7:
				require.NoError(t, value.SetValue(0))
				current = false

			case 8:
				value.Set(!value.Value())
				current = !current

			case 9:
				value.Set(value.Original())
				current = original

			case 10:
				encoded, err := json.Marshal(value)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(encoded, &value))
				original = current

			case 11:
				bsonType, data, err := value.MarshalBSONValue()
				require.NoError(t, err)
				require.NoError(t, value.UnmarshalBSONValue(bsonType, data))
				original = current
			}

			requireBool(t, value, current, original)
		}
	})
}

// FuzzBool_SetValue verifies that SetValue agrees with rosetta for every primitive input
func FuzzBool_SetValue(f *testing.F) {

	f.Add("true", int64(1), 1.0, true, false)
	f.Add("false", int64(0), 0.0, false, true)
	f.Add("TRUE", int64(2), 0.5, true, true)
	f.Add("", int64(-1), -1.5, false, false)

	f.Fuzz(func(t *testing.T, text string, integer int64, float float64, boolean bool, baseline bool) {

		inputs := []any{text, integer, int(integer), int32(integer), float, float32(float), boolean, []byte(text), nil, &boolean}

		for _, input := range inputs {
			want := convert.Bool(input)

			value := Bool{current: !want, original: baseline}
			require.NoError(t, value.SetValue(input))
			requireBool(t, value, want, baseline)
		}

		// A bool always passes through untouched
		value := NewBool(baseline)
		require.NoError(t, value.SetValue(boolean))
		require.Equal(t, boolean, value.Value())
	})
}

// FuzzBool_UnmarshalJSON verifies that arbitrary JSON decodes exactly as it does for a plain bool
func FuzzBool_UnmarshalJSON(f *testing.F) {

	seeds := []string{`true`, `false`, `null`, `"true"`, `1`, `0`, `{}`, `[]`, ``, ` true `, `tru`, `TRUE`, `true false`, `truex`}
	for _, seed := range seeds {
		f.Add([]byte(seed), true)
		f.Add([]byte(seed), false)
	}

	f.Fuzz(func(t *testing.T, data []byte, baseline bool) {

		// A plain bool that starts false is the oracle, which reads a JSON null as false
		plain := false
		value := Bool{current: !baseline, original: baseline}

		if err := json.Unmarshal(data, &plain); err != nil {
			require.Error(t, value.UnmarshalJSON(data))
			requireBool(t, value, !baseline, baseline)
			return
		}

		require.NoError(t, value.UnmarshalJSON(data))
		requireBool(t, value, plain, plain)

		// The same input reaches the same result through encoding/json
		viaPackage := Bool{current: !baseline, original: baseline}
		require.NoError(t, json.Unmarshal(data, &viaPackage))
		require.Equal(t, value, viaPackage)
	})
}

// FuzzBool_UnmarshalJSONDocument verifies that a Bool field decodes as a plain bool field does
func FuzzBool_UnmarshalJSONDocument(f *testing.F) {

	seeds := []string{
		`{"value":true}`, `{"value":false}`, `{"value":null}`, `{"value":1}`, `{}`,
		`{"value":true,"value":false}`, `{"value":true,"value":null}`, `{"VALUE":true}`,
		`{"value":"true"}`, `{"value":true`, `[]`, `null`, `{"other":1,"value":true}`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		// A pointer is the oracle, because it is the plain type that a JSON null resets
		plain := struct {
			Value *bool `json:"value"`
		}{}
		plainErr := json.Unmarshal(data, &plain)

		wrapped := struct {
			Value Bool `json:"value"`
		}{}
		wrappedErr := json.Unmarshal(data, &wrapped)

		require.Equal(t, plainErr == nil, wrappedErr == nil, "plain: %v, wrapped: %v", plainErr, wrappedErr)

		// Baseline and current value always move together during a load
		require.True(t, wrapped.Value.NotChanged())

		if plainErr != nil {
			return
		}

		want := false
		if plain.Value != nil {
			want = *plain.Value
		}

		requireBool(t, wrapped.Value, want, want)
	})
}

// FuzzBool_UnmarshalBSONValue verifies that arbitrary BSON decodes exactly as it does for a plain bool
func FuzzBool_UnmarshalBSONValue(f *testing.F) {

	sources := []any{
		true, false, int32(0), int32(7), int64(0), int64(-1), 0.0, 0.5, primitive.Null{},
		primitive.Undefined{}, "true", "", bson.M{"a": true}, bson.A{true}, primitive.NewObjectID(),
		primitive.DateTime(1), primitive.Binary{Data: []byte{1}}, primitive.NewDecimal128(0, 1),
	}
	for _, source := range sources {
		bsonType, data, err := bson.MarshalValue(source)
		require.NoError(f, err)
		f.Add(byte(bsonType), data, true)
		f.Add(byte(bsonType), data, false)
	}

	f.Add(byte(bson.TypeBoolean), []byte{2}, true)
	f.Add(byte(bson.TypeBoolean), []byte{}, false)
	f.Add(byte(bson.TypeInt32), []byte{1, 0}, false)
	f.Add(byte(0), []byte{}, true)

	f.Fuzz(func(t *testing.T, bsonType byte, data []byte, baseline bool) {

		// A plain bool that starts at the baseline is the oracle
		plain := baseline
		value := Bool{current: !baseline, original: baseline}

		if err := bson.UnmarshalValue(bsontype.Type(bsonType), data, &plain); err != nil {
			require.Error(t, value.UnmarshalBSONValue(bsontype.Type(bsonType), data))
			requireBool(t, value, !baseline, baseline)
			return
		}

		require.NoError(t, value.UnmarshalBSONValue(bsontype.Type(bsonType), data))
		requireBool(t, value, plain, plain)
	})
}

// FuzzBool_UnmarshalBSONDocument verifies that a Bool field decodes as a plain bool field does
func FuzzBool_UnmarshalBSONDocument(f *testing.F) {

	sources := []bson.M{
		{"value": true}, {"value": false}, {"value": nil}, {"value": 12}, {}, {"other": true},
		{"value": "true"}, {"value": bson.M{"a": true}}, {"value": bson.A{true}}, {"value": 0.5},
		{"value": int64(0)}, {"value": primitive.Undefined{}},
	}
	for _, source := range sources {
		document, err := bson.Marshal(source)
		require.NoError(f, err)
		f.Add(document)
	}

	f.Add([]byte{})
	f.Add([]byte{5, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {

		// RULE: Only well-formed documents are decoded, because mongo-driver v1 panics
		// or loops forever on a negative length, with or without a delta type present.
		if !isWellFormedBSON(data) {
			t.Skip("malformed document")
		}

		plain := struct {
			Value bool `bson:"value"`
		}{}
		plainErr := bson.Unmarshal(data, &plain)

		wrapped := struct {
			Value Bool `bson:"value"`
		}{}
		wrappedErr := bson.Unmarshal(data, &wrapped)

		require.Equal(t, plainErr == nil, wrappedErr == nil, "plain: %v, wrapped: %v", plainErr, wrappedErr)

		// Baseline and current value always move together during a load
		require.True(t, wrapped.Value.NotChanged())

		if plainErr == nil {
			requireBool(t, wrapped.Value, plain.Value, plain.Value)
		}
	})
}

/******************************************
 * Examples
 ******************************************/

// ExampleBool shows a change being tracked against the value that was loaded
func ExampleBool() {

	isPublic := NewBool(false)
	fmt.Println(isPublic.IsChanged())

	isPublic.Set(true)
	fmt.Println(isPublic.IsChanged())
	fmt.Println(isPublic.Value())
	fmt.Println(isPublic.Original())

	// Output:
	// false
	// true
	// true
	// false
}

// ExampleBool_Reset shows the baseline moving once the caller has saved the value
func ExampleBool_Reset() {

	isPublic := NewBool(false)
	isPublic.Set(true)

	isPublic.Reset()
	fmt.Println(isPublic.IsChanged())
	fmt.Println(isPublic.Original())

	// Output:
	// false
	// true
}
