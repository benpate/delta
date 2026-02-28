package delta

import (
	"testing"

	"github.com/stretchr/testify/require"
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
