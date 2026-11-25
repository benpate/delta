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

// Clear sets the current value to the nil ObjectID, leaving the baseline as it was
func (v *ObjectID) Clear() {
	v.current = primitive.NilObjectID
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

// Original returns the value as it was when this ObjectID was loaded
func (v ObjectID) Original() primitive.ObjectID {

	// The baseline is captured at construction and at unmarshal, so this is the value the
	// database still holds -- which is what an update needs in order to address the row, or
	// to clean up whatever the old value pointed at.
	return v.original
}

// Reset makes the current value the baseline, for a caller that has just saved it
func (v *ObjectID) Reset() {
	v.original = v.current
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

	const location = "delta.ObjectID.SetValue"

	switch typed := value.(type) {

	case nil:
		v.current = primitive.NilObjectID
		return nil

	case primitive.ObjectID:
		v.current = typed
		return nil

	case *primitive.ObjectID:

		// A nil pointer clears the value
		if typed == nil {
			v.current = primitive.NilObjectID
			return nil
		}

		v.current = *typed
		return nil

	case string:

		// An empty string clears the value
		if typed == "" {
			v.current = primitive.NilObjectID
			return nil
		}

		objectID, err := primitive.ObjectIDFromHex(typed)
		if err != nil {
			return derp.Wrap(err, location, "Unable to convert value to ObjectID", "value", value)
		}
		v.current = objectID
		return nil

	default:

		// RULE: A value that has no string form is rejected, not read as empty
		text, isLossless := convert.StringOk(value, "")
		if (text == "") && !isLossless {
			return derp.BadRequest(location, "Unable to convert value to ObjectID", "value", value)
		}

		return v.SetValue(text)
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

	const location = "delta.ObjectID.UnmarshalJSON"

	// RULE: Only a JSON string or null is accepted. A null leaves the hex string empty.
	hex := ""
	if err := json.Unmarshal(data, &hex); err != nil {
		return derp.Wrap(err, location, "Unable to unmarshal JSON data", data)
	}

	// An empty string or a null becomes the nil ObjectID
	decoded := primitive.NilObjectID

	// RULE: Any other string must be exactly 24 hex characters
	if hex != "" {
		parsed, err := primitive.ObjectIDFromHex(hex)
		if err != nil {
			return derp.Wrap(err, location, "ObjectID must be a 24-character hex string", data)
		}
		decoded = parsed
	}

	v.original = decoded
	v.current = decoded
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
