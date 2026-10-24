package delta

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
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

// TestBool_Accessors covers every reader, and writing through the pointer
func TestBool_Accessors(t *testing.T) {

	value := NewBool(false)
	require.False(t, value.Bool())
	require.False(t, value.IsTrue())
	require.True(t, value.IsFalse())
	require.True(t, value.IsZero())
	require.Equal(t, false, value.GetValue())

	*value.Pointer() = true

	require.True(t, value.Value())
	require.True(t, value.Bool())
	require.True(t, value.IsTrue())
	require.False(t, value.IsFalse())
	require.False(t, value.IsZero())
	require.Equal(t, true, value.GetValue())
	require.True(t, value.IsChanged(), "a write through the pointer is a change")
	require.False(t, value.Original())
}

// TestBool_SetChangedBack verifies that returning to the original value reads as unchanged
func TestBool_SetChangedBack(t *testing.T) {

	value := NewBool(true)
	value.Set(false)
	value.Set(true)

	require.True(t, value.NotChanged())
	require.False(t, value.IsChanged())
}

// TestBool_SetValue pins how each kind of input converts, through rosetta's convert.Bool
func TestBool_SetValue(t *testing.T) {

	check := func(name string, input any, want bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewBool(!want)
			require.NoError(t, value.SetValue(input))
			require.Equal(t, want, value.Value(), "SetValue(%#v)", input)
			require.Equal(t, !want, value.Original(), "SetValue never moves the baseline")
		})
	}

	check("true", true, true)
	check("false", false, false)
	check("string true", "true", true)
	check("string false", "false", false)
	check("int one", 1, true)
	check("int zero", 0, false)
	check("float one", float64(1), true)
	check("float zero", float64(0), false)
	check("unrecognized string", "yes", false)
	check("empty string", "", false)
	check("nil", nil, false)
	check("slice of one true", []any{true}, true)
	check("a Bool", NewBool(true), true)
}

// TestBool_JSON covers marshalling, unmarshalling, and the baseline that unmarshal sets
func TestBool_JSON(t *testing.T) {

	t.Run("marshal writes the current value", func(t *testing.T) {
		value := NewBool(false)
		value.Set(true)

		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, `true`, string(encoded))
	})

	t.Run("unmarshal resets the baseline", func(t *testing.T) {
		value := NewBool(false)
		value.Set(true)

		require.NoError(t, json.Unmarshal([]byte(`false`), &value))
		require.False(t, value.Value())
		require.False(t, value.Original())
		require.False(t, value.IsChanged())
	})

	t.Run("round trip inside a struct", func(t *testing.T) {
		type record struct {
			IsActive Bool `json:"isActive"`
		}

		encoded, err := json.Marshal(record{IsActive: NewBool(true)})
		require.NoError(t, err)
		require.Equal(t, `{"isActive":true}`, string(encoded))

		result := record{}
		require.NoError(t, json.Unmarshal(encoded, &result))
		require.True(t, result.IsActive.Value())
		require.True(t, result.IsActive.NotChanged())
	})

	// A JSON null leaves the baseline where it was and copies it into the current value,
	// so a pending edit is discarded rather than reported as an error
	t.Run("null discards a pending edit", func(t *testing.T) {
		value := NewBool(true)
		value.Set(false)

		require.NoError(t, value.UnmarshalJSON([]byte(`null`)))
		require.True(t, value.Value())
		require.True(t, value.Original())
	})

	checkErr := func(name string, input string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewBool(true)
			value.Set(false)

			require.Error(t, value.UnmarshalJSON([]byte(input)))
			require.False(t, value.Value(), "a failed unmarshal leaves the current value alone")
			require.True(t, value.Original(), "a failed unmarshal leaves the baseline alone")
		})
	}

	checkErr("a string", `"true"`)
	checkErr("a number", `1`)
	checkErr("an object", `{}`)
	checkErr("malformed", `tru`)
	checkErr("empty", ``)
}

// TestBool_BSON covers marshalling, unmarshalling, and the baseline that unmarshal sets
func TestBool_BSON(t *testing.T) {

	t.Run("marshal writes a BSON boolean", func(t *testing.T) {
		value := NewBool(false)
		value.Set(true)

		kind, data, err := value.MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, bson.TypeBoolean, kind)
		require.Equal(t, []byte{0x01}, data)
	})

	t.Run("round trip inside a document", func(t *testing.T) {
		type record struct {
			IsActive Bool `bson:"isActive"`
		}

		document, err := bson.Marshal(record{IsActive: NewBool(true)})
		require.NoError(t, err)

		raw := bson.Raw(document)
		require.True(t, raw.Lookup("isActive").Boolean(), "stored as a plain boolean, not a sub-document")

		result := record{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.True(t, result.IsActive.Value())
		require.True(t, result.IsActive.NotChanged())
	})

	t.Run("unmarshal of the wrong type fails", func(t *testing.T) {
		kind, data, err := bson.MarshalValue("true")
		require.NoError(t, err)

		value := NewBool(true)
		value.Set(false)

		require.Error(t, value.UnmarshalBSONValue(kind, data))
		require.False(t, value.Value(), "a failed unmarshal leaves the current value alone")
		require.True(t, value.Original(), "a failed unmarshal leaves the baseline alone")
	})

	t.Run("unmarshal of truncated data fails", func(t *testing.T) {
		value := NewBool(false)
		require.Error(t, value.UnmarshalBSONValue(bson.TypeBoolean, []byte{}))
	})
}

// FuzzBool_UnmarshalJSON checks that any input either fails or yields an unchanged value
func FuzzBool_UnmarshalJSON(f *testing.F) {

	for _, seed := range []string{`true`, `false`, `null`, `"true"`, `1`, `{}`, `[]`, `tru`, ``, ` true `} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		value := NewBool(true)
		value.Set(false)

		// RULE: A failed unmarshal changes nothing
		if err := value.UnmarshalJSON(data); err != nil {
			require.False(t, value.Value())
			require.True(t, value.Original())
			return
		}

		// A successful unmarshal leaves a value that reads as freshly loaded
		require.True(t, value.NotChanged(), "input %q", data)

		// ...and marshals back to an equal value
		encoded, err := value.MarshalJSON()
		require.NoError(t, err)

		roundTrip := NewBool(!value.Value())
		require.NoError(t, roundTrip.UnmarshalJSON(encoded))
		require.Equal(t, value.Value(), roundTrip.Value())
	})
}

// FuzzBool_UnmarshalBSONValue checks that no BSON type and payload can panic the decoder
func FuzzBool_UnmarshalBSONValue(f *testing.F) {

	f.Add(byte(bson.TypeBoolean), []byte{0x01})
	f.Add(byte(bson.TypeBoolean), []byte{0x00})
	f.Add(byte(bson.TypeBoolean), []byte{})
	f.Add(byte(bson.TypeNull), []byte{})
	f.Add(byte(bson.TypeString), []byte{0x05, 0x00, 0x00, 0x00, 't', 'r', 'u', 'e', 0x00})
	f.Add(byte(bson.TypeInt32), []byte{0x01, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, kind byte, data []byte) {

		value := NewBool(true)
		value.Set(false)

		// RULE: A failed unmarshal changes nothing
		if err := value.UnmarshalBSONValue(bsontype.Type(kind), data); err != nil {
			require.False(t, value.Value())
			require.True(t, value.Original())
			return
		}

		// A successful unmarshal leaves a value that reads as freshly loaded
		require.True(t, value.NotChanged())
	})
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

// Bool must also satisfy rosetta's schema.ValueGetter and schema.ValueSetter, which
// its doc comments name. The shapes are copied here so the tests add no dependency.
var (
	_ interface{ GetValue() any }      = Bool{}
	_ interface{ SetValue(any) error } = &Bool{}
)
