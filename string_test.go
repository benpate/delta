package delta

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/benpate/rosetta/convert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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

// TestString_Reset verifies that Reset moves the baseline to the current value, as a save does
func TestString_Reset(t *testing.T) {

	value := NewString("Sarah Connor")
	value.Set("Sarah Reese")
	require.True(t, value.IsChanged())
	require.Equal(t, "Sarah Connor", value.Original())

	value.Reset()

	require.Equal(t, "Sarah Reese", value.Value(), "Reset does not change the value")
	require.Equal(t, "Sarah Reese", value.Original())
	require.False(t, value.IsChanged())
	require.True(t, value.NotChanged())

	// A second Reset changes nothing
	value.Reset()
	require.Equal(t, "Sarah Reese", value.Original())

	// The next change is measured from the new baseline
	value.Set("Sarah Connor")
	require.True(t, value.IsChanged())
	require.Equal(t, "Sarah Reese", value.Original())
}

// TestString_Clear verifies that Clear zeroes the current value and keeps the baseline
func TestString_Clear(t *testing.T) {

	value := NewString("Sarah Connor")

	value.Clear()

	require.Equal(t, "", value.Value())
	require.True(t, value.IsZero())
	require.Equal(t, "Sarah Connor", value.Original(), "Clear does not move the baseline")
	require.True(t, value.IsChanged())

	// Clearing a value that is already zero changes nothing
	empty := NewString("")
	empty.Clear()
	require.True(t, empty.IsZero())
	require.True(t, empty.NotChanged())
}

/******************************************
 * Interface guards
 ******************************************/

// String satisfies the JSON, formatting, and schema interfaces that its callers rely on
var (
	_ json.Marshaler                   = String{}
	_ json.Unmarshaler                 = &String{}
	_ fmt.Stringer                     = String{}
	_ interface{ GetValue() any }      = String{}
	_ interface{ SetValue(any) error } = &String{}
	_ interface{ IsZero() bool }       = String{}
)

/******************************************
 * Test helpers
 ******************************************/

// stringSample is one named input that the String tests are run against
type stringSample struct {
	name  string
	value string
}

// stringSamples returns one value from each class of string worth testing
func stringSamples() []stringSample {
	return []stringSample{
		{"empty", ""},
		{"simple", "Sarah Connor"},
		{"single space", " "},
		{"unicode", "日本語 🔺 é"},
		{"quotes and backslashes", `"quoted" \ back\\slash`},
		{"html characters", "<b>&</b>"},
		{"control characters", "line\nbreak\ttab\rreturn"},
		{"nul byte", "nul\x00byte"},
		{"line separators", "a\u2028b\u2029c"},
		{"invalid utf8", "a\xffb"},
		{"encoded surrogate", "\xed\xa0\x80"},
		{"truncated rune", "\xe2\x82"},
		{"replacement character", "\ufffd"},
		{"looks like json", `{"name":null}`},
		{"looks like an object id", "6b04f64736058a543006f9a6"},
		{"one megabyte", strings.Repeat("x", 1<<20)},
	}
}

// requireString asserts every reader on a String against the expected current and baseline values
func requireString(t *testing.T, value String, current string, original string) {
	t.Helper()

	require.Equal(t, current, value.Value(), "Value")
	require.Equal(t, current, value.String(), "String")
	require.Equal(t, any(current), value.GetValue(), "GetValue")
	require.Equal(t, current, *value.Pointer(), "Pointer")
	require.Equal(t, original, value.Original(), "Original")
	require.Equal(t, current != original, value.IsChanged(), "IsChanged")
	require.Equal(t, current == original, value.NotChanged(), "NotChanged")
	require.Equal(t, current == "", value.IsZero(), "IsZero")
}

// plainStringViaJSON returns what a plain string becomes after a trip through JSON
func plainStringViaJSON(t *testing.T, value string) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	result := ""
	require.NoError(t, json.Unmarshal(encoded, &result))
	return result
}

/******************************************
 * Data Manipulation Methods
 ******************************************/

// TestString_ZeroValue verifies that an undeclared String is empty and unchanged
func TestString_ZeroValue(t *testing.T) {

	var value String

	requireString(t, value, "", "")
	require.Equal(t, NewString(""), value)
}

// TestString_New verifies that a new String reports its value as both current and baseline
func TestString_New(t *testing.T) {

	for _, sample := range stringSamples() {
		t.Run(sample.name, func(t *testing.T) {
			requireString(t, NewString(sample.value), sample.value, sample.value)
		})
	}
}

// TestString_Set verifies how Set moves the current value against a fixed baseline
func TestString_Set(t *testing.T) {

	for _, sample := range stringSamples() {
		t.Run("sample/"+sample.name, func(t *testing.T) {
			value := NewString("baseline")
			value.Set(sample.value)
			requireString(t, value, sample.value, "baseline")
		})
	}

	t.Run("same value is not a change", func(t *testing.T) {
		value := NewString("baseline")
		value.Set("baseline")
		requireString(t, value, "baseline", "baseline")
	})

	t.Run("returning to the baseline undoes the change", func(t *testing.T) {
		value := NewString("baseline")
		value.Set("other")
		require.True(t, value.IsChanged())
		value.Set("baseline")
		requireString(t, value, "baseline", "baseline")
	})

	t.Run("the baseline is not the previous value", func(t *testing.T) {
		value := NewString("baseline")
		value.Set("first")
		value.Set("second")
		requireString(t, value, "second", "baseline")
	})

	t.Run("an equal string built at run time is not a change", func(t *testing.T) {
		value := NewString("baseline")
		value.Set(strings.Join([]string{"base", "line"}, ""))
		require.True(t, value.NotChanged())
	})

	t.Run("comparison is exact", func(t *testing.T) {

		// Case, whitespace, and Unicode normalization forms all count as changes
		for _, other := range []string{"Baseline", "baseline ", " baseline", "baseline\x00", "base\u200bline"} {
			value := NewString("baseline")
			value.Set(other)
			require.True(t, value.IsChanged(), "setting %q", other)
		}

		composed := NewString("\u00e9")
		composed.Set("e\u0301")
		require.True(t, composed.IsChanged())
	})

	t.Run("zero value", func(t *testing.T) {
		var value String
		value.Set("first")
		requireString(t, value, "first", "")
	})
}

// TestString_Pointer verifies that the pointer aliases the current value and nothing else
func TestString_Pointer(t *testing.T) {

	t.Run("a write through the pointer is a tracked change", func(t *testing.T) {
		value := NewString("baseline")
		*value.Pointer() = "written"
		requireString(t, value, "written", "baseline")
	})

	t.Run("the pointer follows later changes", func(t *testing.T) {
		value := NewString("baseline")
		pointer := value.Pointer()

		value.Set("first")
		require.Equal(t, "first", *pointer)

		value.Clear()
		require.Equal(t, "", *pointer)

		require.NoError(t, value.SetValue(7))
		require.Equal(t, "7", *pointer)

		require.Same(t, pointer, value.Pointer())
	})

	t.Run("the pointer survives a Reset", func(t *testing.T) {
		value := NewString("baseline")
		pointer := value.Pointer()
		value.Set("saved")
		value.Reset()

		*pointer = "edited"
		requireString(t, value, "edited", "saved")
	})

	t.Run("the pointer never reaches the baseline", func(t *testing.T) {
		value := NewString("baseline")
		*value.Pointer() = ""
		require.Equal(t, "baseline", value.Original())
	})
}

// TestString_Copy verifies that a copied String is independent of its source
func TestString_Copy(t *testing.T) {

	source := NewString("baseline")
	source.Set("current")

	duplicate := source
	duplicate.Set("duplicate")
	duplicate.Reset()

	requireString(t, source, "current", "baseline")
	requireString(t, duplicate, "duplicate", "duplicate")

	// A value-receiver reader cannot disturb the value it was called on
	_ = source.Value()
	_ = source.Original()
	_ = source.IsChanged()
	requireString(t, source, "current", "baseline")
}

// TestString_Equality verifies that == compares the baseline as well as the current value
func TestString_Equality(t *testing.T) {

	first, second := NewString("a"), NewString("a")
	require.True(t, first == second)
	require.False(t, first == NewString("b"))

	changed := NewString("b")
	changed.Set("a")
	require.False(t, NewString("a") == changed, "same current value, different baseline")

	changed.Reset()
	require.True(t, NewString("a") == changed)
}

// TestString_ResetAndClear verifies how Reset and Clear combine with each other
func TestString_ResetAndClear(t *testing.T) {

	t.Run("reset on an unchanged value changes nothing", func(t *testing.T) {
		value := NewString("baseline")
		value.Reset()
		requireString(t, value, "baseline", "baseline")
	})

	t.Run("reset on the zero value changes nothing", func(t *testing.T) {
		var value String
		value.Reset()
		requireString(t, value, "", "")
	})

	t.Run("clear then reset makes empty the baseline", func(t *testing.T) {
		value := NewString("baseline")
		value.Clear()
		value.Reset()
		requireString(t, value, "", "")
	})

	t.Run("clear twice is the same as once", func(t *testing.T) {
		value := NewString("baseline")
		value.Clear()
		value.Clear()
		requireString(t, value, "", "baseline")
	})

	t.Run("clear on the zero value changes nothing", func(t *testing.T) {
		var value String
		value.Clear()
		requireString(t, value, "", "")
	})

	t.Run("reset after a pointer write", func(t *testing.T) {
		value := NewString("baseline")
		*value.Pointer() = "written"
		value.Reset()
		requireString(t, value, "written", "written")
	})

	t.Run("reset after SetValue", func(t *testing.T) {
		value := NewString("baseline")
		require.NoError(t, value.SetValue(12))
		value.Reset()
		requireString(t, value, "12", "12")
	})

	t.Run("every sample", func(t *testing.T) {
		for _, sample := range stringSamples() {
			value := NewString("baseline")
			value.Set(sample.value)
			value.Reset()
			requireString(t, value, sample.value, sample.value)

			value.Clear()
			requireString(t, value, "", sample.value)
		}
	})
}

/******************************************
 * Schema Interfaces
 ******************************************/

// TestString_GetValue verifies that GetValue returns a plain string holding the current value
func TestString_GetValue(t *testing.T) {

	value := NewString("baseline")
	value.Set("current")

	result, isString := value.GetValue().(string)
	require.True(t, isString)
	require.Equal(t, "current", result)
}

// TestString_SetValue verifies the conversion of each kind of input, and that the baseline holds
func TestString_SetValue(t *testing.T) {

	check := func(name string, input any, want string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewString("baseline")
			require.NoError(t, value.SetValue(input))
			requireString(t, value, want, "baseline")
		})
	}

	objectID, err := primitive.ObjectIDFromHex("6b04f64736058a543006f9a6")
	require.NoError(t, err)

	check("string", "text", "text")
	check("empty string", "", "")
	check("the baseline itself", "baseline", "baseline")
	check("nil", nil, "")
	check("int", 42, "42")
	check("negative int", -42, "-42")
	check("int64", int64(1)<<62, "4611686018427387904")
	check("bool true", true, "true")
	check("bool false", false, "false")
	check("bytes", []byte("bytes"), "bytes")
	check("empty bytes", []byte{}, "")
	check("invalid utf8", "a\xffb", "a\xffb")
	check("stringer", objectID, "6b04f64736058a543006f9a6")
	check("delta.String", NewString("wrapped"), "wrapped")
	check("delta.ObjectID", NewObjectID(objectID), "6b04f64736058a543006f9a6")
	check("unconvertible", struct{}{}, "")

	t.Run("matches rosetta for every sample", func(t *testing.T) {
		for _, sample := range stringSamples() {
			value := NewString("baseline")
			require.NoError(t, value.SetValue(sample.value))
			requireString(t, value, sample.value, "baseline")
		}
	})
}

/******************************************
 * JSON Serialization
 ******************************************/

// TestString_MarshalJSON verifies that a String encodes exactly as a plain string does
func TestString_MarshalJSON(t *testing.T) {

	for _, sample := range stringSamples() {
		t.Run(sample.name, func(t *testing.T) {
			want, err := json.Marshal(sample.value)
			require.NoError(t, err)

			got, err := json.Marshal(NewString(sample.value))
			require.NoError(t, err)
			require.Equal(t, string(want), string(got))

			direct, err := NewString(sample.value).MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, string(want), string(direct))
		})
	}

	t.Run("literal output", func(t *testing.T) {
		encoded, err := json.Marshal(NewString(`a"b`))
		require.NoError(t, err)
		require.Equal(t, `"a\"b"`, string(encoded))
	})

	t.Run("the current value is written, not the baseline", func(t *testing.T) {
		value := NewString("baseline")
		value.Set("current")

		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, `"current"`, string(encoded))
	})

	t.Run("through a pointer", func(t *testing.T) {
		value := NewString("pointer")
		encoded, err := json.Marshal(&value)
		require.NoError(t, err)
		require.Equal(t, `"pointer"`, string(encoded))
	})
}

// TestString_UnmarshalJSON verifies decoding onto a String that already holds an unsaved change
func TestString_UnmarshalJSON(t *testing.T) {

	// start returns a String whose current value and baseline differ
	start := func() String {
		return String{current: "current", original: "baseline"}
	}

	check := func(name string, input string, want string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := start()
			require.NoError(t, value.UnmarshalJSON([]byte(input)))
			requireString(t, value, want, want)
		})
	}

	checkErr := func(name string, input []byte) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := start()
			err := value.UnmarshalJSON(input)
			require.Error(t, err)
			require.ErrorContains(t, err, "delta.String.UnmarshalJSON")
			require.Equal(t, start(), value, "a failed decode leaves the String as it was")
		})
	}

	check("simple", `"Sarah Connor"`, "Sarah Connor")
	check("empty", `""`, "")
	check("escaped quote", `"a\"b"`, `a"b`)
	check("unicode escape", `"\u00e9"`, "é")
	check("surrogate pair", `"\ud83d\udd3a"`, "🔺")
	check("lone surrogate becomes the replacement character", `"\ud800"`, "\ufffd")
	check("raw utf8", `"日本語"`, "日本語")
	check("escaped nul", `"a\u0000b"`, "a\x00b")
	check("surrounding whitespace", " \n\t\"padded\" ", "padded")
	check("looks like json", `"{\"a\":null}"`, `{"a":null}`)
	check("one megabyte", `"`+strings.Repeat("x", 1<<20)+`"`, strings.Repeat("x", 1<<20))

	check("null becomes empty", `null`, "")

	checkErr("nil input", nil)
	checkErr("empty input", []byte{})
	checkErr("whitespace only", []byte(" "))
	checkErr("number", []byte(`12`))
	checkErr("bool", []byte(`true`))
	checkErr("object", []byte(`{}`))
	checkErr("array", []byte(`["a"]`))
	checkErr("unterminated string", []byte(`"a`))
	checkErr("trailing data", []byte(`"a"x`))
	checkErr("two values", []byte(`"a" "b"`))
	checkErr("single quotes", []byte(`'a'`))
	checkErr("bare word", []byte(`a`))
	checkErr("invalid escape", []byte(`"\x"`))
	checkErr("raw control character", []byte("\"a\nb\""))
	checkErr("mixed-case null", []byte(`NULL`))
}

// TestString_JSONRoundTrip verifies that valid UTF-8 survives a trip through JSON unchanged
func TestString_JSONRoundTrip(t *testing.T) {

	for _, sample := range stringSamples() {
		t.Run(sample.name, func(t *testing.T) {
			source := NewString("baseline")
			source.Set(sample.value)

			encoded, err := json.Marshal(source)
			require.NoError(t, err)

			var result String
			require.NoError(t, json.Unmarshal(encoded, &result))

			want := plainStringViaJSON(t, sample.value)
			requireString(t, result, want, want)

			if utf8.ValidString(sample.value) {
				require.Equal(t, sample.value, result.Value())
			}
		})
	}
}

// TestString_JSONStruct verifies how a String behaves as a field of a larger document
func TestString_JSONStruct(t *testing.T) {

	t.Run("wire format matches a plain string field", func(t *testing.T) {
		type wrapped struct {
			Name String `json:"name"`
		}
		type plain struct {
			Name string `json:"name"`
		}

		for _, sample := range stringSamples() {
			want, err := json.Marshal(plain{Name: sample.value})
			require.NoError(t, err)

			got, err := json.Marshal(wrapped{Name: NewString(sample.value)})
			require.NoError(t, err)
			require.Equal(t, string(want), string(got), sample.name)
		}
	})

	t.Run("omitempty does not omit, omitzero does", func(t *testing.T) {
		record := struct {
			Empty String `json:"empty,omitempty"`
			Zero  String `json:"zero,omitzero"`
			Full  String `json:"full,omitzero"`
		}{Full: NewString("x")}

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.JSONEq(t, `{"empty":"","full":"x"}`, string(encoded))
	})

	t.Run("omitzero follows the current value, not the baseline", func(t *testing.T) {
		record := struct {
			Cleared String `json:"cleared,omitzero"`
			Filled  String `json:"filled,omitzero"`
		}{Cleared: NewString("x")}

		record.Cleared.Clear()
		record.Filled.Set("y")

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.JSONEq(t, `{"filled":"y"}`, string(encoded))
	})

	t.Run("a missing field is left alone", func(t *testing.T) {
		record := struct {
			Name String `json:"name"`
		}{Name: String{current: "current", original: "baseline"}}

		require.NoError(t, json.Unmarshal([]byte(`{"other":"x"}`), &record))
		requireString(t, record.Name, "current", "baseline")
	})

	t.Run("a null field becomes empty", func(t *testing.T) {
		record := struct {
			Name String `json:"name"`
		}{Name: String{current: "current", original: "baseline"}}

		require.NoError(t, json.Unmarshal([]byte(`{"name":null}`), &record))
		requireString(t, record.Name, "", "")
	})

	t.Run("a null field on a fresh record stays empty", func(t *testing.T) {
		record := struct {
			Name String `json:"name"`
		}{}

		require.NoError(t, json.Unmarshal([]byte(`{"name":null}`), &record))
		requireString(t, record.Name, "", "")
	})

	t.Run("a repeated field keeps the last value", func(t *testing.T) {
		record := struct {
			Name String `json:"name"`
		}{}

		require.NoError(t, json.Unmarshal([]byte(`{"name":"first","name":"second"}`), &record))
		requireString(t, record.Name, "second", "second")

		require.NoError(t, json.Unmarshal([]byte(`{"name":"first","name":null}`), &record))
		requireString(t, record.Name, "", "")
	})

	t.Run("a wrong type fails and leaves the field alone", func(t *testing.T) {
		record := struct {
			Name String `json:"name"`
		}{Name: String{current: "current", original: "baseline"}}

		require.Error(t, json.Unmarshal([]byte(`{"name":12}`), &record))
		requireString(t, record.Name, "current", "baseline")
	})

	t.Run("pointer field", func(t *testing.T) {
		record := struct {
			Name *String `json:"name"`
		}{}

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.Equal(t, `{"name":null}`, string(encoded))

		require.NoError(t, json.Unmarshal([]byte(`{"name":"x"}`), &record))
		require.NotNil(t, record.Name)
		requireString(t, *record.Name, "x", "x")

		require.NoError(t, json.Unmarshal([]byte(`{"name":null}`), &record))
		require.Nil(t, record.Name)
	})

	t.Run("slice and map", func(t *testing.T) {
		record := struct {
			List  []String          `json:"list"`
			Index map[string]String `json:"index"`
		}{}

		require.NoError(t, json.Unmarshal([]byte(`{"list":["a","b"],"index":{"k":"v"}}`), &record))
		require.Equal(t, []String{NewString("a"), NewString("b")}, record.List)
		require.Equal(t, map[string]String{"k": NewString("v")}, record.Index)

		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.JSONEq(t, `{"list":["a","b"],"index":{"k":"v"}}`, string(encoded))
	})

	t.Run("reloading a changed record makes it unchanged", func(t *testing.T) {
		record := struct {
			Name String `json:"name"`
		}{Name: String{current: "current", original: "baseline"}}

		require.NoError(t, json.Unmarshal([]byte(`{"name":"loaded"}`), &record))
		requireString(t, record.Name, "loaded", "loaded")
	})
}

/******************************************
 * BSON Serialization
 ******************************************/

// TestString_MarshalBSONValue verifies that a String encodes exactly as a plain string does
func TestString_MarshalBSONValue(t *testing.T) {

	for _, sample := range stringSamples() {
		t.Run(sample.name, func(t *testing.T) {
			wantType, wantData, err := bson.MarshalValue(sample.value)
			require.NoError(t, err)

			gotType, gotData, err := NewString(sample.value).MarshalBSONValue()
			require.NoError(t, err)
			require.Equal(t, bson.TypeString, gotType)
			require.Equal(t, wantType, gotType)
			require.Equal(t, wantData, gotData)
		})
	}

	t.Run("literal output", func(t *testing.T) {
		_, data, err := NewString("hi").MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, []byte{3, 0, 0, 0, 'h', 'i', 0}, data)
	})

	t.Run("the current value is written, not the baseline", func(t *testing.T) {
		value := NewString("baseline")
		value.Set("hi")

		_, data, err := value.MarshalBSONValue()
		require.NoError(t, err)
		require.Equal(t, []byte{3, 0, 0, 0, 'h', 'i', 0}, data)
	})
}

// TestString_UnmarshalBSONValue verifies decoding onto a String that already holds an unsaved change
func TestString_UnmarshalBSONValue(t *testing.T) {

	// start returns a String whose current value and baseline differ
	start := func() String {
		return String{current: "current", original: "baseline"}
	}

	check := func(name string, source any, want string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			bsonType, data, err := bson.MarshalValue(source)
			require.NoError(t, err)

			value := start()
			require.NoError(t, value.UnmarshalBSONValue(bsonType, data))
			requireString(t, value, want, want)
		})
	}

	checkErr := func(name string, bsonType bsontype.Type, data []byte) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := start()
			err := value.UnmarshalBSONValue(bsonType, data)
			require.Error(t, err)
			require.ErrorContains(t, err, "delta.String.UnmarshalBSONValue")
			require.Equal(t, start(), value, "a failed decode leaves the String as it was")
		})
	}

	checkErrFrom := func(name string, source any) {
		t.Helper()
		bsonType, data, err := bson.MarshalValue(source)
		require.NoError(t, err)
		checkErr(name, bsonType, data)
	}

	objectID, err := primitive.ObjectIDFromHex("6b04f64736058a543006f9a6")
	require.NoError(t, err)

	for _, sample := range stringSamples() {
		check("sample/"+sample.name, sample.value, sample.value)
	}

	// The driver's string decoder also accepts these neighbouring BSON types
	check("symbol", primitive.Symbol("symbol"), "symbol")
	check("object id becomes hex", objectID, "6b04f64736058a543006f9a6")
	check("binary becomes raw bytes", primitive.Binary{Data: []byte("raw")}, "raw")
	check("null becomes empty", primitive.Null{}, "")
	check("undefined becomes empty", primitive.Undefined{}, "")

	checkErrFrom("boolean", true)
	checkErrFrom("int32", int32(7))
	checkErrFrom("int64", int64(7))
	checkErrFrom("double", 1.5)
	checkErrFrom("document", bson.M{"a": "b"})
	checkErrFrom("array", bson.A{"a"})
	checkErrFrom("datetime", primitive.DateTime(5))
	checkErrFrom("timestamp", primitive.Timestamp{T: 1, I: 2})
	checkErrFrom("regex", primitive.Regex{Pattern: "a"})
	checkErrFrom("javascript", primitive.JavaScript("a"))
	checkErrFrom("decimal128", primitive.NewDecimal128(1, 2))
	checkErrFrom("min key", primitive.MinKey{})
	checkErrFrom("max key", primitive.MaxKey{})

	checkErr("nil data", bson.TypeString, nil)
	checkErr("empty data", bson.TypeString, []byte{})
	checkErr("length prefix only", bson.TypeString, []byte{1, 0, 0, 0})
	checkErr("truncated body", bson.TypeString, []byte{5, 0, 0, 0, 'a'})
	checkErr("zero length", bson.TypeString, []byte{0, 0, 0, 0, 0})
	checkErr("negative length", bson.TypeString, []byte{0xff, 0xff, 0xff, 0xff, 'a', 0})
	checkErr("huge length", bson.TypeString, []byte{0xff, 0xff, 0xff, 0x7f, 'a', 0})
	checkErr("missing terminator", bson.TypeString, []byte{2, 0, 0, 0, 'a', 'b'})
	checkErr("type zero", bsontype.Type(0), nil)
	checkErr("unknown type", bsontype.Type(0x42), []byte{1, 0, 0, 0, 0})
}

// TestString_BSONDocument verifies how a String behaves as a field of a stored record
func TestString_BSONDocument(t *testing.T) {

	type wrapped struct {
		Name String `bson:"name"`
	}
	type plain struct {
		Name string `bson:"name"`
	}

	t.Run("wire format matches a plain string field", func(t *testing.T) {
		for _, sample := range stringSamples() {
			want, err := bson.Marshal(plain{Name: sample.value})
			require.NoError(t, err)

			got, err := bson.Marshal(wrapped{Name: NewString(sample.value)})
			require.NoError(t, err)
			require.Equal(t, want, got, sample.name)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		for _, sample := range stringSamples() {
			source := wrapped{Name: NewString("baseline")}
			source.Name.Set(sample.value)

			document, err := bson.Marshal(source)
			require.NoError(t, err)

			result := wrapped{}
			require.NoError(t, bson.Unmarshal(document, &result))
			requireString(t, result.Name, sample.value, sample.value)
		}
	})

	t.Run("a document written from a plain field loads into a String", func(t *testing.T) {
		document, err := bson.Marshal(plain{Name: "stored"})
		require.NoError(t, err)

		result := wrapped{}
		require.NoError(t, bson.Unmarshal(document, &result))
		requireString(t, result.Name, "stored", "stored")
	})

	t.Run("a String loads into a plain field", func(t *testing.T) {
		document, err := bson.Marshal(wrapped{Name: NewString("stored")})
		require.NoError(t, err)

		result := plain{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, "stored", result.Name)
	})

	t.Run("omitempty follows the current value", func(t *testing.T) {
		record := struct {
			Empty   String `bson:"empty,omitempty"`
			Cleared String `bson:"cleared,omitempty"`
			Filled  String `bson:"filled,omitempty"`
		}{Cleared: NewString("x")}

		record.Cleared.Clear()
		record.Filled.Set("y")

		document, err := bson.Marshal(record)
		require.NoError(t, err)

		result := bson.M{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, bson.M{"filled": "y"}, result)
	})

	t.Run("a missing field is left alone", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"other": "x"})
		require.NoError(t, err)

		result := wrapped{Name: String{current: "current", original: "baseline"}}
		require.NoError(t, bson.Unmarshal(document, &result))
		requireString(t, result.Name, "current", "baseline")
	})

	t.Run("a null field becomes empty", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"name": nil})
		require.NoError(t, err)

		result := wrapped{Name: String{current: "current", original: "baseline"}}
		require.NoError(t, bson.Unmarshal(document, &result))
		requireString(t, result.Name, "", "")
	})

	t.Run("a wrong type fails and leaves the field alone", func(t *testing.T) {
		document, err := bson.Marshal(bson.M{"name": 12})
		require.NoError(t, err)

		result := wrapped{Name: String{current: "current", original: "baseline"}}
		require.Error(t, bson.Unmarshal(document, &result))
		requireString(t, result.Name, "current", "baseline")
	})

	t.Run("pointer field", func(t *testing.T) {
		type pointered struct {
			Name *String `bson:"name"`
		}

		document, err := bson.Marshal(pointered{})
		require.NoError(t, err)

		raw := bson.M{}
		require.NoError(t, bson.Unmarshal(document, &raw))
		require.Equal(t, bson.M{"name": nil}, raw)

		value := NewString("stored")
		document, err = bson.Marshal(pointered{Name: &value})
		require.NoError(t, err)

		result := pointered{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.NotNil(t, result.Name)
		requireString(t, *result.Name, "stored", "stored")
	})

	t.Run("slice and map", func(t *testing.T) {
		type collections struct {
			List  []String          `bson:"list"`
			Index map[string]String `bson:"index"`
		}

		source := collections{
			List:  []String{NewString("a"), NewString("b")},
			Index: map[string]String{"k": NewString("v")},
		}

		document, err := bson.Marshal(source)
		require.NoError(t, err)

		raw := bson.M{}
		require.NoError(t, bson.Unmarshal(document, &raw))
		require.Equal(t, bson.M{"list": bson.A{"a", "b"}, "index": bson.M{"k": "v"}}, raw)

		result := collections{}
		require.NoError(t, bson.Unmarshal(document, &result))
		require.Equal(t, source, result)
	})

	t.Run("save and reload cycle", func(t *testing.T) {

		// Load a record, edit it, save it, and Reset, as a service does
		document, err := bson.Marshal(plain{Name: "stored"})
		require.NoError(t, err)

		record := wrapped{}
		require.NoError(t, bson.Unmarshal(document, &record))

		record.Name.Set("edited")
		requireString(t, record.Name, "edited", "stored")

		saved, err := bson.Marshal(record)
		require.NoError(t, err)
		requireString(t, record.Name, "edited", "stored")

		record.Name.Reset()
		requireString(t, record.Name, "edited", "edited")

		reloaded := wrapped{}
		require.NoError(t, bson.Unmarshal(saved, &reloaded))
		require.Equal(t, record, reloaded)
	})
}

/******************************************
 * Fuzz Tests
 ******************************************/

// FuzzString_Operations runs random sequences of mutations against a two-variable model
func FuzzString_Operations(f *testing.F) {

	f.Add("baseline", "first", "second", []byte{0, 1, 2, 3, 4, 5, 6, 7, 8})
	f.Add("", "", "", []byte{2, 3, 0, 3})
	f.Add("a\xffb", "\x00", "🔺", []byte{0, 7, 1, 8, 6, 3, 2})
	f.Add("same", "same", "same", []byte{0, 1, 4, 5})
	f.Add("x", "y", "x", []byte{0, 3, 1, 3, 0, 6})

	f.Fuzz(func(t *testing.T, initial string, first string, second string, operations []byte) {

		value := NewString(initial)
		current, original := initial, initial
		requireString(t, value, current, original)

		for _, operation := range operations {

			switch operation % 9 {

			case 0:
				value.Set(first)
				current = first

			case 1:
				value.Set(second)
				current = second

			case 2:
				value.Clear()
				current = ""

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
				value.Set(value.Original())
				current = original

			case 7:
				encoded, err := json.Marshal(value)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(encoded, &value))
				current = plainStringViaJSON(t, current)
				original = current

			case 8:
				bsonType, data, err := value.MarshalBSONValue()
				require.NoError(t, err)
				require.NoError(t, value.UnmarshalBSONValue(bsonType, data))
				original = current
			}

			requireString(t, value, current, original)
		}
	})
}

// FuzzString_SetValue verifies that SetValue agrees with rosetta for every primitive input
func FuzzString_SetValue(f *testing.F) {

	f.Add("text", int64(42), 1.5, true, []byte("bytes"))
	f.Add("", int64(0), 0.0, false, []byte{})
	f.Add("a\xffb", int64(-1)<<63, -1.5, true, []byte{0xff, 0x00})

	f.Fuzz(func(t *testing.T, text string, integer int64, float float64, boolean bool, data []byte) {

		inputs := []any{text, integer, int(integer), int32(integer), float, float32(float), boolean, data, nil, &text}

		for _, input := range inputs {
			value := NewString("baseline")
			require.NoError(t, value.SetValue(input))
			requireString(t, value, convert.String(input), "baseline")
		}

		// A string always passes through untouched
		value := NewString("baseline")
		require.NoError(t, value.SetValue(text))
		require.Equal(t, text, value.Value())
	})
}

// FuzzString_RoundTrip verifies that any string keeps the plain wire format in both encodings
func FuzzString_RoundTrip(f *testing.F) {

	for _, sample := range stringSamples() {
		if len(sample.value) < 1024 {
			f.Add(sample.value, "baseline")
		}
	}

	f.Fuzz(func(t *testing.T, text string, baseline string) {

		type wrapped struct {
			Value String `json:"value" bson:"value"`
		}
		type plain struct {
			Value string `json:"value" bson:"value"`
		}

		source := wrapped{Value: NewString(baseline)}
		source.Value.Set(text)

		// JSON writes the same bytes as a plain string and reads back the same value
		wantJSON, err := json.Marshal(plain{Value: text})
		require.NoError(t, err)

		gotJSON, err := json.Marshal(source)
		require.NoError(t, err)
		require.Equal(t, string(wantJSON), string(gotJSON))

		fromJSON := wrapped{}
		require.NoError(t, json.Unmarshal(gotJSON, &fromJSON))

		wantText := plainStringViaJSON(t, text)
		requireString(t, fromJSON.Value, wantText, wantText)

		if utf8.ValidString(text) {
			require.Equal(t, text, fromJSON.Value.Value())
		}

		// BSON writes the same bytes as a plain string and reads back the same value
		wantBSON, err := bson.Marshal(plain{Value: text})
		require.NoError(t, err)

		gotBSON, err := bson.Marshal(source)
		require.NoError(t, err)
		require.Equal(t, wantBSON, gotBSON)

		fromBSON := wrapped{}
		require.NoError(t, bson.Unmarshal(gotBSON, &fromBSON))
		requireString(t, fromBSON.Value, text, text)

		// Marshalling never disturbs the source
		requireString(t, source.Value, text, baseline)
	})
}

// FuzzString_UnmarshalJSON verifies that arbitrary JSON decodes exactly as it does for a plain string
func FuzzString_UnmarshalJSON(f *testing.F) {

	seeds := []string{
		`"Sarah Connor"`, `""`, `null`, `12`, `true`, `{}`, `[]`, `"a`, `"a"x`, ``, ` "a" `,
		`"\ud800"`, `"\ud83d\udd3a"`, `"\u0000"`, `'a'`, `"\x"`, "\"a\xffb\"", `nul`, `"a" "b"`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		// A plain string that starts empty is the oracle, which reads a JSON null as empty
		plain := ""
		value := String{current: "current", original: "baseline"}

		if err := json.Unmarshal(data, &plain); err != nil {
			require.Error(t, value.UnmarshalJSON(data))
			require.Equal(t, String{current: "current", original: "baseline"}, value)
			return
		}

		require.NoError(t, value.UnmarshalJSON(data))
		requireString(t, value, plain, plain)

		// The same input reaches the same result through encoding/json
		viaPackage := String{current: "current", original: "baseline"}
		require.NoError(t, json.Unmarshal(data, &viaPackage))
		require.Equal(t, value, viaPackage)
	})
}

// FuzzString_UnmarshalJSONDocument verifies that a String field decodes as a plain string field does
func FuzzString_UnmarshalJSONDocument(f *testing.F) {

	seeds := []string{
		`{"value":"a"}`, `{"value":null}`, `{"value":12}`, `{}`, `{"value":"a","value":"b"}`,
		`{"value":"a","value":null}`, `{"VALUE":"a"}`, `{"value":{"a":1}}`, `{"value":["a"]}`,
		`{"value":"a"`, `[]`, `null`, `{"value":"\ud800"}`, `{"other":1,"value":"a"}`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		// A pointer is the oracle, because it is the plain type that a JSON null resets
		plain := struct {
			Value *string `json:"value"`
		}{}
		plainErr := json.Unmarshal(data, &plain)

		wrapped := struct {
			Value String `json:"value"`
		}{}
		wrappedErr := json.Unmarshal(data, &wrapped)

		require.Equal(t, plainErr == nil, wrappedErr == nil, "plain: %v, wrapped: %v", plainErr, wrappedErr)

		// Baseline and current value always move together during a load
		require.True(t, wrapped.Value.NotChanged())

		if plainErr != nil {
			return
		}

		want := ""
		if plain.Value != nil {
			want = *plain.Value
		}

		requireString(t, wrapped.Value, want, want)
	})
}

// FuzzString_UnmarshalBSONValue verifies that arbitrary BSON decodes exactly as it does for a plain string
func FuzzString_UnmarshalBSONValue(f *testing.F) {

	sources := []any{
		"Sarah Connor", "", "a\xffb", primitive.Symbol("symbol"), primitive.NewObjectID(),
		primitive.Binary{Data: []byte("raw")}, primitive.Null{}, primitive.Undefined{},
		true, int32(7), int64(7), 1.5, bson.M{"a": "b"}, bson.A{"a"}, primitive.DateTime(5),
		primitive.JavaScript("a"), primitive.Regex{Pattern: "a"}, primitive.Timestamp{T: 1},
		primitive.CodeWithScope{Code: "a", Scope: bson.M{}}, primitive.DBPointer{DB: "a"},
	}
	for _, source := range sources {
		bsonType, data, err := bson.MarshalValue(source)
		require.NoError(f, err)
		f.Add(byte(bsonType), data)
	}

	f.Add(byte(bson.TypeString), []byte{5, 0, 0, 0, 'a'})
	f.Add(byte(bson.TypeString), []byte{0xff, 0xff, 0xff, 0xff, 'a', 0})
	f.Add(byte(bson.TypeString), []byte{2, 0, 0, 0, 'a', 'b'})
	f.Add(byte(0), []byte{})

	f.Fuzz(func(t *testing.T, bsonType byte, data []byte) {

		// A plain string that starts at the baseline is the oracle
		plain := "baseline"
		value := String{current: "current", original: "baseline"}

		if err := bson.UnmarshalValue(bsontype.Type(bsonType), data, &plain); err != nil {
			require.Error(t, value.UnmarshalBSONValue(bsontype.Type(bsonType), data))
			require.Equal(t, String{current: "current", original: "baseline"}, value)
			return
		}

		require.NoError(t, value.UnmarshalBSONValue(bsontype.Type(bsonType), data))
		requireString(t, value, plain, plain)
	})
}

// FuzzString_UnmarshalBSONDocument verifies that a String field decodes as a plain string field does
func FuzzString_UnmarshalBSONDocument(f *testing.F) {

	sources := []bson.M{
		{"value": "a"}, {"value": ""}, {"value": nil}, {"value": 12}, {}, {"other": "a"},
		{"value": primitive.NewObjectID()}, {"value": bson.M{"a": "b"}}, {"value": bson.A{"a"}},
		{"value": primitive.Symbol("a")}, {"value": true}, {"value": primitive.Undefined{}},
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
			Value string `bson:"value"`
		}{}
		plainErr := bson.Unmarshal(data, &plain)

		wrapped := struct {
			Value String `bson:"value"`
		}{}
		wrappedErr := bson.Unmarshal(data, &wrapped)

		require.Equal(t, plainErr == nil, wrappedErr == nil, "plain: %v, wrapped: %v", plainErr, wrappedErr)

		// Baseline and current value always move together during a load
		require.True(t, wrapped.Value.NotChanged())

		if plainErr == nil {
			requireString(t, wrapped.Value, plain.Value, plain.Value)
		}
	})
}

/******************************************
 * Examples
 ******************************************/

// ExampleString shows a change being tracked against the value that was loaded
func ExampleString() {

	name := NewString("Sarah Connor")
	fmt.Println(name.IsChanged())

	name.Set("Sarah Reese")
	fmt.Println(name.IsChanged())
	fmt.Println(name.Value())
	fmt.Println(name.Original())

	// Output:
	// false
	// true
	// Sarah Reese
	// Sarah Connor
}

// ExampleString_Reset shows the baseline moving once the caller has saved the value
func ExampleString_Reset() {

	name := NewString("Sarah Connor")
	name.Set("Sarah Reese")

	name.Reset()
	fmt.Println(name.IsChanged())
	fmt.Println(name.Original())

	// Output:
	// false
	// Sarah Reese
}

// ExampleString_Clear shows that emptying a value counts as a change
func ExampleString_Clear() {

	name := NewString("Sarah Connor")
	name.Clear()

	fmt.Println(name.IsZero())
	fmt.Println(name.IsChanged())
	fmt.Println(name.Original())

	// Output:
	// true
	// true
	// Sarah Connor
}
