package delta

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestObjectId(t *testing.T) {

	value := NewObjectID(primitive.NewObjectID())

	require.False(t, value.Value().IsZero())
	require.True(t, value.NotChanged())
	require.False(t, value.IsChanged())

	value.Set(primitive.NilObjectID)

	require.True(t, value.Value().IsZero())
	require.False(t, value.NotChanged())
	require.True(t, value.IsChanged())
}

// TestObjectID_Original verifies that the baseline survives a change to the current value
func TestObjectID_Original(t *testing.T) {

	first := primitive.NewObjectID()
	second := primitive.NewObjectID()

	value := NewObjectID(first)
	require.Equal(t, first, value.Original())

	value.Set(second)

	require.Equal(t, second, value.Value())
	require.Equal(t, first, value.Original(), "the baseline does not move when the value does")
}

// TestObjectID_OriginalAfterUnmarshal is the case Original() exists for
func TestObjectID_OriginalAfterUnmarshal(t *testing.T) {

	stored := primitive.NewObjectID()

	document, err := bson.Marshal(bson.M{"parentId": stored})
	require.NoError(t, err)

	result := struct {
		ParentID ObjectID `bson:"parentId"`
	}{}

	require.NoError(t, bson.Unmarshal(document, &result))
	require.Equal(t, stored, result.ParentID.Original())

	result.ParentID.Set(primitive.NewObjectID())

	require.Equal(t, stored, result.ParentID.Original(), "the value the database still holds")
	require.True(t, result.ParentID.IsChanged())
}

/******************************************
 * BSON format guard
 ******************************************/

// ObjectID must satisfy both *Value BSON interfaces. It holds only unexported fields
// and has no plain MarshalBSON to fall back on, so a type that stops satisfying
// them is encoded by the default struct codec instead: a stored ObjectID becomes an empty document, with no error anywhere.
var (
	_ bson.ValueMarshaler   = ObjectID{}
	_ bson.ValueUnmarshaler = &ObjectID{}
)

// TestObjectID_Reset verifies that Reset moves the baseline to the current value, as a save does
func TestObjectID_Reset(t *testing.T) {

	first := primitive.NewObjectID()
	second := primitive.NewObjectID()

	value := NewObjectID(first)
	value.Set(second)
	require.True(t, value.IsChanged())
	require.Equal(t, first, value.Original())

	value.Reset()

	require.Equal(t, second, value.Value(), "Reset does not change the value")
	require.Equal(t, second, value.Original())
	require.False(t, value.IsChanged())

	// The next change is measured from the new baseline
	value.Set(first)
	require.True(t, value.IsChanged())
	require.Equal(t, second, value.Original())
}

// TestObjectID_Clear verifies that Clear zeroes the current value and keeps the baseline
func TestObjectID_Clear(t *testing.T) {

	id := primitive.NewObjectID()
	value := NewObjectID(id)

	value.Clear()

	require.Equal(t, primitive.NilObjectID, value.Value())
	require.True(t, value.IsZero())
	require.Equal(t, id, value.Original(), "Clear does not move the baseline")
	require.True(t, value.IsChanged())

	// Clearing a value that is already zero changes nothing
	empty := NewObjectID(primitive.NilObjectID)
	empty.Clear()
	require.True(t, empty.IsZero())
	require.True(t, empty.NotChanged())
}

/******************************************
 * Interface guards
 ******************************************/

// ObjectID satisfies the JSON, formatting, and schema interfaces that its callers rely on
var (
	_ json.Marshaler                   = ObjectID{}
	_ json.Unmarshaler                 = &ObjectID{}
	_ fmt.Stringer                     = ObjectID{}
	_ interface{ GetValue() any }      = ObjectID{}
	_ interface{ SetValue(any) error } = &ObjectID{}
	_ interface{ IsZero() bool }       = ObjectID{}
)

/******************************************
 * Test helpers
 ******************************************/

// Fixed identifiers used as the current value, the baseline, and a third distinct value
var (
	testObjectID     = primitive.ObjectID{0x6b, 0x04, 0xf6, 0x47, 0x36, 0x05, 0x8a, 0x54, 0x30, 0x06, 0xf9, 0xa6}
	testObjectIDBase = primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c}
	testObjectIDCurr = primitive.ObjectID{0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc}
)

// testObjectIDHex is the hex form of testObjectID
const testObjectIDHex = "6b04f64736058a543006f9a6"

// objectIDSample is one named input that the ObjectID tests are run against
type objectIDSample struct {
	name  string
	value primitive.ObjectID
}

// objectIDSamples returns one value from each class of ObjectID worth testing
func objectIDSamples() []objectIDSample {
	return []objectIDSample{
		{"nil", primitive.NilObjectID},
		{"typical", testObjectID},
		{"all ones", primitive.ObjectID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{"lowest", primitive.ObjectID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
		{"highest byte only", primitive.ObjectID{0x80}},
		{"printable ascii", primitive.ObjectID([]byte("abcdefghijkl"))},
		{"quotes and braces", primitive.ObjectID([]byte(`"{$oid:"}\[]`))},
		{"generated", primitive.NewObjectID()},
	}
}

// objectIDFromBytes builds an ObjectID from fuzz input of any length
func objectIDFromBytes(data []byte) primitive.ObjectID {
	result := primitive.ObjectID{}
	copy(result[:], data)
	return result
}

// requireObjectID asserts every reader on an ObjectID against the expected current and baseline values
func requireObjectID(t *testing.T, value ObjectID, current primitive.ObjectID, original primitive.ObjectID) {
	t.Helper()

	require.Equal(t, current, value.Value(), "Value")
	require.Equal(t, current.Hex(), value.Hex(), "Hex")
	require.Equal(t, current.Hex(), value.String(), "String")
	require.Equal(t, any(current), value.GetValue(), "GetValue")
	require.Equal(t, current, *value.Pointer(), "Pointer")
	require.Equal(t, original, value.Original(), "Original")
	require.Equal(t, current != original, value.IsChanged(), "IsChanged")
	require.Equal(t, current == original, value.NotChanged(), "NotChanged")
	require.Equal(t, current == primitive.NilObjectID, value.IsZero(), "IsZero")
}

// changedObjectID returns an ObjectID whose current value and baseline differ
func changedObjectID() ObjectID {
	return ObjectID{current: testObjectIDCurr, original: testObjectIDBase}
}

/******************************************
 * Data Manipulation Methods
 ******************************************/

// TestObjectID_ZeroValue verifies that an undeclared ObjectID is nil and unchanged
func TestObjectID_ZeroValue(t *testing.T) {

	var value ObjectID

	requireObjectID(t, value, primitive.NilObjectID, primitive.NilObjectID)
	require.Equal(t, NewObjectID(primitive.NilObjectID), value)
	require.Equal(t, "000000000000000000000000", value.Hex())
}

// TestObjectID_New verifies that a new ObjectID reports its value as both current and baseline
func TestObjectID_New(t *testing.T) {

	for _, sample := range objectIDSamples() {
		t.Run(sample.name, func(t *testing.T) {
			requireObjectID(t, NewObjectID(sample.value), sample.value, sample.value)
		})
	}
}

// TestObjectID_Hex verifies the hex and string forms, which differ from the driver's String
func TestObjectID_Hex(t *testing.T) {

	value := NewObjectID(testObjectID)

	require.Equal(t, testObjectIDHex, value.Hex())
	require.Equal(t, testObjectIDHex, value.String())
	require.Equal(t, testObjectIDHex, fmt.Sprint(value))
	require.Equal(t, `ObjectID("`+testObjectIDHex+`")`, testObjectID.String(), "the driver wraps its own String")

	// The text forms follow the current value, not the baseline
	value.Set(testObjectIDCurr)
	require.Equal(t, testObjectIDCurr.Hex(), value.Hex())
	require.Equal(t, testObjectIDCurr.Hex(), value.String())
}

// TestObjectID_Set verifies how Set moves the current value against a fixed baseline
func TestObjectID_Set(t *testing.T) {

	for _, sample := range objectIDSamples() {
		t.Run("sample/"+sample.name, func(t *testing.T) {
			value := NewObjectID(testObjectIDBase)
			value.Set(sample.value)
			requireObjectID(t, value, sample.value, testObjectIDBase)
		})
	}

	t.Run("same value is not a change", func(t *testing.T) {
		value := NewObjectID(testObjectID)
		value.Set(testObjectID)
		requireObjectID(t, value, testObjectID, testObjectID)
	})

	t.Run("returning to the baseline undoes the change", func(t *testing.T) {
		value := NewObjectID(testObjectIDBase)
		value.Set(testObjectID)
		require.True(t, value.IsChanged())
		value.Set(testObjectIDBase)
		requireObjectID(t, value, testObjectIDBase, testObjectIDBase)
	})

	t.Run("the baseline is not the previous value", func(t *testing.T) {
		value := NewObjectID(testObjectIDBase)
		value.Set(testObjectID)
		value.Set(testObjectIDCurr)
		requireObjectID(t, value, testObjectIDCurr, testObjectIDBase)
	})

	t.Run("a single differing bit is a change", func(t *testing.T) {
		for index := range testObjectID {
			other := testObjectID
			other[index] ^= 0x01

			value := NewObjectID(testObjectID)
			value.Set(other)
			require.True(t, value.IsChanged(), "byte %d", index)
		}
	})

	t.Run("zero value", func(t *testing.T) {
		var value ObjectID
		value.Set(testObjectID)
		requireObjectID(t, value, testObjectID, primitive.NilObjectID)
	})
}

// TestObjectID_Pointer verifies that the pointer aliases the current value and nothing else
func TestObjectID_Pointer(t *testing.T) {

	t.Run("a write through the pointer is a tracked change", func(t *testing.T) {
		value := NewObjectID(testObjectIDBase)
		*value.Pointer() = testObjectID
		requireObjectID(t, value, testObjectID, testObjectIDBase)
	})

	t.Run("a single byte written through the pointer is a tracked change", func(t *testing.T) {
		value := NewObjectID(testObjectIDBase)
		value.Pointer()[11] = 0xff

		want := testObjectIDBase
		want[11] = 0xff
		requireObjectID(t, value, want, testObjectIDBase)
	})

	t.Run("the pointer follows later changes", func(t *testing.T) {
		value := NewObjectID(testObjectIDBase)
		pointer := value.Pointer()

		value.Set(testObjectID)
		require.Equal(t, testObjectID, *pointer)

		value.Clear()
		require.Equal(t, primitive.NilObjectID, *pointer)

		require.NoError(t, value.SetValue(testObjectIDHex))
		require.Equal(t, testObjectID, *pointer)

		require.Same(t, pointer, value.Pointer())
	})

	t.Run("the pointer survives a Reset", func(t *testing.T) {
		value := NewObjectID(testObjectIDBase)
		pointer := value.Pointer()
		value.Set(testObjectID)
		value.Reset()

		*pointer = testObjectIDCurr
		requireObjectID(t, value, testObjectIDCurr, testObjectID)
	})

	t.Run("the value returned by Value is a copy", func(t *testing.T) {
		value := NewObjectID(testObjectID)

		copied := value.Value()
		copied[0] = 0xff

		original := value.Original()
		original[0] = 0xff

		requireObjectID(t, value, testObjectID, testObjectID)
	})
}

// TestObjectID_Copy verifies that a copied ObjectID is independent of its source
func TestObjectID_Copy(t *testing.T) {

	source := NewObjectID(testObjectIDBase)
	source.Set(testObjectIDCurr)

	duplicate := source
	duplicate.Set(testObjectID)
	duplicate.Reset()

	requireObjectID(t, source, testObjectIDCurr, testObjectIDBase)
	requireObjectID(t, duplicate, testObjectID, testObjectID)
}

// TestObjectID_Equality verifies that == compares the baseline as well as the current value
func TestObjectID_Equality(t *testing.T) {

	first, second := NewObjectID(testObjectID), NewObjectID(testObjectID)
	require.True(t, first == second)
	require.False(t, first == NewObjectID(testObjectIDBase))

	changed := NewObjectID(testObjectIDBase)
	changed.Set(testObjectID)
	require.False(t, NewObjectID(testObjectID) == changed, "same current value, different baseline")

	changed.Reset()
	require.True(t, NewObjectID(testObjectID) == changed)
}

// TestObjectID_ResetAndClear verifies how Reset and Clear combine with each other
func TestObjectID_ResetAndClear(t *testing.T) {

	t.Run("reset on an unchanged value changes nothing", func(t *testing.T) {
		value := NewObjectID(testObjectID)
		value.Reset()
		requireObjectID(t, value, testObjectID, testObjectID)
	})

	t.Run("reset twice is the same as once", func(t *testing.T) {
		value := changedObjectID()
		value.Reset()
		value.Reset()
		requireObjectID(t, value, testObjectIDCurr, testObjectIDCurr)
	})

	t.Run("clear then reset makes nil the baseline", func(t *testing.T) {
		value := NewObjectID(testObjectID)
		value.Clear()
		value.Reset()
		requireObjectID(t, value, primitive.NilObjectID, primitive.NilObjectID)
	})

	t.Run("clear twice is the same as once", func(t *testing.T) {
		value := NewObjectID(testObjectID)
		value.Clear()
		value.Clear()
		requireObjectID(t, value, primitive.NilObjectID, testObjectID)
	})

	t.Run("zero value", func(t *testing.T) {
		var value ObjectID
		value.Reset()
		value.Clear()
		requireObjectID(t, value, primitive.NilObjectID, primitive.NilObjectID)
	})

	t.Run("every sample", func(t *testing.T) {
		for _, sample := range objectIDSamples() {
			value := NewObjectID(testObjectIDBase)
			value.Set(sample.value)
			value.Reset()
			requireObjectID(t, value, sample.value, sample.value)

			value.Clear()
			requireObjectID(t, value, primitive.NilObjectID, sample.value)
		}
	})
}

/******************************************
 * Schema Interfaces
 ******************************************/

// TestObjectID_GetValue verifies that GetValue returns a primitive.ObjectID holding the current value
func TestObjectID_GetValue(t *testing.T) {

	value := NewObjectID(testObjectIDBase)
	value.Set(testObjectID)

	result, isObjectID := value.GetValue().(primitive.ObjectID)
	require.True(t, isObjectID)
	require.Equal(t, testObjectID, result)
}

// TestObjectID_SetValue verifies each accepted input, each rejected input, and that the baseline holds
func TestObjectID_SetValue(t *testing.T) {

	check := func(name string, input any, want primitive.ObjectID) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedObjectID()
			require.NoError(t, value.SetValue(input))
			requireObjectID(t, value, want, testObjectIDBase)
		})
	}

	checkErr := func(name string, input any) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedObjectID()
			err := value.SetValue(input)
			require.Error(t, err)
			require.ErrorContains(t, err, "delta.ObjectID.SetValue")
			require.Equal(t, changedObjectID(), value, "a rejected input leaves the ObjectID as it was")
		})
	}

	pointer := testObjectID
	hexPointer := testObjectIDHex

	check("primitive.ObjectID", testObjectID, testObjectID)
	check("nil primitive.ObjectID", primitive.NilObjectID, primitive.NilObjectID)
	check("pointer to primitive.ObjectID", &pointer, testObjectID)
	check("hex string", testObjectIDHex, testObjectID)
	check("uppercase hex string", strings.ToUpper(testObjectIDHex), testObjectID)
	check("all-zero hex string", "000000000000000000000000", primitive.NilObjectID)
	check("hex bytes", []byte(testObjectIDHex), testObjectID)
	check("delta.ObjectID", NewObjectID(testObjectID), testObjectID)
	check("delta.String holding hex", NewString(testObjectIDHex), testObjectID)

	for _, sample := range objectIDSamples() {
		check("sample/"+sample.name, sample.value, sample.value)
		check("sample hex/"+sample.name, sample.value.Hex(), sample.value)
	}

	// An empty input clears the value
	check("nil", nil, primitive.NilObjectID)
	check("empty string", "", primitive.NilObjectID)
	check("empty bytes", []byte{}, primitive.NilObjectID)
	check("nil bytes", []byte(nil), primitive.NilObjectID)
	check("delta.String holding nothing", NewString(""), primitive.NilObjectID)

	check("nil pointer to primitive.ObjectID", (*primitive.ObjectID)(nil), primitive.NilObjectID)

	// A value with no string form is rejected rather than read as empty
	checkErr("empty string slice", []string{})
	checkErr("pointer to string", &hexPointer)
	checkErr("whitespace", " ")

	checkErr("short hex", testObjectIDHex[:23])
	checkErr("long hex", testObjectIDHex+"0")
	checkErr("non-hex of the right length", "zzzzzzzzzzzzzzzzzzzzzzzz")
	checkErr("hex with one bad trailing character", testObjectIDHex[:23]+"z")
	checkErr("hex with surrounding space", " "+testObjectIDHex+" ")
	checkErr("hex with a 0x prefix", "0x"+testObjectIDHex)
	checkErr("the driver's own String form", testObjectID.String())
	checkErr("extended json", `{"$oid":"`+testObjectIDHex+`"}`)
	checkErr("raw twelve bytes", testObjectID[:])
	checkErr("byte array", [12]byte(testObjectID))
	checkErr("int", 42)
	checkErr("bool", true)
	checkErr("float", 1.5)
	checkErr("struct", struct{}{})
}

/******************************************
 * JSON Serialization
 ******************************************/

// TestObjectID_MarshalJSON verifies that an ObjectID encodes exactly as a primitive.ObjectID does
func TestObjectID_MarshalJSON(t *testing.T) {

	for _, sample := range objectIDSamples() {
		t.Run(sample.name, func(t *testing.T) {
			want, err := json.Marshal(sample.value)
			require.NoError(t, err)

			got, err := json.Marshal(NewObjectID(sample.value))
			require.NoError(t, err)
			require.Equal(t, string(want), string(got))
			require.Equal(t, `"`+sample.value.Hex()+`"`, string(got))

			direct, err := NewObjectID(sample.value).MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, string(want), string(direct))
		})
	}

	t.Run("the current value is written, not the baseline", func(t *testing.T) {
		value := NewObjectID(testObjectIDBase)
		value.Set(testObjectID)

		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, `"`+testObjectIDHex+`"`, string(encoded))
	})

	t.Run("through a pointer", func(t *testing.T) {
		value := NewObjectID(testObjectID)
		encoded, err := json.Marshal(&value)
		require.NoError(t, err)
		require.Equal(t, `"`+testObjectIDHex+`"`, string(encoded))
	})
}

// TestObjectID_UnmarshalJSON verifies decoding onto an ObjectID that already holds an unsaved change
func TestObjectID_UnmarshalJSON(t *testing.T) {

	check := func(name string, input string, want primitive.ObjectID) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedObjectID()
			require.NoError(t, value.UnmarshalJSON([]byte(input)))
			requireObjectID(t, value, want, want)
		})
	}

	checkErr := func(name string, input []byte) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedObjectID()
			err := value.UnmarshalJSON(input)
			require.Error(t, err)
			require.ErrorContains(t, err, "delta.ObjectID.UnmarshalJSON")
			require.Equal(t, changedObjectID(), value, "a failed decode leaves the ObjectID as it was")
		})
	}

	check("hex string", `"`+testObjectIDHex+`"`, testObjectID)
	check("uppercase hex string", `"`+strings.ToUpper(testObjectIDHex)+`"`, testObjectID)
	check("surrounding whitespace", ` "`+testObjectIDHex+`" `, testObjectID)
	check("all-zero hex string", `"000000000000000000000000"`, primitive.NilObjectID)
	check("empty string becomes nil", `""`, primitive.NilObjectID)
	check("null becomes nil", `null`, primitive.NilObjectID)

	for _, sample := range objectIDSamples() {
		check("sample/"+sample.name, `"`+sample.value.Hex()+`"`, sample.value)
	}

	// RULE: Only a hex string is accepted, so the driver's other input forms are errors
	checkErr("extended json", []byte(`{"$oid":"`+testObjectIDHex+`"}`))
	checkErr("extended json with an empty string", []byte(`{"$oid":""}`))
	checkErr("twelve-byte string literal", []byte(`"abcdefghij"`))
	checkErr("twelve-character string", []byte(`"abcdefghijkl"`))
	checkErr("twelve-digit number", []byte(`123456789012`))
	checkErr("whitespace string", []byte(`" "`))
	checkErr("hex with inner whitespace", []byte(`" `+testObjectIDHex+`"`))
	checkErr("the driver's own String form", []byte(`"ObjectID(\"`+testObjectIDHex+`\")"`))

	checkErr("nil input", nil)
	checkErr("empty input", []byte{})
	checkErr("short hex", []byte(`"`+testObjectIDHex[:23]+`"`))
	checkErr("long hex", []byte(`"`+testObjectIDHex+`0"`))
	checkErr("non-hex of the right length", []byte(`"zzzzzzzzzzzzzzzzzzzzzzzz"`))
	checkErr("number", []byte(`12`))
	checkErr("bool", []byte(`true`))
	checkErr("array", []byte(`["`+testObjectIDHex+`"]`))
	checkErr("empty object", []byte(`{}`))
	checkErr("extended json with a number", []byte(`{"$oid":12}`))
	checkErr("extended json with bad hex", []byte(`{"$oid":"zz"}`))
	checkErr("extended json with the wrong key", []byte(`{"oid":"`+testObjectIDHex+`"}`))
	checkErr("unterminated string", []byte(`"`+testObjectIDHex))
	checkErr("trailing data", []byte(`"`+testObjectIDHex+`"x`))
	checkErr("mixed-case null", []byte(`NULL`))

	// Hex that turns invalid part-way through must not leave its valid prefix in the baseline
	checkErr("hex with one bad trailing character", []byte(`"aaaaaaaaaaaaaaaaaaaaaazz"`))
	checkErr("hex with one bad leading character", []byte(`"zaaaaaaaaaaaaaaaaaaaaaaa"`))
	checkErr("hex with one bad middle character", []byte(`"aaaaaaaaaaaazaaaaaaaaaaa"`))
	checkErr("extended json with partly valid hex", []byte(`{"$oid":"aaaaaaaaaaaaaaaaaaaaaazz"}`))

	t.Run("a failed decode does not mark a fresh ObjectID as changed", func(t *testing.T) {
		record := struct {
			ParentID ObjectID `json:"parentId"`
		}{}

		require.Error(t, json.Unmarshal([]byte(`{"parentId":"aaaaaaaaaaaaaaaaaaaaaazz"}`), &record))
		requireObjectID(t, record.ParentID, primitive.NilObjectID, primitive.NilObjectID)
	})
}

// TestObjectID_JSONRoundTrip verifies that every ObjectID survives a trip through JSON unchanged
func TestObjectID_JSONRoundTrip(t *testing.T) {

	for _, sample := range objectIDSamples() {
		t.Run(sample.name, func(t *testing.T) {
			source := NewObjectID(testObjectIDBase)
			source.Set(sample.value)

			encoded, err := json.Marshal(source)
			require.NoError(t, err)

			result := changedObjectID()
			require.NoError(t, json.Unmarshal(encoded, &result))
			requireObjectID(t, result, sample.value, sample.value)
		})
	}
}

// TestObjectID_JSONStruct verifies how an ObjectID behaves as a field of a larger document
func TestObjectID_JSONStruct(t *testing.T) {

	type wrapped struct {
		ID ObjectID `json:"id"`
	}
	type plain struct {
		ID primitive.ObjectID `json:"id"`
	}

	t.Run("wire format matches a primitive.ObjectID field", func(t *testing.T) {
		for _, sample := range objectIDSamples() {
			want, err := json.Marshal(plain{ID: sample.value})
			require.NoError(t, err)

			got, err := json.Marshal(wrapped{ID: NewObjectID(sample.value)})
			require.NoError(t, err)
			require.Equal(t, string(want), string(got), sample.name)
		}
	})

	t.Run("omitempty does not omit, omitzero does", func(t *testing.T) {
		record := struct {
			Empty ObjectID `json:"empty,omitempty"`
			Zero  ObjectID `json:"zero,omitzero"`
			Full  ObjectID `json:"full,omitzero"`
		}{Full: NewObjectID(testObjectID)}

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.JSONEq(t, `{"empty":"000000000000000000000000","full":"`+testObjectIDHex+`"}`, string(encoded))
	})

	t.Run("a missing field is left alone", func(t *testing.T) {
		record := wrapped{ID: changedObjectID()}
		require.NoError(t, json.Unmarshal([]byte(`{"other":"x"}`), &record))
		require.Equal(t, changedObjectID(), record.ID)
	})

	t.Run("a null field becomes nil", func(t *testing.T) {
		record := wrapped{ID: changedObjectID()}
		require.NoError(t, json.Unmarshal([]byte(`{"id":null}`), &record))
		requireObjectID(t, record.ID, primitive.NilObjectID, primitive.NilObjectID)
	})

	t.Run("extended json fails and leaves the field alone", func(t *testing.T) {
		record := wrapped{ID: changedObjectID()}
		require.Error(t, json.Unmarshal([]byte(`{"id":{"$oid":"`+testObjectIDHex+`"}}`), &record))
		require.Equal(t, changedObjectID(), record.ID)
	})

	t.Run("a wrong type fails and leaves the field alone", func(t *testing.T) {
		record := wrapped{ID: changedObjectID()}
		require.Error(t, json.Unmarshal([]byte(`{"id":12}`), &record))
		require.Equal(t, changedObjectID(), record.ID)
	})

	t.Run("pointer field", func(t *testing.T) {
		record := struct {
			ID *ObjectID `json:"id"`
		}{}

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.Equal(t, `{"id":null}`, string(encoded))

		require.NoError(t, json.Unmarshal([]byte(`{"id":"`+testObjectIDHex+`"}`), &record))
		require.NotNil(t, record.ID)
		requireObjectID(t, *record.ID, testObjectID, testObjectID)

		require.NoError(t, json.Unmarshal([]byte(`{"id":null}`), &record))
		require.Nil(t, record.ID)
	})

	t.Run("slice and map", func(t *testing.T) {
		record := struct {
			List  []ObjectID          `json:"list"`
			Index map[string]ObjectID `json:"index"`
		}{}

		input := `{"list":["` + testObjectIDHex + `",""],"index":{"k":"` + testObjectIDHex + `"}}`
		require.NoError(t, json.Unmarshal([]byte(input), &record))
		require.Equal(t, []ObjectID{NewObjectID(testObjectID), NewObjectID(primitive.NilObjectID)}, record.List)
		require.Equal(t, map[string]ObjectID{"k": NewObjectID(testObjectID)}, record.Index)
	})
}

/******************************************
 * BSON Serialization
 ******************************************/

// TestObjectID_MarshalBSONValue verifies that an ObjectID encodes exactly as a primitive.ObjectID does
func TestObjectID_MarshalBSONValue(t *testing.T) {

	for _, sample := range objectIDSamples() {
		t.Run(sample.name, func(t *testing.T) {
			wantType, wantData, err := bson.MarshalValue(sample.value)
			require.NoError(t, err)

			gotType, gotData, err := NewObjectID(sample.value).MarshalBSONValue()
			require.NoError(t, err)
			require.Equal(t, bson.TypeObjectID, gotType)
			require.Equal(t, wantType, gotType)
			require.Equal(t, wantData, gotData)
			require.Equal(t, sample.value[:], gotData)
		})
	}

	t.Run("the current value is written, not the baseline", func(t *testing.T) {
		value := NewObjectID(testObjectIDBase)
		value.Set(testObjectID)

		_, data, err := value.MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, testObjectID[:], data)
	})
}

// TestObjectID_UnmarshalBSONValue verifies decoding onto an ObjectID that already holds an unsaved change
func TestObjectID_UnmarshalBSONValue(t *testing.T) {

	check := func(name string, source any, want primitive.ObjectID) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			bsonType, data, err := bson.MarshalValue(source)
			require.NoError(t, err)

			value := changedObjectID()
			require.NoError(t, value.UnmarshalBSONValue(bsonType, data))
			requireObjectID(t, value, want, want)
		})
	}

	checkErr := func(name string, bsonType bsontype.Type, data []byte) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedObjectID()
			err := value.UnmarshalBSONValue(bsonType, data)
			require.Error(t, err)
			require.ErrorContains(t, err, "delta.ObjectID.UnmarshalBSONValue")
			require.Equal(t, changedObjectID(), value, "a failed decode leaves the ObjectID as it was")
		})
	}

	checkErrFrom := func(name string, source any) {
		t.Helper()
		bsonType, data, err := bson.MarshalValue(source)
		require.NoError(t, err)
		checkErr(name, bsonType, data)
	}

	for _, sample := range objectIDSamples() {
		check("sample/"+sample.name, sample.value, sample.value)
		check("sample hex/"+sample.name, sample.value.Hex(), sample.value)
	}

	// The driver's ObjectID decoder also accepts these neighbouring BSON values
	check("uppercase hex string", strings.ToUpper(testObjectIDHex), testObjectID)
	check("twelve-character string is read as raw bytes", "abcdefghijkl", primitive.ObjectID([]byte("abcdefghijkl")))
	check("null becomes nil", primitive.Null{}, primitive.NilObjectID)
	check("undefined becomes nil", primitive.Undefined{}, primitive.NilObjectID)

	checkErrFrom("empty string", "")
	checkErrFrom("short hex string", testObjectIDHex[:23])
	checkErrFrom("long hex string", testObjectIDHex+"0")
	checkErrFrom("non-hex string of the right length", "zzzzzzzzzzzzzzzzzzzzzzzz")
	checkErrFrom("hex string with one bad trailing character", "aaaaaaaaaaaaaaaaaaaaaazz")
	checkErrFrom("boolean", true)
	checkErrFrom("int32", int32(7))
	checkErrFrom("int64", int64(7))
	checkErrFrom("double", 1.5)
	checkErrFrom("document", bson.M{"$oid": testObjectIDHex})
	checkErrFrom("array", bson.A{testObjectID})
	checkErrFrom("binary", primitive.Binary{Data: testObjectID[:]})
	checkErrFrom("datetime", primitive.DateTime(5))
	checkErrFrom("symbol", primitive.Symbol(testObjectIDHex))

	checkErr("nil data", bson.TypeObjectID, nil)
	checkErr("empty data", bson.TypeObjectID, []byte{})
	checkErr("eleven bytes", bson.TypeObjectID, testObjectID[:11])
	checkErr("truncated string", bson.TypeString, []byte{25, 0, 0, 0, 'a'})
	checkErr("type zero", bsontype.Type(0), nil)
	checkErr("unknown type", bsontype.Type(0x42), testObjectID[:])
}

// TestObjectID_BSONDocument verifies how an ObjectID behaves as a field of a stored record
func TestObjectID_BSONDocument(t *testing.T) {

	type wrapped struct {
		ID ObjectID `bson:"_id"`
	}
	type plain struct {
		ID primitive.ObjectID `bson:"_id"`
	}

	t.Run("wire format matches a primitive.ObjectID field", func(t *testing.T) {
		for _, sample := range objectIDSamples() {
			want, err := bson.Marshal(plain{ID: sample.value})
			require.NoError(t, err)

			got, err := bson.Marshal(wrapped{ID: NewObjectID(sample.value)})
			require.NoError(t, err)
			require.Equal(t, want, got, sample.name)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		for _, sample := range objectIDSamples() {
			source := wrapped{ID: NewObjectID(testObjectIDBase)}
			source.ID.Set(sample.value)

			document, err := bson.Marshal(source)
			require.NoError(t, err)

			result := wrapped{ID: changedObjectID()}
			require.NoError(t, bson.Unmarshal(document, &result))
			requireObjectID(t, result.ID, sample.value, sample.value)

			plainResult := plain{}
			require.NoError(t, bson.Unmarshal(document, &plainResult))
			require.Equal(t, sample.value, plainResult.ID)
		}
	})

	t.Run("omitempty follows the current value", func(t *testing.T) {
		record := struct {
			Empty   ObjectID `bson:"empty,omitempty"`
			Cleared ObjectID `bson:"cleared,omitempty"`
			Filled  ObjectID `bson:"filled,omitempty"`
		}{Cleared: NewObjectID(testObjectID)}

		record.Cleared.Clear()
		record.Filled.Set(testObjectID)

		document, err := bson.Marshal(record)
		require.NoError(t, err)

		result := bson.M{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, bson.M{"filled": testObjectID}, result)
	})

	t.Run("a missing field is left alone", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"other": testObjectID})
		require.NoError(t, err)

		result := wrapped{ID: changedObjectID()}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, changedObjectID(), result.ID)
	})

	t.Run("a null field becomes nil", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"_id": nil})
		require.NoError(t, err)

		result := wrapped{ID: changedObjectID()}
		require.NoError(t, bson.Unmarshal(document, &result))
		requireObjectID(t, result.ID, primitive.NilObjectID, primitive.NilObjectID)
	})

	t.Run("a wrong type fails and leaves the field alone", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"_id": 12})
		require.NoError(t, err)

		result := wrapped{ID: changedObjectID()}
		require.Error(t, bson.Unmarshal(document, &result))
		require.Equal(t, changedObjectID(), result.ID)
	})

	t.Run("pointer field", func(t *testing.T) {
		type pointered struct {
			ID *ObjectID `bson:"_id"`
		}

		value := NewObjectID(testObjectID)
		document, err := bson.Marshal(pointered{ID: &value})
		require.NoError(t, err)

		result := pointered{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.NotNil(t, result.ID)
		requireObjectID(t, *result.ID, testObjectID, testObjectID)
	})

	t.Run("slice", func(t *testing.T) {
		type collection struct {
			List []ObjectID `bson:"list"`
		}

		source := collection{List: []ObjectID{NewObjectID(testObjectID), NewObjectID(primitive.NilObjectID)}}
		document, err := bson.Marshal(source)
		require.NoError(t, err)

		raw := bson.M{}
		require.NoError(t, bson.Unmarshal(document, &raw))
		require.Equal(t, bson.M{"list": bson.A{testObjectID, primitive.NilObjectID}}, raw)

		result := collection{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, source, result)
	})

	t.Run("save and reload cycle", func(t *testing.T) {

		// Load a record, edit it, save it, and Reset, as a service does
		document, err := bson.Marshal(plain{ID: testObjectIDBase})
		require.NoError(t, err)

		record := wrapped{}
		require.NoError(t, bson.Unmarshal(document, &record))

		record.ID.Set(testObjectID)
		requireObjectID(t, record.ID, testObjectID, testObjectIDBase)

		saved, err := bson.Marshal(record)
		require.NoError(t, err)

		record.ID.Reset()
		requireObjectID(t, record.ID, testObjectID, testObjectID)

		reloaded := wrapped{}
		require.NoError(t, bson.Unmarshal(saved, &reloaded))
		require.Equal(t, record, reloaded)
	})
}

/******************************************
 * Fuzz Tests
 ******************************************/

// FuzzObjectID_Operations runs random sequences of mutations against a two-variable model
func FuzzObjectID_Operations(f *testing.F) {

	f.Add(testObjectIDBase[:], testObjectID[:], testObjectIDCurr[:], []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	f.Add([]byte{}, []byte{}, []byte{}, []byte{2, 3, 0, 3})
	f.Add([]byte("abcdefghijkl"), []byte{0xff}, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, []byte{0, 8, 1, 9, 7, 3, 2, 10, 11})
	f.Add(testObjectID[:], testObjectID[:], testObjectID[:], []byte{0, 1, 4, 5, 6})

	f.Fuzz(func(t *testing.T, initialBytes []byte, firstBytes []byte, secondBytes []byte, operations []byte) {

		initial := objectIDFromBytes(initialBytes)
		first := objectIDFromBytes(firstBytes)
		second := objectIDFromBytes(secondBytes)

		value := NewObjectID(initial)
		current, original := initial, initial
		requireObjectID(t, value, current, original)

		for _, operation := range operations {

			switch operation % 12 {

			case 0:
				value.Set(first)
				current = first

			case 1:
				value.Set(second)
				current = second

			case 2:
				value.Clear()
				current = primitive.NilObjectID

			case 3:
				value.Reset()
				original = current

			case 4:
				*value.Pointer() = first
				current = first

			case 5:
				require.NoError(t, value.SetValue(second))
				current = second

			case 6:
				require.NoError(t, value.SetValue(first.Hex()))
				current = first

			case 7:
				value.Set(value.Original())
				current = original

			case 8:
				encoded, err := json.Marshal(value)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(encoded, &value))
				original = current

			case 9:
				bsonType, data, err := value.MarshalBSONValue()
				require.NoError(t, err)
				require.NoError(t, value.UnmarshalBSONValue(bsonType, data))
				original = current

			case 10:
				require.Error(t, value.SetValue(first.Hex()[:23]+"z"))

			case 11:
				require.Error(t, value.UnmarshalJSON([]byte(`"`+second.Hex()[:23]+`z"`)))
			}

			requireObjectID(t, value, current, original)
		}
	})
}

// FuzzObjectID_SetValue verifies that SetValue accepts exactly the strings the driver parses as hex
func FuzzObjectID_SetValue(f *testing.F) {

	f.Add(testObjectIDHex, testObjectID[:])
	f.Add(strings.ToUpper(testObjectIDHex), []byte{})
	f.Add("", []byte{0xff})
	f.Add("zzzzzzzzzzzzzzzzzzzzzzzz", []byte("abcdefghijkl"))
	f.Add(testObjectIDHex[:23]+"z", []byte{})
	f.Add(" "+testObjectIDHex, []byte{})
	f.Add(`ObjectID("`+testObjectIDHex+`")`, []byte{})
	f.Add("abcdefghijkl", []byte{})

	f.Fuzz(func(t *testing.T, text string, data []byte) {

		// Text is accepted when it is empty, or when the driver accepts it as hex
		want, wantErr := primitive.ObjectIDFromHex(text)
		if text == "" {
			want, wantErr = primitive.NilObjectID, nil
		}

		for _, input := range []any{text, []byte(text), NewString(text)} {
			value := changedObjectID()
			err := value.SetValue(input)

			if wantErr != nil {
				require.Error(t, err)
				require.Equal(t, changedObjectID(), value)
				continue
			}

			require.NoError(t, err)
			requireObjectID(t, value, want, testObjectIDBase)
		}

		// Every form of a real ObjectID is accepted and read back exactly
		id := objectIDFromBytes(data)

		for _, input := range []any{id, &id, id.Hex(), strings.ToUpper(id.Hex()), NewObjectID(id), []byte(id.Hex())} {
			value := changedObjectID()
			require.NoError(t, value.SetValue(input))
			requireObjectID(t, value, id, testObjectIDBase)
		}
	})
}

// FuzzObjectID_RoundTrip verifies that any ObjectID keeps the driver's wire format in both encodings
func FuzzObjectID_RoundTrip(f *testing.F) {

	for _, sample := range objectIDSamples() {
		f.Add(sample.value[:], testObjectIDBase[:])
	}
	f.Add([]byte{}, []byte{})

	f.Fuzz(func(t *testing.T, data []byte, baselineData []byte) {

		type wrapped struct {
			Value ObjectID `json:"value" bson:"value"`
		}
		type plain struct {
			Value primitive.ObjectID `json:"value" bson:"value"`
		}

		id := objectIDFromBytes(data)
		baseline := objectIDFromBytes(baselineData)

		source := wrapped{Value: NewObjectID(baseline)}
		source.Value.Set(id)

		// JSON writes the same bytes as the driver's type and reads back the same value
		wantJSON, err := json.Marshal(plain{Value: id})
		require.NoError(t, err)

		gotJSON, err := json.Marshal(source)
		require.NoError(t, err)
		require.Equal(t, string(wantJSON), string(gotJSON))

		fromJSON := wrapped{Value: changedObjectID()}
		require.NoError(t, json.Unmarshal(gotJSON, &fromJSON))
		requireObjectID(t, fromJSON.Value, id, id)

		// Uppercase hex reads back as the same value
		fromUpper := wrapped{Value: changedObjectID()}
		require.NoError(t, json.Unmarshal([]byte(`{"value":"`+strings.ToUpper(id.Hex())+`"}`), &fromUpper))
		requireObjectID(t, fromUpper.Value, id, id)

		// RULE: Extended JSON is not a hex string, so it is rejected
		fromExtended := wrapped{Value: changedObjectID()}
		require.Error(t, json.Unmarshal([]byte(`{"value":{"$oid":"`+id.Hex()+`"}}`), &fromExtended))
		require.Equal(t, changedObjectID(), fromExtended.Value)

		// BSON writes the same bytes as the driver's type and reads back the same value
		wantBSON, err := bson.Marshal(plain{Value: id})
		require.NoError(t, err)

		gotBSON, err := bson.Marshal(source)
		require.NoError(t, err)
		require.Equal(t, wantBSON, gotBSON)

		fromBSON := wrapped{Value: changedObjectID()}
		require.NoError(t, bson.Unmarshal(gotBSON, &fromBSON))
		requireObjectID(t, fromBSON.Value, id, id)

		// A hex string stored in place of an ObjectID loads as the same value
		hexDocument, err := bson.Marshal(bson.M{"value": id.Hex()})
		require.NoError(t, err)

		fromHex := wrapped{Value: changedObjectID()}
		require.NoError(t, bson.Unmarshal(hexDocument, &fromHex))
		requireObjectID(t, fromHex.Value, id, id)

		// Marshalling never disturbs the source
		requireObjectID(t, source.Value, id, baseline)
	})
}

// FuzzObjectID_UnmarshalJSON verifies that only null, an empty string, or 24 hex characters decode
func FuzzObjectID_UnmarshalJSON(f *testing.F) {

	seeds := []string{
		`"` + testObjectIDHex + `"`, `{"$oid":"` + testObjectIDHex + `"}`, `""`, `null`, `12`, `true`, `{}`, `[]`, ``,
		`"abcdefghij"`, `123456789012`, `"aaaaaaaaaaaaaaaaaaaaaazz"`, `"zaaaaaaaaaaaaaaaaaaaaaaa"`,
		`{"$oid":"aaaaaaaaaaaaaaaaaaaaaazz"}`, `{"$oid":12}`, `{"$oid":""}`, `"` + testObjectIDHex + `"x`,
		` "` + testObjectIDHex + `" `, `"` + testObjectIDHex[:23] + `"`, `"abcdefghijkl"`, `" "`,
		`"` + strings.ToUpper(testObjectIDHex) + `"`, `"000000000000000000000000"`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		// The oracle reads the input as an optional string, then as hex
		text := ""
		want := primitive.NilObjectID
		isValid := json.Unmarshal(data, &text) == nil

		if isValid && (text != "") {
			parsed, err := primitive.ObjectIDFromHex(text)
			want, isValid = parsed, err == nil
		}

		value := changedObjectID()
		err := value.UnmarshalJSON(data)

		viaPackage := changedObjectID()
		viaPackageErr := json.Unmarshal(data, &viaPackage)

		if !isValid {
			require.Error(t, err)
			require.Error(t, viaPackageErr)
			require.Equal(t, changedObjectID(), value)
			require.Equal(t, changedObjectID(), viaPackage)
			return
		}

		require.NoError(t, err)
		require.NoError(t, viaPackageErr)
		requireObjectID(t, value, want, want)
		requireObjectID(t, viaPackage, want, want)

		// Whatever decodes is written back out as lowercase hex
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, `"`+want.Hex()+`"`, string(encoded))
	})
}

// FuzzObjectID_UnmarshalJSONDocument verifies that an ObjectID field decodes only hex strings and null
func FuzzObjectID_UnmarshalJSONDocument(f *testing.F) {

	seeds := []string{
		`{"value":"` + testObjectIDHex + `"}`, `{"value":{"$oid":"` + testObjectIDHex + `"}}`, `{"value":null}`,
		`{"value":""}`, `{"value":12}`, `{}`, `{"value":"aaaaaaaaaaaaaaaaaaaaaazz"}`, `{"value":"abcdefghij"}`,
		`{"value":"` + testObjectIDHex + `","value":null}`, `{"VALUE":"` + testObjectIDHex + `"}`, `[]`, `null`,
		`{"value":"` + testObjectIDHex + `","value":"zz"}`, `{"value":123456789012}`,
		`{"value":"zz","value":"` + testObjectIDHex + `"}`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		// A string pointer is the oracle for the JSON itself, and the driver for the hex
		plain := struct {
			Value *string `json:"value"`
		}{}

		wrapped := struct {
			Value ObjectID `json:"value"`
		}{}
		wrappedErr := json.Unmarshal(data, &wrapped)

		// Baseline and current value always move together during a load, even a failed one
		require.True(t, wrapped.Value.NotChanged())

		if err := json.Unmarshal(data, &plain); err != nil {
			require.Error(t, wrappedErr)
			return
		}

		// A repeated key can fail on an earlier value that the oracle has already overwritten
		if wrappedErr != nil {
			return
		}

		want := primitive.NilObjectID
		if (plain.Value != nil) && (*plain.Value != "") {
			parsed, err := primitive.ObjectIDFromHex(*plain.Value)
			require.NoError(t, err)
			want = parsed
		}

		requireObjectID(t, wrapped.Value, want, want)
	})
}

// FuzzObjectID_UnmarshalBSONValue verifies that arbitrary BSON decodes exactly as it does for the driver's type
func FuzzObjectID_UnmarshalBSONValue(f *testing.F) {

	sources := []any{
		testObjectID, primitive.NilObjectID, testObjectIDHex, "abcdefghijkl", "", "zzzzzzzzzzzzzzzzzzzzzzzz",
		"aaaaaaaaaaaaaaaaaaaaaazz", primitive.Null{}, primitive.Undefined{}, true, int32(7), int64(7), 1.5,
		bson.M{"$oid": testObjectIDHex}, bson.A{testObjectID}, primitive.Binary{Data: testObjectID[:]},
		primitive.Symbol(testObjectIDHex), primitive.DateTime(5), primitive.DBPointer{DB: "a", Pointer: testObjectID},
	}
	for _, source := range sources {
		bsonType, data, err := bson.MarshalValue(source)
		require.NoError(f, err)
		f.Add(byte(bsonType), data)
	}

	f.Add(byte(bson.TypeObjectID), testObjectID[:11])
	f.Add(byte(bson.TypeObjectID), append(testObjectID[:], 0))
	f.Add(byte(bson.TypeString), []byte{25, 0, 0, 0, 'a'})
	f.Add(byte(0), []byte{})

	f.Fuzz(func(t *testing.T, bsonType byte, data []byte) {

		// The driver's type, starting at the baseline, is the oracle
		plain := testObjectIDBase
		value := changedObjectID()

		if err := bson.UnmarshalValue(bsontype.Type(bsonType), data, &plain); err != nil {
			require.Error(t, value.UnmarshalBSONValue(bsontype.Type(bsonType), data))
			require.Equal(t, changedObjectID(), value)
			return
		}

		require.NoError(t, value.UnmarshalBSONValue(bsontype.Type(bsonType), data))
		requireObjectID(t, value, plain, plain)
	})
}

// FuzzObjectID_UnmarshalBSONDocument verifies that an ObjectID field decodes as the driver's type does
func FuzzObjectID_UnmarshalBSONDocument(f *testing.F) {

	sources := []bson.M{
		{"value": testObjectID}, {"value": testObjectIDHex}, {"value": nil}, {"value": 12}, {}, {"other": testObjectID},
		{"value": "abcdefghijkl"}, {"value": ""}, {"value": "aaaaaaaaaaaaaaaaaaaaaazz"}, {"value": bson.M{"a": 1}},
		{"value": bson.A{testObjectID}}, {"value": primitive.Undefined{}}, {"value": primitive.Binary{Data: testObjectID[:]}},
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
			Value primitive.ObjectID `bson:"value"`
		}{}
		plainErr := bson.Unmarshal(data, &plain)

		wrapped := struct {
			Value ObjectID `bson:"value"`
		}{}
		wrappedErr := bson.Unmarshal(data, &wrapped)

		require.Equal(t, plainErr == nil, wrappedErr == nil, "plain: %v, wrapped: %v", plainErr, wrappedErr)

		// Baseline and current value always move together during a load, even a failed one
		require.True(t, wrapped.Value.NotChanged())

		if plainErr == nil {
			requireObjectID(t, wrapped.Value, plain.Value, plain.Value)
		}
	})
}

/******************************************
 * Examples
 ******************************************/

// ExampleObjectID shows a change being tracked against the value that was loaded
func ExampleObjectID() {

	stored, _ := primitive.ObjectIDFromHex("6b04f64736058a543006f9a6")
	next, _ := primitive.ObjectIDFromHex("6b04f64736058a543006f9a7")

	parentID := NewObjectID(stored)
	fmt.Println(parentID.IsChanged())

	parentID.Set(next)
	fmt.Println(parentID.IsChanged())
	fmt.Println(parentID.Hex())
	fmt.Println(parentID.Original().Hex())

	// Output:
	// false
	// true
	// 6b04f64736058a543006f9a7
	// 6b04f64736058a543006f9a6
}

// ExampleObjectID_Reset shows the baseline moving once the caller has saved the value
func ExampleObjectID_Reset() {

	stored, _ := primitive.ObjectIDFromHex("6b04f64736058a543006f9a6")
	next, _ := primitive.ObjectIDFromHex("6b04f64736058a543006f9a7")

	parentID := NewObjectID(stored)
	parentID.Set(next)

	parentID.Reset()
	fmt.Println(parentID.IsChanged())
	fmt.Println(parentID.Original().Hex())

	// Output:
	// false
	// 6b04f64736058a543006f9a7
}
