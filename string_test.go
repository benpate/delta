package delta

import (
	"encoding/json"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// ExampleNewString shows a value that remembers what it was loaded as
func ExampleNewString() {

	name := NewString("Sarah Connor")
	name.Set("Sarah Reese")

	fmt.Println(name.IsChanged())
	fmt.Println(name.Value())
	fmt.Println(name.Original())

	// Output:
	// true
	// Sarah Reese
	// Sarah Connor
}

// TestString verifies that a String tracks a change to its value
func TestString(t *testing.T) {

	value := NewString("Sarah Connor")

	require.Equal(t, "Sarah Connor", value.Value())
	require.Equal(t, "Sarah Connor", value.String())
	require.True(t, value.NotChanged())
	require.False(t, value.IsChanged())

	value.Set("Sarah Reese")

	require.Equal(t, "Sarah Reese", value.Value())
	require.False(t, value.NotChanged())
	require.True(t, value.IsChanged())
}

// TestString_Original verifies that the baseline survives a change to the current value
func TestString_Original(t *testing.T) {

	value := NewString("Sarah Connor")
	require.Equal(t, "Sarah Connor", value.Original())

	value.Set("Sarah Reese")

	require.Equal(t, "Sarah Reese", value.Value())
	require.Equal(t, "Sarah Connor", value.Original(), "the baseline does not move when the value does")
}

// TestString_OriginalAfterUnmarshal is the case Original() exists for
func TestString_OriginalAfterUnmarshal(t *testing.T) {

	// A record loaded from the database and then edited has to be able to name the value the
	// database still holds -- to address the old row, or to clean up whatever it pointed at.

	t.Run("bson", func(t *testing.T) {

		document, err := bson.Marshal(bson.M{"name": "Sarah Connor"})
		require.NoError(t, err)

		result := struct {
			Name String `bson:"name"`
		}{}

		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, "Sarah Connor", result.Name.Original())

		result.Name.Set("Sarah Reese")
		require.Equal(t, "Sarah Connor", result.Name.Original())
		require.True(t, result.Name.IsChanged())
	})

	t.Run("json", func(t *testing.T) {

		result := struct {
			Name String `json:"name"`
		}{}

		require.NoError(t, json.Unmarshal([]byte(`{"name":"Sarah Connor"}`), &result))
		require.Equal(t, "Sarah Connor", result.Name.Original())

		result.Name.Set("Sarah Reese")
		require.Equal(t, "Sarah Connor", result.Name.Original())
	})
}

// TestString_Accessors covers the remaining surface of the type
func TestString_Accessors(t *testing.T) {

	value := NewString("")
	require.True(t, value.IsZero())

	require.NoError(t, value.SetValue(42))
	require.Equal(t, "42", value.Value())
	require.Equal(t, "42", value.GetValue())
	require.False(t, value.IsZero())

	*value.Pointer() = "written through the pointer"
	require.Equal(t, "written through the pointer", value.Value())

	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	require.Equal(t, `"written through the pointer"`, string(encoded))
}

// TestString_SetChangedBack verifies that returning to the original value reads as unchanged
func TestString_SetChangedBack(t *testing.T) {

	value := NewString("Sarah Connor")
	value.Set("Sarah Reese")
	value.Set("Sarah Connor")

	require.True(t, value.NotChanged())
	require.False(t, value.IsChanged())
}

// TestString_SetValue pins how each kind of input converts, through rosetta's convert.String
func TestString_SetValue(t *testing.T) {

	check := func(name string, input any, want string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewString("original")
			require.NoError(t, value.SetValue(input))
			require.Equal(t, want, value.Value(), "SetValue(%#v)", input)
			require.Equal(t, "original", value.Original(), "SetValue never moves the baseline")
		})
	}

	check("string", "Sarah Connor", "Sarah Connor")
	check("empty string", "", "")
	check("int", 42, "42")
	check("negative int64", int64(-7), "-7")
	check("bool", true, "true")
	check("nil", nil, "")
	check("a String", NewString("John Connor"), "John Connor")

	// A whole-number float has no decimals, so that a value read back as an integer
	// parses; any other float is rounded to two decimals
	check("whole float", float64(150), "150")
	check("negative whole float", float64(-3), "-3")
	check("zero float", float64(0), "0")
	check("fractional float", 33.333, "33.33")
	check("half", 0.5, "0.50")
	check("rounds up", 2.999, "3.00")
}

// TestString_JSON covers marshalling, unmarshalling, and the baseline that unmarshal sets
func TestString_JSON(t *testing.T) {

	t.Run("marshal writes the current value", func(t *testing.T) {
		value := NewString("Sarah Connor")
		value.Set(`quote " and <tag>`)

		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, `"quote \" and \u003ctag\u003e"`, string(encoded))
	})

	t.Run("unmarshal resets the baseline", func(t *testing.T) {
		value := NewString("Sarah Connor")
		value.Set("Sarah Reese")

		require.NoError(t, json.Unmarshal([]byte(`"Kyle Reese"`), &value))
		require.Equal(t, "Kyle Reese", value.Value())
		require.Equal(t, "Kyle Reese", value.Original())
		require.False(t, value.IsChanged())
	})

	// A JSON null leaves the baseline where it was and copies it into the current value,
	// so a pending edit is discarded rather than reported as an error
	t.Run("null discards a pending edit", func(t *testing.T) {
		value := NewString("Sarah Connor")
		value.Set("Sarah Reese")

		require.NoError(t, value.UnmarshalJSON([]byte(`null`)))
		require.Equal(t, "Sarah Connor", value.Value())
		require.Equal(t, "Sarah Connor", value.Original())
	})

	checkErr := func(name string, input string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewString("Sarah Connor")
			value.Set("Sarah Reese")

			require.Error(t, value.UnmarshalJSON([]byte(input)))
			require.Equal(t, "Sarah Reese", value.Value(), "a failed unmarshal leaves the current value alone")
			require.Equal(t, "Sarah Connor", value.Original(), "a failed unmarshal leaves the baseline alone")
		})
	}

	checkErr("a number", `42`)
	checkErr("a bool", `true`)
	checkErr("an array", `["a"]`)
	checkErr("unterminated", `"Sarah`)
	checkErr("empty", ``)
}

// TestString_BSON covers marshalling, unmarshalling, and the baseline that unmarshal sets
func TestString_BSON(t *testing.T) {

	t.Run("marshal writes a BSON string", func(t *testing.T) {
		value := NewString("")
		value.Set("Sarah")

		kind, data, err := value.MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, bson.TypeString, kind)
		require.Equal(t, []byte{0x06, 0x00, 0x00, 0x00, 'S', 'a', 'r', 'a', 'h', 0x00}, data)
	})

	t.Run("round trip inside a document", func(t *testing.T) {
		type record struct {
			Name String `bson:"name"`
		}

		document, err := bson.Marshal(record{Name: NewString("Sarah Connor")})
		require.NoError(t, err)
		require.Equal(t, "Sarah Connor", bson.Raw(document).Lookup("name").StringValue(), "stored as a plain string, not a sub-document")

		result := record{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, "Sarah Connor", result.Name.Value())
		require.True(t, result.Name.NotChanged())
	})

	t.Run("unmarshal of the wrong type fails", func(t *testing.T) {
		kind, data, err := bson.MarshalValue(int32(42))
		require.NoError(t, err)

		value := NewString("Sarah Connor")
		value.Set("Sarah Reese")

		require.Error(t, value.UnmarshalBSONValue(kind, data))
		require.Equal(t, "Sarah Reese", value.Value(), "a failed unmarshal leaves the current value alone")
		require.Equal(t, "Sarah Connor", value.Original(), "a failed unmarshal leaves the baseline alone")
	})

	t.Run("unmarshal of truncated data fails", func(t *testing.T) {
		value := NewString("")
		require.Error(t, value.UnmarshalBSONValue(bson.TypeString, []byte{0x06, 0x00, 0x00, 0x00, 'S'}))
	})
}

// FuzzString_RoundTrip checks that any valid UTF-8 string survives JSON and BSON unchanged
func FuzzString_RoundTrip(f *testing.F) {

	for _, seed := range []string{"", "Sarah Connor", `quote " and \ backslash`, "<script>", "日本語", "emoji 🤖", "nul \x00 byte", "line\nbreak"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {

		// JSON replaces invalid UTF-8 with U+FFFD, so only valid strings can round-trip
		if !utf8.ValidString(input) {
			t.Skip()
		}

		// Round-trip through JSON
		encoded, err := NewString(input).MarshalJSON()
		require.NoError(t, err)

		fromJSON := NewString("")
		require.NoError(t, fromJSON.UnmarshalJSON(encoded))
		require.Equal(t, input, fromJSON.Value())
		require.True(t, fromJSON.NotChanged())

		// Round-trip through BSON
		kind, data, err := NewString(input).MarshalBSONValue()
		require.NoError(t, err)

		fromBSON := NewString("")
		require.NoError(t, fromBSON.UnmarshalBSONValue(kind, data))
		require.Equal(t, input, fromBSON.Value())
		require.True(t, fromBSON.NotChanged())
	})
}

// FuzzString_UnmarshalJSON checks that any input either fails or yields an unchanged value
func FuzzString_UnmarshalJSON(f *testing.F) {

	for _, seed := range []string{`"Sarah"`, `""`, `null`, `42`, `"\u00e9"`, `"\ud800"`, `"Sarah`, ``, `["a"]`} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		value := NewString("Sarah Connor")
		value.Set("Sarah Reese")

		// RULE: A failed unmarshal changes nothing
		if err := value.UnmarshalJSON(data); err != nil {
			require.Equal(t, "Sarah Reese", value.Value())
			require.Equal(t, "Sarah Connor", value.Original())
			return
		}

		// A successful unmarshal leaves a value that reads as freshly loaded
		require.True(t, value.NotChanged(), "input %q", data)
	})
}

// FuzzString_UnmarshalBSONValue checks that no BSON type and payload can panic the decoder
func FuzzString_UnmarshalBSONValue(f *testing.F) {

	f.Add(byte(bson.TypeString), []byte{0x06, 0x00, 0x00, 0x00, 'S', 'a', 'r', 'a', 'h', 0x00})
	f.Add(byte(bson.TypeString), []byte{0x01, 0x00, 0x00, 0x00, 0x00})
	f.Add(byte(bson.TypeString), []byte{0xff, 0xff, 0xff, 0x7f, 'S'})
	f.Add(byte(bson.TypeNull), []byte{})
	f.Add(byte(bson.TypeSymbol), []byte{0x02, 0x00, 0x00, 0x00, 'S', 0x00})
	f.Add(byte(bson.TypeInt32), []byte{0x2a, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, kind byte, data []byte) {

		value := NewString("Sarah Connor")
		value.Set("Sarah Reese")

		// RULE: A failed unmarshal changes nothing
		if err := value.UnmarshalBSONValue(bsontype.Type(kind), data); err != nil {
			require.Equal(t, "Sarah Reese", value.Value())
			require.Equal(t, "Sarah Connor", value.Original())
			return
		}

		// A successful unmarshal leaves a value that reads as freshly loaded
		require.True(t, value.NotChanged())
	})
}

/******************************************
 * BSON format guard
 ******************************************/

// String must satisfy both *Value BSON interfaces. It holds only unexported fields
// and has no plain MarshalBSON to fall back on, so a type that stops satisfying
// them is encoded by the default struct codec instead: a stored string becomes an empty document, with no error anywhere.
var (
	_ bson.ValueMarshaler   = String{}
	_ bson.ValueUnmarshaler = &String{}
)

// String must also satisfy rosetta's schema.ValueGetter and schema.ValueSetter, which
// its doc comments name. The shapes are copied here so the tests add no dependency.
var (
	_ interface{ GetValue() any }      = String{}
	_ interface{ SetValue(any) error } = &String{}
)
