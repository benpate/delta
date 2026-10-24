package delta

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// testObjectID returns a fixed ObjectID, so that failures print the same value every run
func testObjectID(t *testing.T, value string) primitive.ObjectID {
	t.Helper()

	result, err := primitive.ObjectIDFromHex(value)
	require.NoError(t, err)
	return result
}

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

// TestObjectID_Accessors covers every reader, and writing through the pointer
func TestObjectID_Accessors(t *testing.T) {

	first := testObjectID(t, "5f1e2d3c4b5a69788796a5b4")
	second := testObjectID(t, "0123456789abcdef01234567")

	value := NewObjectID(first)
	require.Equal(t, "5f1e2d3c4b5a69788796a5b4", value.Hex())
	require.Equal(t, "5f1e2d3c4b5a69788796a5b4", value.String())
	require.Equal(t, first, value.GetValue())
	require.False(t, value.IsZero())

	*value.Pointer() = second

	require.Equal(t, second, value.Value())
	require.Equal(t, "0123456789abcdef01234567", value.Hex())
	require.True(t, value.IsChanged(), "a write through the pointer is a change")
	require.Equal(t, first, value.Original())

	value.Set(primitive.NilObjectID)
	require.True(t, value.IsZero())
	require.Equal(t, "000000000000000000000000", value.Hex())
}

// TestObjectID_SetChangedBack verifies that returning to the original value reads as unchanged
func TestObjectID_SetChangedBack(t *testing.T) {

	first := testObjectID(t, "5f1e2d3c4b5a69788796a5b4")

	value := NewObjectID(first)
	value.Set(primitive.NilObjectID)
	value.Set(first)

	require.True(t, value.NotChanged())
	require.False(t, value.IsChanged())
}

// TestObjectID_SetValue pins which inputs convert into an ObjectID, and which fail
func TestObjectID_SetValue(t *testing.T) {

	original := testObjectID(t, "5f1e2d3c4b5a69788796a5b4")
	want := testObjectID(t, "0123456789abcdef01234567")

	check := func(name string, input any) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewObjectID(original)
			require.NoError(t, value.SetValue(input))
			require.Equal(t, want, value.Value(), "SetValue(%#v)", input)
			require.Equal(t, original, value.Original(), "SetValue never moves the baseline")
		})
	}

	checkErr := func(name string, input any) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewObjectID(original)
			require.Error(t, value.SetValue(input), "SetValue(%#v)", input)
			require.Equal(t, original, value.Value(), "a failed SetValue leaves the value alone")
		})
	}

	check("an ObjectID", want)
	check("a hex string", "0123456789abcdef01234567")
	check("an upper-case hex string", "0123456789ABCDEF01234567")
	check("an ObjectID wrapper", NewObjectID(want))

	checkErr("an empty string", "")
	checkErr("too short", "0123456789abcdef")
	checkErr("too long", "0123456789abcdef0123456789")
	checkErr("not hex", "0123456789abcdefghijklmn")
	checkErr("nil", nil)
	checkErr("an int", 42)
	checkErr("a bool", true)
}

// TestObjectID_JSON covers marshalling, unmarshalling, and the baseline that unmarshal sets
func TestObjectID_JSON(t *testing.T) {

	original := testObjectID(t, "5f1e2d3c4b5a69788796a5b4")
	edited := testObjectID(t, "0123456789abcdef01234567")

	t.Run("marshal writes the current value as hex", func(t *testing.T) {
		value := NewObjectID(original)
		value.Set(edited)

		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, `"0123456789abcdef01234567"`, string(encoded))
	})

	check := func(name string, input string, want primitive.ObjectID) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewObjectID(original)
			value.Set(edited)

			require.NoError(t, value.UnmarshalJSON([]byte(input)))
			require.Equal(t, want, value.Value())
			require.Equal(t, want, value.Original(), "unmarshal resets the baseline")
		})
	}

	check("a hex string", `"0123456789abcdef01234567"`, edited)
	check("extended JSON", `{"$oid":"0123456789abcdef01234567"}`, edited)
	check("an empty string is the nil ObjectID", `""`, primitive.NilObjectID)

	// A JSON null leaves the baseline where it was and copies it into the current value,
	// so a pending edit is discarded rather than reported as an error
	check("null discards a pending edit", `null`, original)

	// mongo-driver copies ANY 12-byte input straight into the ObjectID, before reading it
	// as JSON, so a 10-character JSON string decodes to its own raw bytes, quotes included
	check("twelve raw bytes", `"abcdefghij"`, primitive.ObjectID([]byte(`"abcdefghij"`)))

	checkErr := func(name string, input string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewObjectID(original)
			value.Set(edited)

			require.Error(t, value.UnmarshalJSON([]byte(input)))
			require.Equal(t, edited, value.Value(), "a failed unmarshal leaves the current value alone")
			require.Equal(t, original, value.Original(), "a failed unmarshal leaves the baseline alone")
		})
	}

	checkErr("too short", `"0123456789abcdef"`)
	checkErr("not hex", `"0123456789abcdefghijklmn"`)
	checkErr("a number", `42`)
	checkErr("an object without $oid", `{"id":"0123456789abcdef01234567"}`)
	checkErr("a non-string $oid", `{"$oid":42}`)
	checkErr("malformed", `"0123`)
	checkErr("empty", ``)
}

// TestObjectID_BSON covers marshalling, unmarshalling, and the baseline that unmarshal sets
func TestObjectID_BSON(t *testing.T) {

	original := testObjectID(t, "5f1e2d3c4b5a69788796a5b4")
	edited := testObjectID(t, "0123456789abcdef01234567")

	t.Run("marshal writes a BSON ObjectID", func(t *testing.T) {
		value := NewObjectID(original)
		value.Set(edited)

		kind, data, err := value.MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, bson.TypeObjectID, kind)
		require.Equal(t, edited[:], data)
	})

	t.Run("round trip inside a document", func(t *testing.T) {
		type record struct {
			ParentID ObjectID `bson:"parentId"`
		}

		document, err := bson.Marshal(record{ParentID: NewObjectID(original)})
		require.NoError(t, err)
		require.Equal(t, original, bson.Raw(document).Lookup("parentId").ObjectID(), "stored as a plain ObjectID, not a sub-document")

		result := record{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, original, result.ParentID.Value())
		require.True(t, result.ParentID.NotChanged())
	})

	t.Run("unmarshal of the wrong type fails", func(t *testing.T) {
		kind, data, err := bson.MarshalValue(int32(42))
		require.NoError(t, err)

		value := NewObjectID(original)
		value.Set(edited)

		require.Error(t, value.UnmarshalBSONValue(kind, data))
		require.Equal(t, edited, value.Value(), "a failed unmarshal leaves the current value alone")
		require.Equal(t, original, value.Original(), "a failed unmarshal leaves the baseline alone")
	})

	t.Run("unmarshal of truncated data fails", func(t *testing.T) {
		value := NewObjectID(original)
		require.Error(t, value.UnmarshalBSONValue(bson.TypeObjectID, edited[:6]))
	})
}

// FuzzObjectID_SetValue checks that a string either fails or names the ObjectID it spells
func FuzzObjectID_SetValue(f *testing.F) {

	for _, seed := range []string{"0123456789abcdef01234567", "0123456789ABCDEF01234567", "", "0123", "zzzzzzzzzzzzzzzzzzzzzzzz", "0123456789abcdef0123456789"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {

		original := primitive.NewObjectID()
		value := NewObjectID(original)

		// RULE: A failed SetValue changes nothing
		if err := value.SetValue(input); err != nil {
			require.Equal(t, original, value.Value())
			return
		}

		// A successful SetValue holds exactly the ObjectID the hex string spells
		require.Equal(t, strings.ToLower(input), value.Hex())
		require.Equal(t, original, value.Original())
	})
}

// FuzzObjectID_RoundTrip checks that any ObjectID survives JSON and BSON unchanged
func FuzzObjectID_RoundTrip(f *testing.F) {

	f.Add([]byte{0x5f, 0x1e, 0x2d, 0x3c, 0x4b, 0x5a, 0x69, 0x78, 0x87, 0x96, 0xa5, 0xb4})
	f.Add(make([]byte, 12))
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

	f.Fuzz(func(t *testing.T, input []byte) {

		// An ObjectID holds exactly twelve bytes
		if len(input) != 12 {
			t.Skip()
		}

		objectID := primitive.ObjectID(input)

		// Round-trip through JSON
		encoded, err := NewObjectID(objectID).MarshalJSON()
		require.NoError(t, err)

		fromJSON := NewObjectID(primitive.NilObjectID)
		require.NoError(t, fromJSON.UnmarshalJSON(encoded))
		require.Equal(t, objectID, fromJSON.Value(), "JSON %s", encoded)
		require.True(t, fromJSON.NotChanged())

		// Round-trip through BSON
		kind, data, err := NewObjectID(objectID).MarshalBSONValue()
		require.NoError(t, err)

		fromBSON := NewObjectID(primitive.NilObjectID)
		require.NoError(t, fromBSON.UnmarshalBSONValue(kind, data))
		require.Equal(t, objectID, fromBSON.Value(), "BSON %s", hex.EncodeToString(data))
		require.True(t, fromBSON.NotChanged())
	})
}

// FuzzObjectID_UnmarshalJSON checks that any input either fails or yields an unchanged value
func FuzzObjectID_UnmarshalJSON(f *testing.F) {

	for _, seed := range []string{`"0123456789abcdef01234567"`, `{"$oid":"0123456789abcdef01234567"}`, `""`, `null`, `"abcdefghij"`, `42`, `{}`, `"0123`, ``} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		original := testObjectID(t, "5f1e2d3c4b5a69788796a5b4")
		edited := testObjectID(t, "0123456789abcdef01234567")

		value := NewObjectID(original)
		value.Set(edited)

		// RULE: A failed unmarshal changes nothing
		if err := value.UnmarshalJSON(data); err != nil {
			require.Equal(t, edited, value.Value())
			require.Equal(t, original, value.Original())
			return
		}

		// A successful unmarshal leaves a value that reads as freshly loaded
		require.True(t, value.NotChanged(), "input %q", data)
	})
}

// FuzzObjectID_UnmarshalBSONValue checks that no BSON type and payload can panic the decoder
func FuzzObjectID_UnmarshalBSONValue(f *testing.F) {

	f.Add(byte(bson.TypeObjectID), []byte{0x5f, 0x1e, 0x2d, 0x3c, 0x4b, 0x5a, 0x69, 0x78, 0x87, 0x96, 0xa5, 0xb4})
	f.Add(byte(bson.TypeObjectID), []byte{0x5f, 0x1e})
	f.Add(byte(bson.TypeNull), []byte{})
	f.Add(byte(bson.TypeString), []byte{0x19, 0x00, 0x00, 0x00, '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', '0', '1', '2', '3', '4', '5', '6', '7', 0x00})
	f.Add(byte(bson.TypeInt32), []byte{0x2a, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, kind byte, data []byte) {

		original := testObjectID(t, "5f1e2d3c4b5a69788796a5b4")
		edited := testObjectID(t, "0123456789abcdef01234567")

		value := NewObjectID(original)
		value.Set(edited)

		// RULE: A failed unmarshal changes nothing
		if err := value.UnmarshalBSONValue(bsontype.Type(kind), data); err != nil {
			require.Equal(t, edited, value.Value())
			require.Equal(t, original, value.Original())
			return
		}

		// A successful unmarshal leaves a value that reads as freshly loaded
		require.True(t, value.NotChanged())
	})
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

// ObjectID must also satisfy rosetta's schema.ValueGetter and schema.ValueSetter, which
// its doc comments name. The shapes are copied here so the tests add no dependency.
var (
	_ interface{ GetValue() any }      = ObjectID{}
	_ interface{ SetValue(any) error } = &ObjectID{}
)
