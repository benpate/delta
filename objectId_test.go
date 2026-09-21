package delta

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
