package delta

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestBool(t *testing.T) {

	value := NewBool(false)

	require.False(t, value.Value())
	require.True(t, value.NotChanged())
	require.False(t, value.IsChanged())

	value.Set(true)

	require.True(t, value.Value())
	require.False(t, value.NotChanged())
	require.True(t, value.IsChanged())
}

// TestBool_Original verifies that the baseline survives a change to the current value
func TestBool_Original(t *testing.T) {

	value := NewBool(false)
	require.False(t, value.Original())

	value.Set(true)

	require.True(t, value.Value())
	require.False(t, value.Original(), "the baseline does not move when the value does")
}

// TestBool_OriginalAfterUnmarshal is the case Original() exists for
func TestBool_OriginalAfterUnmarshal(t *testing.T) {

	document, err := bson.Marshal(bson.M{"isActive": true})
	require.NoError(t, err)

	result := struct {
		IsActive Bool `bson:"isActive"`
	}{}

	require.NoError(t, bson.Unmarshal(document, &result))
	require.True(t, result.IsActive.Original())

	result.IsActive.Set(false)

	require.True(t, result.IsActive.Original(), "the value the database still holds")
	require.True(t, result.IsActive.IsChanged())
}
