package delta

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

/******************************************
 * Interface guards
 ******************************************/

// Slice must satisfy both *Value BSON interfaces. Without them the default struct codec
// takes over, and a stored list becomes an empty document with no error anywhere.
var (
	_ bson.ValueMarshaler   = Slice[string]{}
	_ bson.ValueUnmarshaler = &Slice[string]{}
	_ json.Marshaler        = Slice[string]{}
	_ json.Unmarshaler      = &Slice[string]{}
)

// Slice must satisfy the rosetta schema interfaces, which are declared inline here
// so that the tests do not import the schema package.
var (
	_ interface{ GetValue() any }            = Slice[string]{}
	_ interface{ SetValue(any) error }       = &Slice[string]{}
	_ interface{ IsZero() bool }             = Slice[string]{}
	_ interface{ Length() int }              = Slice[string]{}
	_ interface{ GetIndex(int) (any, bool) } = Slice[string]{}
	_ interface{ SetIndex(int, any) bool }   = &Slice[string]{}
)

/******************************************
 * Test helpers
 ******************************************/

// legacySlice has the layout of rosetta's delta.Slice, which is how the legacy documents were written
type legacySlice[T comparable] struct {
	Values  []T `json:"values" bson:"values"`
	Added   []T `json:"-"      bson:"-"`
	Deleted []T `json:"-"      bson:"-"`
}

// testList presents itself as a list through Length and GetIndex, as a form widget's value does
type testList []string

// Length implements the rosetta ArrayGetter interface
func (list testList) Length() int {
	return len(list)
}

// GetIndex implements the rosetta ArrayGetter interface
func (list testList) GetIndex(index int) (any, bool) {

	if (index < 0) || (index >= len(list)) {
		return nil, false
	}

	return list[index], true
}

// requireSlice asserts the current and baseline lists of a Slice, and every reader derived from them
func requireSlice[T comparable](t *testing.T, value Slice[T], current []T, original []T) {
	t.Helper()

	current = cloneSlice(current)
	original = cloneSlice(original)
	isEqual := slices.Equal(current, original)

	require.Equal(t, current, value.Value())
	require.Equal(t, original, value.Original())
	require.Equal(t, current, value.GetValue())
	require.Equal(t, len(current), value.Length())
	require.Equal(t, len(current) == 0, value.IsZero())
	require.Equal(t, isEqual, value.NotChanged())
	require.Equal(t, !isEqual, value.IsChanged())

	// The change lists are checked against a slower, more obvious implementation
	unchanged, added := naiveMatch(current, original)
	_, deleted := naiveMatch(original, current)

	require.Equal(t, added, value.Added())
	require.Equal(t, deleted, value.Deleted())
	require.Equal(t, unchanged, value.Unchanged())
}

// naiveMatch is the oracle for matchSlice. It pairs values by searching and removing.
func naiveMatch[T comparable](first []T, second []T) (matched []T, unmatched []T) {

	matched = make([]T, 0)
	unmatched = make([]T, 0)
	pool := cloneSlice(second)

	for _, value := range first {

		index := slices.Index(pool, value)

		if index < 0 {
			unmatched = append(unmatched, value)
			continue
		}

		pool = slices.Delete(pool, index, index+1)
		matched = append(matched, value)
	}

	return matched, unmatched
}

// changedSlice returns a Slice whose current list differs from its baseline
func changedSlice() Slice[string] {
	value := NewSlice("a", "b")
	value.Set([]string{"b", "c"})
	return value
}

// marshalBSONValue encodes a Go value as a BSON value, and fails the test if it cannot
func marshalBSONValue(t testing.TB, value any) (bsontype.Type, []byte) {
	t.Helper()

	valueType, data, err := bson.MarshalValue(value)
	require.NoError(t, err)

	return valueType, data
}

/******************************************
 * Data Manipulation Methods
 ******************************************/

// TestSlice verifies that a Slice tracks a change to its list
func TestSlice(t *testing.T) {

	value := NewSlice("mastodon", "bluesky")

	require.Equal(t, []string{"mastodon", "bluesky"}, value.Value())
	require.True(t, value.NotChanged())
	require.False(t, value.IsChanged())

	value.Set([]string{"bluesky", "threads"})

	require.True(t, value.IsChanged())
	require.Equal(t, []string{"threads"}, value.Added())
	require.Equal(t, []string{"mastodon"}, value.Deleted())
	require.Equal(t, []string{"bluesky"}, value.Unchanged())
	require.Equal(t, []string{"mastodon", "bluesky"}, value.Original())
}

// TestSlice_ZeroValue verifies that an undeclared Slice is empty, unchanged, and safe to use
func TestSlice_ZeroValue(t *testing.T) {

	var value Slice[string]

	requireSlice(t, value, nil, nil)
	require.NotNil(t, value.Value())
	require.NotNil(t, value.Original())
	require.NotNil(t, value.Added())
	require.NotNil(t, value.Deleted())
	require.NotNil(t, value.Unchanged())

	// Every writer works on the zero value
	value.Set([]string{"a"})
	requireSlice(t, value, []string{"a"}, nil)

	var indexed Slice[string]
	require.True(t, indexed.SetIndex(0, "a"))
	requireSlice(t, indexed, []string{"a"}, nil)
}

// TestSlice_New verifies that NewSlice sets both lists, and copies the caller's values
func TestSlice_New(t *testing.T) {

	t.Run("no values", func(t *testing.T) {
		requireSlice(t, NewSlice[string](), nil, nil)
	})

	t.Run("nil list", func(t *testing.T) {
		var values []string
		requireSlice(t, NewSlice(values...), nil, nil)
	})

	t.Run("values", func(t *testing.T) {
		requireSlice(t, NewSlice(1, 2, 3), []int{1, 2, 3}, []int{1, 2, 3})
	})

	t.Run("does not alias the caller's list", func(t *testing.T) {
		values := []string{"a", "b"}
		value := NewSlice(values...)

		values[0] = "changed"
		requireSlice(t, value, []string{"a", "b"}, []string{"a", "b"})

		// The current list and the baseline do not alias one another either
		require.True(t, value.SetIndex(0, "z"))
		requireSlice(t, value, []string{"z", "b"}, []string{"a", "b"})
	})
}

// TestSlice_Set verifies that Set replaces the current list with a copy, and leaves the baseline alone
func TestSlice_Set(t *testing.T) {

	t.Run("replaces", func(t *testing.T) {
		value := NewSlice("a", "b")
		value.Set([]string{"c"})
		requireSlice(t, value, []string{"c"}, []string{"a", "b"})
	})

	t.Run("nil empties", func(t *testing.T) {
		value := NewSlice("a")
		value.Set(nil)
		requireSlice(t, value, nil, []string{"a"})
	})

	t.Run("back to the baseline is unchanged", func(t *testing.T) {
		value := NewSlice("a", "b")
		value.Set([]string{"c"})
		value.Set([]string{"a", "b"})
		requireSlice(t, value, []string{"a", "b"}, []string{"a", "b"})
		require.False(t, value.IsChanged())
	})

	t.Run("does not alias the caller's list", func(t *testing.T) {
		values := []string{"a", "b"}
		value := NewSlice[string]()
		value.Set(values)

		values[0] = "changed"
		requireSlice(t, value, []string{"a", "b"}, nil)
	})

	t.Run("twice keeps the loaded baseline", func(t *testing.T) {
		value := NewSlice("a")
		value.Set([]string{"b"})
		value.Set([]string{"c"})
		requireSlice(t, value, []string{"c"}, []string{"a"})
	})
}

// TestSlice_Clear verifies that Clear empties the current list, and counts as a change
func TestSlice_Clear(t *testing.T) {

	value := NewSlice("a", "b")
	value.Clear()

	requireSlice(t, value, nil, []string{"a", "b"})
	require.True(t, value.IsChanged())
	require.Equal(t, []string{"a", "b"}, value.Deleted())

	// Clearing an empty Slice changes nothing
	empty := NewSlice[string]()
	empty.Clear()
	requireSlice(t, empty, nil, nil)
	require.False(t, empty.IsChanged())
}

// TestSlice_ValueIsACopy verifies that no reader hands out the Slice's own memory
func TestSlice_ValueIsACopy(t *testing.T) {

	value := changedSlice()

	value.Value()[0] = "changed"
	value.Original()[0] = "changed"
	value.GetValue().([]string)[0] = "changed"
	value.Added()[0] = "changed"
	value.Deleted()[0] = "changed"
	value.Unchanged()[0] = "changed"

	requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
}

// TestSlice_Pointer verifies that a write through the pointer is tracked like any other
func TestSlice_Pointer(t *testing.T) {

	t.Run("append", func(t *testing.T) {
		value := NewSlice("a")
		pointer := value.Pointer()
		*pointer = append(*pointer, "b")
		requireSlice(t, value, []string{"a", "b"}, []string{"a"})
	})

	t.Run("edit in place leaves the baseline alone", func(t *testing.T) {
		value := NewSlice("a", "b")
		(*value.Pointer())[0] = "z"
		requireSlice(t, value, []string{"z", "b"}, []string{"a", "b"})
	})

	t.Run("nil", func(t *testing.T) {
		value := NewSlice("a")
		*value.Pointer() = nil
		requireSlice(t, value, nil, []string{"a"})
		require.NotNil(t, value.Value())
	})

	t.Run("zero value", func(t *testing.T) {
		var value Slice[int]
		*value.Pointer() = []int{1}
		requireSlice(t, value, []int{1}, nil)
	})
}

// TestSlice_Reset verifies that Reset makes the current list the baseline
func TestSlice_Reset(t *testing.T) {

	value := changedSlice()
	value.Reset()

	requireSlice(t, value, []string{"b", "c"}, []string{"b", "c"})
	require.False(t, value.IsChanged())
	require.Empty(t, value.Added())
	require.Empty(t, value.Deleted())

	// The new baseline does not alias the current list
	require.True(t, value.SetIndex(0, "z"))
	requireSlice(t, value, []string{"z", "c"}, []string{"b", "c"})

	// Resetting twice changes nothing more
	value.Reset()
	value.Reset()
	requireSlice(t, value, []string{"z", "c"}, []string{"z", "c"})
}

// TestSlice_Copy verifies that a copied Slice shares no list with the one it was copied from
func TestSlice_Copy(t *testing.T) {

	first := NewSlice("a", "b")
	second := first

	second.Set([]string{"c"})

	requireSlice(t, first, []string{"a", "b"}, []string{"a", "b"})
	requireSlice(t, second, []string{"c"}, []string{"a", "b"})

	// SetIndex writes in place, so it must not write through to the copy's source
	third := first
	require.True(t, third.SetIndex(0, "z"))
	require.True(t, third.SetIndex(3, "y"))

	requireSlice(t, first, []string{"a", "b"}, []string{"a", "b"})
	requireSlice(t, third, []string{"z", "b", "", "y"}, []string{"a", "b"})

	// Reset on a copy leaves the source's baseline alone
	third.Reset()
	requireSlice(t, first, []string{"a", "b"}, []string{"a", "b"})
}

/******************************************
 * Change Lists
 ******************************************/

// TestSlice_ChangeLists covers Added, Deleted, Unchanged, and IsChanged for each kind of edit
func TestSlice_ChangeLists(t *testing.T) {

	check := func(name string, original []string, current []string, added []string, deleted []string, unchanged []string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewSlice(original...)
			value.Set(current)

			require.Equal(t, cloneSlice(added), value.Added(), "added")
			require.Equal(t, cloneSlice(deleted), value.Deleted(), "deleted")
			require.Equal(t, cloneSlice(unchanged), value.Unchanged(), "unchanged")
			require.Equal(t, !slices.Equal(original, current), value.IsChanged())
			requireSlice(t, value, current, original)
		})
	}

	list := func(values ...string) []string { return values }

	check("both empty", nil, nil, nil, nil, nil)
	check("same", list("a", "b"), list("a", "b"), nil, nil, list("a", "b"))
	check("added to empty", nil, list("a", "b"), list("a", "b"), nil, nil)
	check("emptied", list("a", "b"), nil, nil, list("a", "b"), nil)
	check("appended", list("a"), list("a", "b"), list("b"), nil, list("a"))
	check("prepended", list("b"), list("a", "b"), list("a"), nil, list("b"))
	check("removed first", list("a", "b"), list("b"), nil, list("a"), list("b"))
	check("removed last", list("a", "b"), list("a"), nil, list("b"), list("a"))
	check("replaced all", list("a", "b"), list("c", "d"), list("c", "d"), list("a", "b"), nil)
	check("replaced one", list("a", "b", "c"), list("a", "x", "c"), list("x"), list("b"), list("a", "c"))
	check("empty string is a value", list("a"), list("", "a"), list(""), nil, list("a"))

	// A reorder is a change, although nothing was added or deleted
	check("reordered", list("a", "b", "c"), list("c", "a", "b"), nil, nil, list("c", "a", "b"))

	// Duplicates are counted, one pair at a time
	check("duplicate added", list("a"), list("a", "a"), list("a"), nil, list("a"))
	check("duplicate removed", list("a", "a"), list("a"), nil, list("a"), list("a"))
	check("duplicates kept", list("a", "a"), list("a", "a"), nil, nil, list("a", "a"))
	check("duplicates swapped", list("a", "a", "b"), list("a", "b", "b"), list("b"), list("a"), list("a", "b"))

	// Results keep the order of the list they came from
	check("order of added", list("x"), list("c", "x", "a", "b"), list("c", "a", "b"), nil, list("x"))
	check("order of deleted", list("c", "x", "a", "b"), list("x"), nil, list("c", "a", "b"), list("x"))
}

// TestSlice_ChangeListsFollowTheBaseline verifies that the lists compare against what was
// loaded, not against the previous write
func TestSlice_ChangeListsFollowTheBaseline(t *testing.T) {

	value := NewSlice("a", "b")

	value.Set([]string{"b", "c"})
	value.Set([]string{"c", "d"})

	require.Equal(t, []string{"c", "d"}, value.Added())
	require.Equal(t, []string{"a", "b"}, value.Deleted())
	require.Empty(t, value.Unchanged())

	// A value that is removed and then restored was never deleted
	value.Set([]string{"a", "d"})
	require.Equal(t, []string{"d"}, value.Added())
	require.Equal(t, []string{"b"}, value.Deleted())
	require.Equal(t, []string{"a"}, value.Unchanged())
}

// TestSlice_ElementTypes verifies the change lists for element types other than string
func TestSlice_ElementTypes(t *testing.T) {

	t.Run("int", func(t *testing.T) {
		value := NewSlice(1, 2, 3)
		value.Set([]int{3, 4})
		require.Equal(t, []int{4}, value.Added())
		require.Equal(t, []int{1, 2}, value.Deleted())
		require.Equal(t, []int{3}, value.Unchanged())
	})

	t.Run("ObjectID", func(t *testing.T) {
		first, second, third := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
		value := NewSlice(first, second)
		value.Set([]primitive.ObjectID{second, third})
		require.Equal(t, []primitive.ObjectID{third}, value.Added())
		require.Equal(t, []primitive.ObjectID{first}, value.Deleted())
		require.Equal(t, []primitive.ObjectID{second}, value.Unchanged())
	})

	t.Run("struct", func(t *testing.T) {
		type pair struct {
			Key   string
			Value int
		}
		value := NewSlice(pair{"a", 1}, pair{"b", 2})
		value.Set([]pair{{"b", 2}, {"b", 3}})
		require.Equal(t, []pair{{"b", 3}}, value.Added())
		require.Equal(t, []pair{{"a", 1}}, value.Deleted())
	})

	t.Run("bool", func(t *testing.T) {
		value := NewSlice(true, true)
		value.Set([]bool{true, false})
		require.Equal(t, []bool{false}, value.Added())
		require.Equal(t, []bool{true}, value.Deleted())
	})
}

/******************************************
 * Schema Interfaces
 ******************************************/

// TestSlice_SetValue covers each class of input that converts into a list
func TestSlice_SetValue(t *testing.T) {

	check := func(name string, input any, want ...string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := NewSlice("baseline")
			require.NoError(t, value.SetValue(input))
			requireSlice(t, value, want, []string{"baseline"})
		})
	}

	strings := []string{"a", "b"}
	list := testList{"a", "b"}

	check("nil clears", nil)
	check("strings", []string{"a", "b"}, "a", "b")
	check("empty strings", []string{})
	check("nil strings", []string(nil))
	check("pointer to strings", &strings, "a", "b")
	check("anys", []any{"a", 1, true}, "a", "1", "true")
	check("ints", []int{1, 2}, "1", "2")
	check("lossy floats are kept", []float64{3.14159}, "3.14")
	check("array", [2]string{"a", "b"}, "a", "b")
	check("ArrayGetter", list, "a", "b")
	check("pointer to ArrayGetter", &list, "a", "b")
	check("the baseline itself", []string{"baseline"}, "baseline")
	check("duplicates are kept", []string{"a", "a"}, "a", "a")

	// A bare scalar becomes a list of one
	check("string", "a", "a")
	check("empty string", "", "")
	check("int", 42, "42")

	// Another Slice is read through its Length and GetIndex methods
	check("delta.Slice", NewSlice("a", "b"), "a", "b")
}

// TestSlice_SetValueInt verifies that values are converted into the element type
func TestSlice_SetValueInt(t *testing.T) {

	value := NewSlice(1)

	require.NoError(t, value.SetValue([]string{"2", "3"}))
	requireSlice(t, value, []int{2, 3}, []int{1})

	// A bare string is split on commas, for a list of ints only
	require.NoError(t, value.SetValue("4,5"))
	requireSlice(t, value, []int{4, 5}, []int{1})

	// A value that cannot be an int becomes the zero value, as rosetta converts it
	require.NoError(t, value.SetValue([]string{"not a number"}))
	requireSlice(t, value, []int{0}, []int{1})
}

// TestSlice_SetValueRefused verifies that a value which is not a list is an error,
// and leaves the Slice exactly as it was
func TestSlice_SetValueRefused(t *testing.T) {

	check := func(name string, input any) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedSlice()
			require.Error(t, value.SetValue(input))
			requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
		})
	}

	var nilPointer *[]string

	check("struct", struct{}{})
	check("map", map[string]any{"a": "b"})
	check("channel", make(chan int))
	check("func", func() {})
	check("nil pointer", nilPointer)
	check("int32", int32(1))
	check("float32", float32(1))
}

// TestSlice_SetValueDoesNotAlias verifies that the caller's list never becomes the Slice's own
func TestSlice_SetValueDoesNotAlias(t *testing.T) {

	values := []string{"a", "b"}
	value := NewSlice[string]()

	require.NoError(t, value.SetValue(values))
	values[0] = "changed"
	requireSlice(t, value, []string{"a", "b"}, nil)

	require.True(t, value.SetIndex(1, "z"))
	require.Equal(t, []string{"changed", "b"}, values)
}

// TestSlice_GetIndex covers every index in and out of range
func TestSlice_GetIndex(t *testing.T) {

	value := NewSlice("a", "b")

	check := func(index int, want any, wantOK bool) {
		t.Helper()
		result, ok := value.GetIndex(index)
		require.Equal(t, wantOK, ok, "index %d", index)
		require.Equal(t, want, result, "index %d", index)
	}

	check(0, "a", true)
	check(1, "b", true)
	check(2, nil, false)
	check(-1, nil, false)
	check(1<<40, nil, false)
	check(-1<<40, nil, false)

	var empty Slice[string]
	result, ok := empty.GetIndex(0)
	require.False(t, ok)
	require.Nil(t, result)
}

// TestSlice_SetIndex covers a write in range, a write that grows the list, and each refusal
func TestSlice_SetIndex(t *testing.T) {

	t.Run("in range", func(t *testing.T) {
		value := NewSlice("a", "b")
		require.True(t, value.SetIndex(1, "z"))
		requireSlice(t, value, []string{"a", "z"}, []string{"a", "b"})
		require.Equal(t, []string{"z"}, value.Added())
		require.Equal(t, []string{"b"}, value.Deleted())
	})

	t.Run("appends", func(t *testing.T) {
		value := NewSlice("a")
		require.True(t, value.SetIndex(1, "b"))
		requireSlice(t, value, []string{"a", "b"}, []string{"a"})
	})

	t.Run("grows with zero values", func(t *testing.T) {
		value := NewSlice("a")
		require.True(t, value.SetIndex(3, "d"))
		requireSlice(t, value, []string{"a", "", "", "d"}, []string{"a"})
	})

	t.Run("same value is not a change", func(t *testing.T) {
		value := NewSlice("a")
		require.True(t, value.SetIndex(0, "a"))
		require.False(t, value.IsChanged())
	})

	t.Run("negative index", func(t *testing.T) {
		value := changedSlice()
		require.False(t, value.SetIndex(-1, "z"))
		requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
	})

	t.Run("wrong type", func(t *testing.T) {
		value := changedSlice()
		require.False(t, value.SetIndex(0, 42))
		require.False(t, value.SetIndex(0, nil))
		require.False(t, value.SetIndex(9, []string{"a"}))
		requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
	})

	t.Run("int", func(t *testing.T) {
		value := NewSlice[int]()
		require.True(t, value.SetIndex(2, 7))
		require.False(t, value.SetIndex(0, int64(7)))
		requireSlice(t, value, []int{0, 0, 7}, nil)
	})
}

/******************************************
 * JSON Serialization
 ******************************************/

// TestSlice_MarshalJSON verifies that a Slice encodes as a JSON array, and never as null
func TestSlice_MarshalJSON(t *testing.T) {

	check := func(name string, value any, want string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			encoded, err := json.Marshal(value)
			require.NoError(t, err)
			require.JSONEq(t, want, string(encoded))
			require.Equal(t, want, string(encoded))
		})
	}

	var zero Slice[string]
	nilled := NewSlice("a")
	*nilled.Pointer() = nil

	check("strings", NewSlice("a", "b"), `["a","b"]`)
	check("ints", NewSlice(1, 2), `[1,2]`)
	check("empty", NewSlice[string](), `[]`)
	check("zero value", zero, `[]`)
	check("nil through the pointer", nilled, `[]`)
	check("current, not baseline", changedSlice(), `["b","c"]`)
	check("escaped", NewSlice(`"<>&`), `["\"\u003c\u003e\u0026"]`)
	check("pointer", &nilled, `[]`)
}

// TestSlice_UnmarshalJSON verifies that a decoded list becomes both the value and the baseline
func TestSlice_UnmarshalJSON(t *testing.T) {

	check := func(name string, input string, want ...string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedSlice()
			require.NoError(t, json.Unmarshal([]byte(input), &value))
			requireSlice(t, value, want, want)
			require.False(t, value.IsChanged())
		})
	}

	check("array", `["x","y"]`, "x", "y")
	check("empty array", `[]`)
	check("null is the empty list", `null`)
	check("whitespace", " [ \"x\" ] ", "x")
	check("duplicates", `["x","x"]`, "x", "x")
	check("null item is the zero value", `["x",null]`, "x", "")
	check("unicode", `["é","é"]`, "é", "é")
}

// TestSlice_UnmarshalJSONRefused verifies that a failed decode leaves the Slice exactly as it was
func TestSlice_UnmarshalJSONRefused(t *testing.T) {

	check := func(name string, input string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedSlice()
			require.Error(t, json.Unmarshal([]byte(input), &value))
			requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})

			// Called directly, without the encoding/json syntax check in front of it
			require.Error(t, value.UnmarshalJSON([]byte(input)))
			requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
		})
	}

	check("empty", ``)
	check("not json", `not json`)
	check("string", `"x"`)
	check("number", `42`)
	check("bool", `true`)
	check("object", `{"a":"b"}`)
	check("legacy object", `{"values":["x"]}`)
	check("wrong item type", `["x",42]`)
	check("wrong item type after valid items", `["x","y",{}]`)
	check("truncated", `["x",`)
	check("nested", `[["x"]]`)
}

// TestSlice_JSONStruct verifies a Slice inside a struct, with and without omitzero
func TestSlice_JSONStruct(t *testing.T) {

	type record struct {
		Always Slice[string] `json:"always"`
		Omit   Slice[string] `json:"omit,omitzero"`
	}

	t.Run("empty", func(t *testing.T) {
		encoded, err := json.Marshal(record{})
		require.NoError(t, err)
		require.Equal(t, `{"always":[]}`, string(encoded))
	})

	t.Run("round trip", func(t *testing.T) {
		original := record{Always: NewSlice("a"), Omit: NewSlice("b", "c")}
		encoded, err := json.Marshal(original)
		require.NoError(t, err)
		require.Equal(t, `{"always":["a"],"omit":["b","c"]}`, string(encoded))

		decoded := record{}
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		requireSlice(t, decoded.Always, []string{"a"}, []string{"a"})
		requireSlice(t, decoded.Omit, []string{"b", "c"}, []string{"b", "c"})
	})

	t.Run("missing property leaves the field alone", func(t *testing.T) {
		decoded := record{Always: changedSlice()}
		require.NoError(t, json.Unmarshal([]byte(`{}`), &decoded))
		requireSlice(t, decoded.Always, []string{"b", "c"}, []string{"a", "b"})
	})

	t.Run("a failed property leaves the field alone", func(t *testing.T) {
		decoded := record{Always: changedSlice()}
		require.Error(t, json.Unmarshal([]byte(`{"always":["x",1]}`), &decoded))
		requireSlice(t, decoded.Always, []string{"b", "c"}, []string{"a", "b"})
	})
}

/******************************************
 * BSON Serialization
 ******************************************/

// TestSlice_MarshalBSONValue verifies that a Slice encodes exactly as a plain list does,
// and never as a null
func TestSlice_MarshalBSONValue(t *testing.T) {

	check := func(name string, value Slice[string], plain []string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			wantType, wantData := marshalBSONValue(t, plain)

			gotType, gotData, err := value.MarshalBSONValue()
			require.NoError(t, err)
			require.Equal(t, bson.TypeArray, gotType)
			require.Equal(t, wantType, gotType)
			require.Equal(t, wantData, gotData)
		})
	}

	var zero Slice[string]

	check("strings", NewSlice("a", "b"), []string{"a", "b"})
	check("empty", NewSlice[string](), []string{})
	check("zero value", zero, []string{})
	check("current, not baseline", changedSlice(), []string{"b", "c"})
}

// TestSlice_UnmarshalBSONValue verifies each BSON value that decodes into a list
func TestSlice_UnmarshalBSONValue(t *testing.T) {

	check := func(name string, source any, want ...string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			valueType, data := marshalBSONValue(t, source)

			value := changedSlice()
			require.NoError(t, value.UnmarshalBSONValue(valueType, data))
			requireSlice(t, value, want, want)
			require.False(t, value.IsChanged())
		})
	}

	check("array", bson.A{"x", "y"}, "x", "y")
	check("empty array", bson.A{})
	check("null", primitive.Null{})
	check("undefined", primitive.Undefined{})
	check("duplicates", bson.A{"x", "x"}, "x", "x")
	check("plain list", []string{"x", "y"}, "x", "y")

	// LEGACY: the document that rosetta's delta.Slice stored
	check("legacy", bson.M{"values": bson.A{"x", "y"}}, "x", "y")
	check("legacy empty", bson.M{"values": bson.A{}})
	check("legacy null", bson.M{"values": nil})
	check("legacy struct", legacySlice[string]{Values: []string{"x", "y"}, Added: []string{"y"}}, "x", "y")
	check("legacy struct, nil values", legacySlice[string]{})
}

// TestSlice_UnmarshalBSONValueRefused verifies that every other BSON value is an error,
// and leaves the Slice exactly as it was
func TestSlice_UnmarshalBSONValueRefused(t *testing.T) {

	checkBytes := func(name string, valueType bsontype.Type, data []byte) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			value := changedSlice()
			require.Error(t, value.UnmarshalBSONValue(valueType, data))
			requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
		})
	}

	check := func(name string, source any) {
		t.Helper()
		valueType, data := marshalBSONValue(t, source)
		checkBytes(name, valueType, data)
	}

	check("string", "x")
	check("int", 42)
	check("bool", true)
	check("ObjectID", primitive.NewObjectID())
	check("binary", primitive.Binary{Data: []byte("x")})
	check("wrong item type", bson.A{"x", 42})
	check("wrong item type after valid items", bson.A{"x", "y", bson.M{}})
	check("nested array", bson.A{bson.A{"x"}})

	// LEGACY: only a document holding "values", and nothing else, is read
	check("empty document", bson.M{})
	check("document with another key", bson.M{"other": bson.A{"x"}})
	check("legacy with an extra key", bson.D{{Key: "values", Value: bson.A{"x"}}, {Key: "added", Value: bson.A{}}})
	check("legacy with an extra key first", bson.D{{Key: "added", Value: bson.A{}}, {Key: "values", Value: bson.A{"x"}}})
	check("legacy values is a string", bson.M{"values": "x"})
	check("legacy values is a document", bson.M{"values": bson.M{"values": bson.A{"x"}}})
	check("legacy wrong item type", bson.M{"values": bson.A{"x", 42}})
	check("legacy key differs by case", bson.M{"Values": bson.A{"x"}})

	// Malformed bytes, which the driver panics on if they reach it
	_, array := marshalBSONValue(t, bson.A{"x", "y"})
	_, document := marshalBSONValue(t, bson.M{"values": bson.A{"x"}})

	checkBytes("array, empty", bson.TypeArray, nil)
	checkBytes("array, four bytes", bson.TypeArray, []byte{4, 0, 0, 0})
	checkBytes("array, truncated", bson.TypeArray, array[:len(array)-1])
	checkBytes("array, trailing byte", bson.TypeArray, append(slices.Clone(array), 0))
	checkBytes("array, negative length", bson.TypeArray, []byte{0xff, 0xff, 0xff, 0xff, 0})
	checkBytes("array, no terminator", bson.TypeArray, []byte{5, 0, 0, 0, 1})
	checkBytes("document, empty", bson.TypeEmbeddedDocument, nil)
	checkBytes("document, truncated", bson.TypeEmbeddedDocument, document[:len(document)-1])
	checkBytes("document, trailing byte", bson.TypeEmbeddedDocument, append(slices.Clone(document), 0))
	checkBytes("document, negative length", bson.TypeEmbeddedDocument, []byte{0xff, 0xff, 0xff, 0xff, 0})
	checkBytes("document, no terminator", bson.TypeEmbeddedDocument, []byte{5, 0, 0, 0, 1})

	// The right length and terminator, around an item whose own length is wrong
	badItem := slices.Clone(array)
	require.Greater(t, len(badItem), 7)
	badItem[7] = 0x7f
	checkBytes("array, item length too long", bson.TypeArray, badItem)

	badType := slices.Clone(array)
	require.Greater(t, len(badType), 4)
	badType[4] = 0x7f
	checkBytes("array, unknown item type", bson.TypeArray, badType)

	legacyNoTerminator := slices.Clone(document)
	require.NotEmpty(t, legacyNoTerminator)
	legacyNoTerminator[len(legacyNoTerminator)-1] = '0'
	checkBytes("document, terminator is not null", bson.TypeEmbeddedDocument, legacyNoTerminator)

	checkBytes("unknown type", bsontype.Type(0x7f), array)
}

// TestSlice_UnmarshalBSONLegacyInnerErrors corrupts the inside of a legacy document, where
// the outer length is still correct
func TestSlice_UnmarshalBSONLegacyInnerErrors(t *testing.T) {

	_, document := marshalBSONValue(t, bson.M{"values": bson.A{"x"}})

	// The inner array's length header follows the element type and the key "values\0"
	const innerLength = 4 + 1 + len("values") + 1

	check := func(name string, corrupt func(data []byte)) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			data := slices.Clone(document)
			corrupt(data)

			value := changedSlice()
			require.Error(t, value.UnmarshalBSONValue(bson.TypeEmbeddedDocument, data))
			requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
		})
	}

	check("inner length too long", func(data []byte) { data[innerLength] = 0x7f })
	check("inner length too short", func(data []byte) { data[innerLength] = 5 })
	check("inner length negative", func(data []byte) { data[innerLength+3] = 0xff })
	check("key has no terminator", func(data []byte) {
		for index := 5; index < len(data); index++ {
			data[index] = 'a'
		}
	})
	check("unknown element type", func(data []byte) { data[4] = 0x7f })
}

// TestSlice_BSONDocument verifies a Slice inside a document, with and without omitempty
func TestSlice_BSONDocument(t *testing.T) {

	type record struct {
		Always Slice[string] `bson:"always"`
		Omit   Slice[string] `bson:"omit,omitempty"`
	}

	t.Run("stored as a plain array", func(t *testing.T) {
		encoded, err := bson.Marshal(record{Always: NewSlice("a", "b"), Omit: NewSlice("c")})
		require.NoError(t, err)

		plain := bson.M{}
		require.NoError(t, bson.Unmarshal(encoded, &plain))
		require.Equal(t, bson.M{"always": bson.A{"a", "b"}, "omit": bson.A{"c"}}, plain)
	})

	t.Run("empty is an empty array, or omitted", func(t *testing.T) {
		encoded, err := bson.Marshal(record{})
		require.NoError(t, err)

		plain := bson.M{}
		require.NoError(t, bson.Unmarshal(encoded, &plain))
		require.Equal(t, bson.M{"always": bson.A{}}, plain)
	})

	t.Run("round trip", func(t *testing.T) {
		original := record{Always: changedSlice(), Omit: NewSlice("c")}
		encoded, err := bson.Marshal(original)
		require.NoError(t, err)

		decoded := record{}
		require.NoError(t, bson.Unmarshal(encoded, &decoded))
		requireSlice(t, decoded.Always, []string{"b", "c"}, []string{"b", "c"})
		requireSlice(t, decoded.Omit, []string{"c"}, []string{"c"})
	})

	t.Run("missing property is empty and unchanged", func(t *testing.T) {
		encoded, err := bson.Marshal(bson.M{})
		require.NoError(t, err)

		decoded := record{}
		require.NoError(t, bson.Unmarshal(encoded, &decoded))
		requireSlice(t, decoded.Always, nil, nil)
		require.False(t, decoded.Always.IsChanged())
	})

	t.Run("null property is empty and unchanged", func(t *testing.T) {
		encoded, err := bson.Marshal(bson.M{"always": nil})
		require.NoError(t, err)

		decoded := record{}
		require.NoError(t, bson.Unmarshal(encoded, &decoded))
		requireSlice(t, decoded.Always, nil, nil)
	})

	t.Run("wrong type is an error", func(t *testing.T) {
		encoded, err := bson.Marshal(bson.M{"always": "x"})
		require.NoError(t, err)
		require.Error(t, bson.Unmarshal(encoded, &record{}))
	})

	t.Run("ObjectIDs", func(t *testing.T) {
		type owners struct {
			IDs Slice[primitive.ObjectID] `bson:"ids"`
		}
		first, second := primitive.NewObjectID(), primitive.NewObjectID()

		encoded, err := bson.Marshal(owners{IDs: NewSlice(first, second)})
		require.NoError(t, err)

		decoded := owners{}
		require.NoError(t, bson.Unmarshal(encoded, &decoded))
		requireSlice(t, decoded.IDs, []primitive.ObjectID{first, second}, []primitive.ObjectID{first, second})
	})

	t.Run("ints", func(t *testing.T) {
		type counts struct {
			Values Slice[int] `bson:"values"`
		}

		encoded, err := bson.Marshal(counts{Values: NewSlice(1, 2)})
		require.NoError(t, err)

		decoded := counts{}
		require.NoError(t, bson.Unmarshal(encoded, &decoded))
		requireSlice(t, decoded.Values, []int{1, 2}, []int{1, 2})
	})
}

// TestSlice_BSONLegacyDocument is the migration path: a record that rosetta's delta.Slice
// wrote is read as it stands, and written back as a plain array
func TestSlice_BSONLegacyDocument(t *testing.T) {

	type legacyRecord struct {
		Syndication legacySlice[string] `bson:"syndication,omitempty"`
	}

	type record struct {
		Syndication Slice[string] `bson:"syndication,omitempty"`
	}

	check := func(name string, legacy legacySlice[string], want ...string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {

			// Written by the old type...
			stored, err := bson.Marshal(legacyRecord{Syndication: legacy})
			require.NoError(t, err)

			// ...read by the new one, as an unchanged value...
			decoded := record{}
			require.NoError(t, bson.Unmarshal(stored, &decoded))
			requireSlice(t, decoded.Syndication, want, want)
			require.False(t, decoded.Syndication.IsChanged())

			// ...and written back as a plain array, or omitted when empty.
			healed, err := bson.Marshal(decoded)
			require.NoError(t, err)

			plain := bson.M{}
			require.NoError(t, bson.Unmarshal(healed, &plain))

			if len(want) == 0 {
				require.Equal(t, bson.M{}, plain)
				return
			}

			wantArray := bson.A{}
			for _, item := range want {
				wantArray = append(wantArray, item)
			}
			require.Equal(t, bson.M{"syndication": wantArray}, plain)

			// The healed record reads back the same
			reloaded := record{}
			require.NoError(t, bson.Unmarshal(healed, &reloaded))
			requireSlice(t, reloaded.Syndication, want, want)
		})
	}

	check("values", legacySlice[string]{Values: []string{"mastodon", "bluesky"}}, "mastodon", "bluesky")
	check("one value", legacySlice[string]{Values: []string{"mastodon"}}, "mastodon")
	check("empty values", legacySlice[string]{Values: []string{}})
	check("nil values", legacySlice[string]{})
	check("change lists were never stored", legacySlice[string]{Values: []string{"a"}, Added: []string{"a"}, Deleted: []string{"b"}}, "a")
}

/******************************************
 * Helpers
 ******************************************/

// TestIsSizedBSON covers the length check that stands in front of the driver
func TestIsSizedBSON(t *testing.T) {

	require.True(t, isSizedBSON([]byte{5, 0, 0, 0, 0}))
	require.True(t, isSizedBSON([]byte{6, 0, 0, 0, 1, 0}))

	require.False(t, isSizedBSON(nil))
	require.False(t, isSizedBSON([]byte{}))
	require.False(t, isSizedBSON([]byte{4, 0, 0, 0}))
	require.False(t, isSizedBSON([]byte{6, 0, 0, 0, 0}))
	require.False(t, isSizedBSON([]byte{4, 0, 0, 0, 0}))
	require.False(t, isSizedBSON([]byte{0xff, 0xff, 0xff, 0xff, 0}))
	require.False(t, isSizedBSON([]byte{5, 0, 0, 0x80, 0}))

	// A missing terminator
	require.False(t, isSizedBSON([]byte{5, 0, 0, 0, 1}))
	require.False(t, isSizedBSON([]byte{6, 0, 0, 0, 0, 1}))
}

// TestCloneSlice verifies that a clone is never nil, and never shares memory
func TestCloneSlice(t *testing.T) {

	require.Equal(t, []string{}, cloneSlice[string](nil))
	require.Equal(t, []string{}, cloneSlice([]string{}))

	original := []string{"a", "b"}
	clone := cloneSlice(original)
	require.Len(t, clone, 2)
	clone[0] = "changed"
	require.Equal(t, []string{"a", "b"}, original)

	// A slice with spare capacity is not written through to
	spare := make([]string, 1, 4)
	clone = append(cloneSlice(spare), "x")
	require.Equal(t, []string{"", "x"}, clone)
	require.Equal(t, []string{""}, spare)
}

// TestMatchSlice checks the pairing against its slower oracle, for every pair of short lists
func TestMatchSlice(t *testing.T) {

	alphabet := []string{"a", "b", ""}
	lists := [][]string{nil}

	// Every list of up to three values from the alphabet
	for range 3 {
		for _, list := range slices.Clone(lists) {
			for _, letter := range alphabet {
				lists = append(lists, append(slices.Clone(list), letter))
			}
		}
	}

	for _, first := range lists {
		for _, second := range lists {
			wantMatched, wantUnmatched := naiveMatch(first, second)
			matched, unmatched := matchSlice(first, second)
			require.Equal(t, wantMatched, matched, "%q against %q", first, second)
			require.Equal(t, wantUnmatched, unmatched, "%q against %q", first, second)
		}
	}
}

/******************************************
 * Fuzz Tests
 ******************************************/

// fuzzList turns fuzz bytes into a short list of small numbers, so that duplicates are common
func fuzzList(data []byte) []int {

	result := make([]int, 0, len(data))

	for _, item := range data {
		result = append(result, int(item%8))
	}

	return result
}

// fuzzStrings splits a fuzz string into a list. An empty input is the empty list.
func fuzzStrings(data string) []string {

	if data == "" {
		return []string{}
	}

	return strings.Split(data, ",")
}

// FuzzSlice_Operations drives a Slice through a random series of operations, and checks it
// against a model made of two plain lists after every one.
func FuzzSlice_Operations(f *testing.F) {

	f.Add([]byte{}, []byte{})
	f.Add([]byte{1, 2, 3}, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{1, 1, 2}, []byte{4, 250, 5, 3, 0, 9, 1, 2})
	f.Add([]byte{7}, []byte{6, 6, 6, 2, 7, 8, 3})

	f.Fuzz(func(t *testing.T, initial []byte, operations []byte) {

		current := fuzzList(initial)
		original := fuzzList(initial)
		value := NewSlice(current...)

		requireSlice(t, value, current, original)

		for index, operation := range operations {

			// The bytes that follow an operation are its argument
			argument := fuzzList(operations[index+1:])
			if len(argument) > 6 {
				argument = argument[:6]
			}

			switch operation % 10 {

			case 0:
				value.Set(argument)
				current = cloneSlice(argument)

			case 1:
				value.Clear()
				current = []int{}

			case 2:
				value.Reset()
				original = cloneSlice(current)

			case 3:
				require.NoError(t, value.SetValue(argument))
				current = cloneSlice(argument)

			case 4:
				position := len(argument)
				require.True(t, value.SetIndex(position, index))
				for position >= len(current) {
					current = append(current, 0)
				}
				current[position] = index

			case 5:
				pointer := value.Pointer()
				*pointer = append(*pointer, index)
				current = append(current, index)

			case 6:
				encoded, err := json.Marshal(value)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(encoded, &value))
				original = cloneSlice(current)

			case 7:
				valueType, encoded, err := value.MarshalBSONValue()
				require.NoError(t, err)
				require.NoError(t, value.UnmarshalBSONValue(valueType, encoded))
				original = cloneSlice(current)

			case 8:
				require.Error(t, value.SetValue(struct{}{}))
				require.False(t, value.SetIndex(-1-index, 0))
				require.False(t, value.SetIndex(0, "wrong type"))

			case 9:
				require.NoError(t, value.SetValue(nil))
				current = []int{}
			}

			requireSlice(t, value, current, original)
		}
	})
}

// FuzzSlice_ChangeLists checks the change lists of two arbitrary lists against their invariants
func FuzzSlice_ChangeLists(f *testing.F) {

	f.Add([]byte{}, []byte{})
	f.Add([]byte{1, 2, 3}, []byte{3, 2, 1})
	f.Add([]byte{1, 1, 1}, []byte{1})
	f.Add([]byte{1}, []byte{2, 2})

	f.Fuzz(func(t *testing.T, first []byte, second []byte) {

		original := fuzzList(first)
		current := fuzzList(second)

		value := NewSlice(original...)
		value.Set(current)

		requireSlice(t, value, current, original)

		added, deleted, unchanged := value.Added(), value.Deleted(), value.Unchanged()

		// Every current value is either added or unchanged, and every baseline value
		// is either deleted or unchanged
		require.Len(t, current, len(added)+len(unchanged))
		require.Len(t, original, len(deleted)+len(unchanged))

		// No value is both added and deleted
		for _, item := range added {
			require.NotContains(t, deleted, item)
		}

		// Applying the change lists to the baseline gives the current values, in some order
		rebuilt := append(cloneSlice(unchanged), added...)
		slices.Sort(rebuilt)
		sorted := cloneSlice(current)
		slices.Sort(sorted)
		require.Equal(t, sorted, rebuilt)

		// Nothing changed means nothing was added or deleted, but not the reverse
		if value.NotChanged() {
			require.Empty(t, added)
			require.Empty(t, deleted)
		}

		// Swapping the two lists swaps added and deleted
		reverse := NewSlice(current...)
		reverse.Set(original)
		require.ElementsMatch(t, added, reverse.Deleted())
		require.ElementsMatch(t, deleted, reverse.Added())
	})
}

// FuzzSlice_SetValue checks that SetValue never panics, and that a refused value changes nothing
func FuzzSlice_SetValue(f *testing.F) {

	f.Add("a,b", int64(0), 0.0, uint8(0))
	f.Add("", int64(42), 3.14159, uint8(1))
	f.Add("a,a,", int64(-1), -1.5, uint8(2))
	f.Add("\xff", int64(1)<<62, 1e300, uint8(7))

	f.Fuzz(func(t *testing.T, text string, number int64, fraction float64, selector uint8) {

		list := fuzzStrings(text)
		asList := testList(list)

		inputs := []any{
			list, &list, asList, &asList, text, number, fraction, nil,
			[]any{text, number, fraction, nil, true},
			[]int64{number}, []float64{fraction},
			struct{}{}, map[string]any{text: number}, int32(number), &text,
		}

		input := inputs[int(selector)%len(inputs)]

		{ // A list of strings
			value := changedSlice()
			err := value.SetValue(input)

			if err != nil {
				requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
			} else {
				require.Equal(t, []string{"a", "b"}, value.Original())
				require.NotNil(t, value.Value())
			}

			// A list of strings is stored exactly as it was given
			if int(selector)%len(inputs) < 4 {
				require.NoError(t, err)
				requireSlice(t, value, list, []string{"a", "b"})
			}
		}

		{ // A list of ints refuses exactly the same inputs
			strings := changedSlice()
			stringsErr := strings.SetValue(input)

			numbers := NewSlice(1)
			numbersErr := numbers.SetValue(input)

			// The lengths can differ: rosetta splits a string on commas for a list of ints only
			require.Equal(t, stringsErr == nil, numbersErr == nil)
			require.Equal(t, []int{1}, numbers.Original())
		}
	})
}

// FuzzSlice_RoundTrip checks that a list survives JSON and BSON, and reads back unchanged
func FuzzSlice_RoundTrip(f *testing.F) {

	f.Add("")
	f.Add("a")
	f.Add("a,b,c")
	f.Add(",,")
	f.Add("a,a")
	f.Add("\x00,\"quoted\",<html>")

	f.Fuzz(func(t *testing.T, text string) {

		list := fuzzStrings(text)
		value := NewSlice("baseline")
		value.Set(list)

		{ // BSON carries any string, valid UTF-8 or not, exactly
			valueType, encoded, err := value.MarshalBSONValue()
			require.NoError(t, err)

			decoded := changedSlice()
			err = decoded.UnmarshalBSONValue(valueType, encoded)

			// RULE: The driver refuses a string that holds a zero byte... only as a key, so
			// every list here must decode.
			require.NoError(t, err)
			requireSlice(t, decoded, list, list)

			// The legacy document that held the same list decodes to the same value
			legacyType, legacy := marshalBSONValue(t, legacySlice[string]{Values: list})
			fromLegacy := changedSlice()
			require.NoError(t, fromLegacy.UnmarshalBSONValue(legacyType, legacy))
			requireSlice(t, fromLegacy, list, list)

			// ...and encodes to the same bytes
			_, healed, err := fromLegacy.MarshalBSONValue()
			require.NoError(t, err)
			require.Equal(t, encoded, healed)
		}

		{ // JSON encodes as the plain list does, and round-trips valid UTF-8 exactly
			encoded, err := json.Marshal(value)
			require.NoError(t, err)

			plain, err := json.Marshal(list)
			require.NoError(t, err)
			require.Equal(t, string(plain), string(encoded))

			decoded := changedSlice()
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.False(t, decoded.IsChanged())

			if utf8.ValidString(text) {
				requireSlice(t, decoded, list, list)
			}
		}
	})
}

// FuzzSlice_UnmarshalJSON checks arbitrary JSON against a plain list decoded from the same bytes
func FuzzSlice_UnmarshalJSON(f *testing.F) {

	seeds := []string{
		`["a","b"]`, `[]`, `null`, ``, `not json`, `"a"`, `42`, `{"values":["a"]}`,
		`["a",1]`, `[null]`, `[["a"]]`, `["a",`, ` [ "a" ] `, `["\ud800"]`, `[1,2]`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		var plain []string
		plainErr := json.Unmarshal(data, &plain)

		value := changedSlice()
		err := json.Unmarshal(data, &value)

		require.Equal(t, plainErr == nil, err == nil, "plain: %v, wrapped: %v", plainErr, err)

		if err != nil {
			requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
			return
		}

		requireSlice(t, value, plain, plain)

		// Called directly, the method never panics, and never half-writes
		if direct := changedSlice(); direct.UnmarshalJSON(data) != nil {
			requireSlice(t, direct, []string{"b", "c"}, []string{"a", "b"})
		}

		// A list of ints accepts exactly what a plain list of ints does
		var plainInts []int
		numbers := NewSlice(1)
		require.Equal(t, json.Unmarshal(data, &plainInts) == nil, json.Unmarshal(data, &numbers) == nil)
		require.True(t, numbers.NotChanged())
	})
}

// FuzzSlice_UnmarshalJSONDocument checks a Slice inside a struct against a plain list in its place
func FuzzSlice_UnmarshalJSONDocument(f *testing.F) {

	seeds := []string{
		`{"value":["a","b"]}`, `{"value":[]}`, `{"value":null}`, `{}`, `{"value":"a"}`,
		`{"value":{"values":["a"]}}`, `{"value":["a",1]}`, `{"other":["a"]}`, `[]`, `null`,
		`{"value":["a"],"value":["b"]}`, `{"VALUE":["a"]}`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {

		plain := struct {
			Value *[]string `json:"value"`
		}{}
		plainErr := json.Unmarshal(data, &plain)

		wrapped := struct {
			Value Slice[string] `json:"value"`
		}{}
		wrappedErr := json.Unmarshal(data, &wrapped)

		require.Equal(t, plainErr == nil, wrappedErr == nil, "plain: %v, wrapped: %v", plainErr, wrappedErr)

		// Baseline and current value always move together during a load
		require.True(t, wrapped.Value.NotChanged())

		if plainErr != nil {
			return
		}

		// A missing property and a null are both the empty list
		if plain.Value == nil {
			requireSlice(t, wrapped.Value, nil, nil)
			return
		}

		requireSlice(t, wrapped.Value, *plain.Value, *plain.Value)
	})
}

// FuzzSlice_UnmarshalBSONValue checks that arbitrary bytes of any BSON type never panic,
// never half-write, and decode as a plain list does
func FuzzSlice_UnmarshalBSONValue(f *testing.F) {

	sources := []any{
		bson.A{"a", "b"}, bson.A{}, primitive.Null{}, "a", 42, true, bson.A{"a", 1}, bson.A{bson.A{"a"}},
		bson.M{"values": bson.A{"a"}}, bson.M{"values": nil}, bson.M{"values": "a"}, bson.M{},
		bson.M{"other": bson.A{"a"}}, bson.D{{Key: "values", Value: bson.A{}}, {Key: "x", Value: 1}},
		primitive.NewObjectID(), primitive.Undefined{}, primitive.Symbol("a"),
		bson.M{"values": bson.A{primitive.Symbol("a")}}, bson.A{primitive.Symbol("a")},
	}
	for _, source := range sources {
		valueType, data := marshalBSONValue(f, source)
		f.Add(byte(valueType), data)
	}

	f.Add(byte(bson.TypeArray), []byte{})
	f.Add(byte(bson.TypeArray), []byte{0xff, 0xff, 0xff, 0xff, 0})
	f.Add(byte(bson.TypeEmbeddedDocument), []byte{5, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, typeByte byte, data []byte) {

		valueType := bsontype.Type(typeByte)

		value := changedSlice()
		err := value.UnmarshalBSONValue(valueType, data)

		if err != nil {
			requireSlice(t, value, []string{"b", "c"}, []string{"a", "b"})
		} else {
			require.True(t, value.NotChanged())
		}

		switch valueType {

		// A null is the empty list, whatever bytes follow it
		case bson.TypeNull, bson.TypeUndefined:
			require.NoError(t, err)
			requireSlice(t, value, nil, nil)

		// A well-formed array decodes exactly as a plain list does
		case bson.TypeArray:
			if !isWellFormedBSON(data) {
				require.Error(t, err)
				return
			}

			var plain []string
			plainErr := bson.UnmarshalValue(valueType, data, &plain)
			require.Equal(t, plainErr == nil, err == nil, "plain: %v, wrapped: %v", plainErr, err)

			if err == nil {
				requireSlice(t, value, plain, plain)
			}

		// A well-formed legacy document decodes exactly as the legacy struct does
		case bson.TypeEmbeddedDocument:
			if err != nil {
				return
			}

			require.True(t, isWellFormedBSON(data))

			var legacy legacySlice[string]
			require.NoError(t, bson.UnmarshalValue(valueType, data, &legacy))
			requireSlice(t, value, legacy.Values, legacy.Values)

			elements, elementsErr := bson.Raw(data).Elements()
			require.NoError(t, elementsErr)
			require.Len(t, elements, 1)
			require.Equal(t, "values", elements[0].Key())

		// Every other type is refused
		default:
			require.Error(t, err)
		}
	})
}

// FuzzSlice_UnmarshalBSONDocument checks a Slice inside a document against a plain list,
// and against the legacy struct, in its place
func FuzzSlice_UnmarshalBSONDocument(f *testing.F) {

	sources := []bson.M{
		{"value": bson.A{"a", "b"}}, {"value": bson.A{}}, {"value": nil}, {}, {"other": bson.A{"a"}},
		{"value": "a"}, {"value": 12}, {"value": bson.A{"a", 1}}, {"value": bson.M{"values": bson.A{"a"}}},
		{"value": bson.M{"values": nil}}, {"value": bson.M{}}, {"value": bson.M{"a": "b"}},
		{"value": primitive.Undefined{}}, {"value": bson.A{primitive.Symbol("a")}},
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

		// RULE: The driver matches a key to a field without regard to case, and decodes
		// every match in turn, so the test counts the matches as the driver would
		elements, err := bson.Raw(data).Elements()
		require.NoError(t, err)

		matches, documents := 0, 0
		for _, element := range elements {
			if strings.EqualFold(element.Key(), "value") {
				matches++
				if element.Value().Type == bson.TypeEmbeddedDocument {
					documents++
				}
			}
		}

		wrapped := struct {
			Value Slice[string] `bson:"value"`
		}{}

		switch wrappedErr := bson.Unmarshal(data, &wrapped); {

		// No document is involved, so the Slice is read exactly as a plain list is
		case documents == 0:
			plain := struct {
				Value []string `bson:"value"`
			}{}
			plainErr := bson.Unmarshal(data, &plain)

			require.Equal(t, plainErr == nil, wrappedErr == nil, "plain: %v, wrapped: %v", plainErr, wrappedErr)

			if plainErr == nil {
				requireSlice(t, wrapped.Value, plain.Value, plain.Value)
			}

		// One document, which is read as the legacy struct when it is one
		case (matches == 1) && (wrappedErr == nil):
			legacy := struct {
				Value legacySlice[string] `bson:"value"`
			}{}
			require.NoError(t, bson.Unmarshal(data, &legacy))
			requireSlice(t, wrapped.Value, legacy.Value.Values, legacy.Value.Values)
		}

		// A repeated key that mixes arrays and documents has no oracle; it must only not panic.
		// Baseline and current value always move together during a load.
		require.True(t, wrapped.Value.NotChanged())
	})
}

/******************************************
 * Examples
 ******************************************/

// ExampleSlice shows a change being tracked against the list that was loaded
func ExampleSlice() {

	targets := NewSlice("mastodon", "bluesky")
	fmt.Println(targets.IsChanged())

	targets.Set([]string{"bluesky", "threads"})
	fmt.Println(targets.IsChanged())
	fmt.Println(targets.Added())
	fmt.Println(targets.Deleted())
	fmt.Println(targets.Unchanged())
	fmt.Println(targets.Original())

	// Output:
	// false
	// true
	// [threads]
	// [mastodon]
	// [bluesky]
	// [mastodon bluesky]
}

// ExampleSlice_Reset shows the baseline moving once the caller has saved the list
func ExampleSlice_Reset() {

	targets := NewSlice("mastodon")
	targets.Set([]string{"mastodon", "bluesky"})

	targets.Reset()
	fmt.Println(targets.IsChanged())
	fmt.Println(targets.Added())
	fmt.Println(targets.Original())

	// Output:
	// false
	// []
	// [mastodon bluesky]
}

// ExampleSlice_UnmarshalBSONValue shows a list that an older version stored inside a
// document being read, and written back as a plain array
func ExampleSlice_UnmarshalBSONValue() {

	stored, _ := bson.Marshal(bson.M{"targets": bson.M{"values": bson.A{"mastodon", "bluesky"}}})

	record := struct {
		Targets Slice[string] `bson:"targets"`
	}{}

	if err := bson.Unmarshal(stored, &record); err != nil {
		fmt.Println(err)
		return
	}

	saved, _ := bson.Marshal(record)

	fmt.Println(record.Targets.Value())
	fmt.Println(record.Targets.IsChanged())
	fmt.Println(bson.Raw(saved))

	// Output:
	// [mastodon bluesky]
	// false
	// {"targets": ["mastodon","bluesky"]}
}
