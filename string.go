package delta

import (
	"encoding/json"

	"github.com/benpate/derp"
	"github.com/benpate/rosetta/convert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// String tracks changes to a value since it has been loaded into memory
type String struct {
	current  string
	original string
}

// NewString returns a fully initialized Slice object
func NewString(value string) String {

	return String{
		current:  value,
		original: value,
	}
}

/******************************************
 * Data Manipulation Methods
 ******************************************/

// Set updates the current value
func (v *String) Set(value string) {
	v.current = value
}

// Value returns the value as a string
func (v String) Value() string {
	return v.current
}

// String returns the value as a string
func (v String) String() string {
	return v.current
}

// Pointer returns a pointer to the current value
func (v *String) Pointer() *string {
	return &v.current
}

// Original returns the value as it was when this String was loaded
func (v String) Original() string {

	// The baseline is captured at construction and at unmarshal, so this is the value the
	// database still holds -- which is what an update needs in order to address the row, or
	// to clean up whatever the old value pointed at.
	return v.original
}

// NotChanged returns TRUE if the value has not changed since it was loaded
func (v String) NotChanged() bool {
	return v.current == v.original
}

// IsChanged returns TRUE if the value has changed since it was loaded
func (v String) IsChanged() bool {
	return v.current != v.original
}

// IsZero returns TRUE if the current value is the zero value for this type
func (v String) IsZero() bool {
	return v.current == ""
}

/******************************************
 * Schema Interfaces
 ******************************************/

// GetValue implements schema.StringGetter and returns the current value
func (v String) GetValue() any {
	return v.current
}

// SetValue implements schema.StringSetter and updates the current value,
// tracking changes to the added and deleted lists.
func (v *String) SetValue(value any) error {
	v.current = convert.String(value)
	return nil
}

/******************************************
 * JSON Serialization
 ******************************************/

// MarshalJSON implements the json.Marshaler interface
// and serializes the Slice into a JSON array
func (v String) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.current)
}

// UnmarshalJSON implements the json.Unmarshaler interface
// and deserializes the Slice from a JSON array
func (v *String) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &v.original); err != nil {
		return derp.Wrap(err, "delta.String.UnmarshalJSON", "Unable to unmarshal JSON data", data)
	}

	v.current = v.original
	return nil
}

/******************************************
 * BSON Serialization
 ******************************************/

// MarshalBSONValue implements the bson.ValueMarshaler interface
func (v String) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.MarshalValue(v.current)
}

// UnmarshalBSONValue implements the bson.ValueUnmarshaler interface
func (v *String) UnmarshalBSONValue(t bsontype.Type, data []byte) error {

	if err := bson.UnmarshalValue(t, data, &v.original); err != nil {
		return derp.Wrap(err, "delta.String.UnmarshalBSONValue", "Unable to unmarshal BSON data", t, data)
	}

	v.current = v.original
	return nil
}
