package delta

import (
	"encoding/binary"
	"encoding/json"
	"slices"

	"github.com/benpate/derp"
	"github.com/benpate/rosetta/convert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// legacySliceKey is the one property of the document that rosetta's delta.Slice stored
const legacySliceKey = "values"

// Slice tracks changes to a list of values since it has been loaded into memory
type Slice[T comparable] struct {
	current  []T
	original []T
}

// NewSlice returns a fully initialized Slice object, holding a copy of the values
func NewSlice[T comparable](values ...T) Slice[T] {

	return Slice[T]{
		current:  cloneSlice(values),
		original: cloneSlice(values),
	}
}

/******************************************
 * Data Manipulation Methods
 ******************************************/

// Set updates the current value to a copy of the provided list
func (v *Slice[T]) Set(values []T) {
	v.current = cloneSlice(values)
}

// Clear sets the current value to the empty list, leaving the baseline as it was
func (v *Slice[T]) Clear() {
	v.current = make([]T, 0)
}

// Value returns a copy of the current list, which is never nil
func (v Slice[T]) Value() []T {
	return cloneSlice(v.current)
}

// Pointer returns a pointer to the current list, for a caller that edits it in place
func (v *Slice[T]) Pointer() *[]T {
	return &v.current
}

// Original returns a copy of the list as it was when this Slice was loaded
func (v Slice[T]) Original() []T {
	return cloneSlice(v.original)
}

// Reset makes the current value the baseline, for a caller that has just saved it
func (v *Slice[T]) Reset() {
	v.original = cloneSlice(v.current)
}

// NotChanged returns TRUE if the list has not changed since it was loaded
func (v Slice[T]) NotChanged() bool {
	return slices.Equal(v.current, v.original)
}

// IsChanged returns TRUE if the list has changed since it was loaded.
// A change of order alone is a change.
func (v Slice[T]) IsChanged() bool {
	return !slices.Equal(v.current, v.original)
}

// IsZero returns TRUE if the current list is empty
func (v Slice[T]) IsZero() bool {
	return len(v.current) == 0
}

// Length returns the number of values in the current list
func (v Slice[T]) Length() int {
	return len(v.current)
}

/******************************************
 * Change Lists
 ******************************************/

// Added returns the current values that the baseline does not hold, in their current order
func (v Slice[T]) Added() []T {
	_, added := matchSlice(v.current, v.original)
	return added
}

// Deleted returns the baseline values that the current list does not hold, in their original order
func (v Slice[T]) Deleted() []T {
	_, deleted := matchSlice(v.original, v.current)
	return deleted
}

// Unchanged returns the current values that the baseline also holds, in their current order
func (v Slice[T]) Unchanged() []T {
	unchanged, _ := matchSlice(v.current, v.original)
	return unchanged
}

/******************************************
 * Schema Interfaces
 ******************************************/

// GetValue implements schema.ValueGetter and returns a copy of the current list
func (v Slice[T]) GetValue() any {
	return cloneSlice(v.current)
}

// SetValue implements schema.ValueSetter and updates the current list.
// It converts loosely, and returns an error for a value that is not a list.
func (v *Slice[T]) SetValue(value any) error {

	const location = "delta.Slice.SetValue"

	// RULE: A nil value clears the list, because it means "nothing is selected any more"
	if value == nil {
		v.current = make([]T, 0)
		return nil
	}

	// RULE: A value that is not a list is refused before any state is touched
	if _, isList := convert.SliceOfAnyOk(value); !isList {
		return derp.BadRequest(location, "Unable to convert value into a slice", value)
	}

	// A lossy conversion is kept, and the result is copied so that it never aliases the caller's list
	v.current = cloneSlice(convert.SliceOf[T](value))
	return nil
}

// GetIndex implements schema.ArrayGetter and returns the value at the index,
// or (nil, false) if the index is out of range.
func (v Slice[T]) GetIndex(index int) (any, bool) {

	if (index < 0) || (index >= len(v.current)) {
		return nil, false
	}

	return v.current[index], true
}

// SetIndex implements schema.ArraySetter and stores a value at the index, growing the list
// to fit. It returns FALSE for a negative index, or a value that is not a T.
func (v *Slice[T]) SetIndex(index int, value any) bool {

	typed, isCorrectType := value.(T)

	if !isCorrectType {
		return false
	}

	if index < 0 {
		return false
	}

	// RULE: Write to a copy, because a Slice that was copied by assignment shares this list
	result := cloneSlice(v.current)

	// Grow the list to fit, filling the gap with zero values
	var zero T
	for index >= len(result) {
		result = append(result, zero)
	}

	result[index] = typed
	v.current = result
	return true
}

/******************************************
 * JSON Serialization
 ******************************************/

// MarshalJSON implements the json.Marshaler interface
// and serializes the Slice into a JSON array, which is never null
func (v Slice[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(cloneSlice(v.current))
}

// UnmarshalJSON implements the json.Unmarshaler interface
// and deserializes the Slice from a JSON array
func (v *Slice[T]) UnmarshalJSON(data []byte) error {

	// Decode into a local value, so that a failed decode leaves this Slice as it was
	var decoded []T
	if err := json.Unmarshal(data, &decoded); err != nil {
		return derp.Wrap(err, "delta.Slice.UnmarshalJSON", "Unable to unmarshal JSON data", data)
	}

	// A JSON null becomes the empty list
	v.original = cloneSlice(decoded)
	v.current = cloneSlice(decoded)
	return nil
}

/******************************************
 * BSON Serialization
 ******************************************/

// MarshalBSONValue implements the bson.ValueMarshaler interface
// and serializes the Slice into a BSON array, which is never null
func (v Slice[T]) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.MarshalValue(cloneSlice(v.current))
}

// UnmarshalBSONValue implements the bson.ValueUnmarshaler interface. It reads a BSON
// array, a null, or the {values: [...]} document that rosetta's delta.Slice stored.
func (v *Slice[T]) UnmarshalBSONValue(t bsontype.Type, data []byte) error {

	const location = "delta.Slice.UnmarshalBSONValue"

	decoded, err := decodeBSONSlice[T](t, data)

	if err != nil {
		return derp.Wrap(err, location, "Unable to unmarshal BSON data", t, data)
	}

	v.original = cloneSlice(decoded)
	v.current = cloneSlice(decoded)
	return nil
}

// decodeBSONSlice reads a list from a BSON array, a null, or a legacy {values: [...]} document
func decodeBSONSlice[T comparable](t bsontype.Type, data []byte) ([]T, error) {

	const location = "delta.decodeBSONSlice"

	switch t {

	// A null, like a missing property, is the empty list
	case bson.TypeNull, bson.TypeUndefined:
		return nil, nil

	case bson.TypeArray:
		return decodeBSONArray[T](data)

	// LEGACY: rosetta's delta.Slice stored its list inside a document
	case bson.TypeEmbeddedDocument:
		return decodeBSONLegacy[T](data)
	}

	return nil, derp.BadRequest(location, "Slice must be a BSON array", t.String())
}

// decodeBSONArray reads a list from the bytes of a BSON array
func decodeBSONArray[T comparable](data []byte) ([]T, error) {

	const location = "delta.decodeBSONArray"

	// RULE: The length is checked here, because the driver panics on one that is wrong
	if !isSizedBSON(data) {
		return nil, derp.BadRequest(location, "BSON array length or terminator does not match its data")
	}

	if err := bson.Raw(data).Validate(); err != nil {
		return nil, derp.Wrap(err, location, "BSON array is malformed")
	}

	var decoded []T
	if err := bson.UnmarshalValue(bson.TypeArray, data, &decoded); err != nil {
		return nil, derp.Wrap(err, location, "Unable to decode BSON array")
	}

	return decoded, nil
}

// decodeBSONLegacy reads a list from the {values: [...]} document that rosetta's
// delta.Slice stored. A document of any other shape is an error.
func decodeBSONLegacy[T comparable](data []byte) ([]T, error) {

	const location = "delta.decodeBSONLegacy"

	// RULE: The length is checked here, because the driver panics on one that is wrong
	if !isSizedBSON(data) {
		return nil, derp.BadRequest(location, "BSON document length or terminator does not match its data")
	}

	elements, err := bson.Raw(data).Elements()

	if err != nil {
		return nil, derp.Wrap(err, location, "BSON document is malformed")
	}

	// RULE: The legacy document holds "values" and nothing else
	if len(elements) != 1 {
		return nil, derp.BadRequest(location, "Legacy Slice document must hold exactly one property")
	}

	// Elements() has already validated the key and the value, so neither read can fail
	if elements[0].Key() != legacySliceKey {
		return nil, derp.BadRequest(location, "Legacy Slice document must hold a 'values' property")
	}

	value := elements[0].Value()

	// A nil list was stored as a null
	if value.Type == bson.TypeNull {
		return nil, nil
	}

	if value.Type != bson.TypeArray {
		return nil, derp.BadRequest(location, "Legacy Slice 'values' must be a BSON array", value.Type.String())
	}

	return decodeBSONArray[T](value.Value)
}

/******************************************
 * Helpers
 ******************************************/

// isSizedBSON returns TRUE if the data is as long as its own BSON length header says it is,
// and ends with the null terminator that every document and array carries
func isSizedBSON(data []byte) bool {

	// The smallest document is a four-byte length and a null terminator
	const minimumLength = 5

	if len(data) < minimumLength {
		return false
	}

	if uint64(binary.LittleEndian.Uint32(data)) != uint64(len(data)) {
		return false
	}

	// RULE: The terminator is checked here, because the driver's Elements() does not look for it
	return data[len(data)-1] == 0
}

// cloneSlice returns a copy of the list that shares no memory with it, and is never nil
func cloneSlice[T any](values []T) []T {
	return append(make([]T, 0, len(values)), values...)
}

// matchSlice pairs each value in the first list with an equal value in the second, counting
// duplicates. It returns the values that found a pair, and the values that did not.
func matchSlice[T comparable](first []T, second []T) (matched []T, unmatched []T) {

	matched = make([]T, 0, len(first))
	unmatched = make([]T, 0)

	// Count how many of each value the second list has to offer
	available := make(map[T]int, len(second))
	for _, value := range second {
		available[value]++
	}

	for _, value := range first {

		// Each value in the second list pairs only once
		if available[value] > 0 {
			available[value]--
			matched = append(matched, value)
			continue
		}

		unmatched = append(unmatched, value)
	}

	return matched, unmatched
}
