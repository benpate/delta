package delta

import (
	"encoding/json"

	"github.com/benpate/derp"
	"github.com/benpate/rosetta/convert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// Bool tracks changes to a value since it has been loaded into memory
type Bool struct {
	current  bool
	original bool
}

// NewBool returns a fully initialized Slice object
func NewBool(value bool) Bool {

	return Bool{
		current:  value,
		original: value,
	}
}

/******************************************
 * Data Manipulation Methods
 ******************************************/

// Set updates the current value
func (v *Bool) Set(value bool) {
	v.current = value
}

// Value returns the value as a bool
func (v Bool) Value() bool {
	return v.current
}

// Bool returns the value as a bool
func (v Bool) Bool() bool {
	return v.current
}

// Pointer returns a pointer to the current value
func (v *Bool) Pointer() *bool {
	return &v.current
}

// NotChanged returns TRUE if the value has not changed since it was loaded
func (v Bool) NotChanged() bool {
	return v.current == v.original
}

// IsChanged returns TRUE if the value has changed since it was loaded
func (v Bool) IsChanged() bool {
	return v.current != v.original
}

// IsZero returns TRUE if the current value is the zero value for this type
func (v Bool) IsZero() bool {
	return !v.current
}

// IsTrue returns TRUE if the current value is true
func (v Bool) IsTrue() bool {
	return v.current
}

// IsFalse returns TRUE if the current value is false
func (v Bool) IsFalse() bool {
	return !v.current
}

/******************************************
 * Schema Interfaces
 ******************************************/

// GetValue implements schema.BoolGetter and returns the current value
func (v Bool) GetValue() any {
	return v.current
}

// SetValue implements schema.BoolSetter and updates the current value,
// tracking changes to the added and deleted lists.
func (v *Bool) SetValue(value any) error {
	v.current = convert.Bool(value)
	return nil
}

/******************************************
 * JSON Serialization
 ******************************************/

// MarshalJSON implements the json.Marshaler interface
// and serializes the Slice into a JSON array
func (v Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.current)
}

// UnmarshalJSON implements the json.Unmarshaler interface
// and deserializes the Slice from a JSON array
func (v *Bool) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &v.original); err != nil {
		return derp.Wrap(err, "delta.Bool.UnmarshalJSON", "Unable to unmarshal JSON data", data)
	}

	v.current = v.original
	return nil
}

/******************************************
 * BSON Serialization
 ******************************************/

// MarshalBSONValue implements the bson.ValueMarshaler interface
func (v Bool) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.MarshalValue(v.current)
}

// UnmarshalBSONValue implements the bson.ValueUnmarshaler interface
func (v *Bool) UnmarshalBSONValue(t bsontype.Type, data []byte) error {

	if err := bson.UnmarshalValue(t, data, &v.original); err != nil {
		return derp.Wrap(err, "delta.Bool.UnmarshalBSONValue", "Unable to unmarshal BSON data", t, data)
	}

	v.current = v.original
	return nil
}
