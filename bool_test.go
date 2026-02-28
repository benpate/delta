package delta

import (
	"testing"

	"github.com/stretchr/testify/require"
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
