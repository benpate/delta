package delta

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
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
