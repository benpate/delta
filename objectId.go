package delta

import (
	"encoding/json"

	"github.com/benpate/derp"
	"github.com/benpate/rosetta/convert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ObjectID tracks changes to a value since it has been loaded into memory
type ObjectID struct {
	current  primitive.ObjectID
	original primitive.ObjectID
}

// NewObjectID returns a fully initialized Slice object
func NewObjectID(value primitive.ObjectID) ObjectID {

	return ObjectID{
		current:  value,
		original: value,
	}
}

/******************************************
 * Data Manipulation Methods
 ******************************************/

// Set updates the current value
func (v *ObjectID) Set(value primitive.ObjectID) {
	v.current = value
}

// Value returns the value as a primitive.ObjectID
func (v ObjectID) Value() primitive.ObjectID {
	return v.current
}

// String returns the value as a hex string
func (v ObjectID) String() string {
	return v.current.Hex()
}

// Hex returns the value as a hex string
func (v ObjectID) Hex() string {
	return v.current.Hex()
}

// Pointer returns a pointer to the current value
func (v *ObjectID) Pointer() *primitive.ObjectID {
	return &v.current
}

// NotChanged returns TRUE if the value has not changed since it was loaded
func (v ObjectID) NotChanged() bool {
	return v.current == v.original
}

// IsChanged returns TRUE if the value has changed since it was loaded
func (v ObjectID) IsChanged() bool {
	return v.current != v.original
}

// IsZero returns TRUE if the current value is the zero value for this type
func (v ObjectID) IsZero() bool {
	return v.current.IsZero()
}

/******************************************
 * Schema Interfaces
 ******************************************/

// GetValue implements schema.ObjectIDGetter and returns the current value
func (v ObjectID) GetValue() any {
	return v.current
}

// SetValue implements schema.ObjectIDSetter and updates the current value,
// tracking changes to the added and deleted lists.
func (v *ObjectID) SetValue(value any) error {

	switch typed := value.(type) {

	case primitive.ObjectID:
		v.current = typed
		return nil

	case string:

		objectID, err := primitive.ObjectIDFromHex(typed)
		if err != nil {
			return derp.Wrap(err, "delta.ObjectID.SetValue", "Unable to convert value to ObjectID", "value", value)
		}
		v.current = objectID
		return nil

	default:
		return v.SetValue(convert.String(value))
	}
}

/******************************************
 * JSON Serialization
 ******************************************/

// MarshalJSON implements the json.Marshaler interface
// and serializes the Slice into a JSON array
func (v ObjectID) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.current)
}

// UnmarshalJSON implements the json.Unmarshaler interface
// and deserializes the Slice from a JSON array
func (v *ObjectID) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &v.original); err != nil {
		return derp.Wrap(err, "delta.ObjectID.UnmarshalJSON", "Unable to unmarshal JSON data", data)
	}

	v.current = v.original
	return nil
}

/******************************************
 * BSON Serialization
 ******************************************/

// MarshalBSONValue implements the bson.ValueMarshaler interface
func (v ObjectID) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.MarshalValue(v.current)
}

// UnmarshalBSONValue implements the bson.ValueUnmarshaler interface
func (v *ObjectID) UnmarshalBSONValue(t bsontype.Type, data []byte) error {

	if err := bson.UnmarshalValue(t, data, &v.original); err != nil {
		return derp.Wrap(err, "delta.ObjectID.UnmarshalBSONValue", "Unable to unmarshal BSON data", t, data)
	}

	v.current = v.original
	return nil
}
