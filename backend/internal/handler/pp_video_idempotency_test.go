package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvePPVideoIdempotencyKey(t *testing.T) {
	t.Run("preserves explicit key", func(t *testing.T) {
		require.Equal(t, "client-key-1", resolvePPVideoIdempotencyKey("  client-key-1  "))
	})

	t.Run("does not derive key from request content", func(t *testing.T) {
		require.Empty(t, resolvePPVideoIdempotencyKey(""))
		require.Empty(t, resolvePPVideoIdempotencyKey("   "))
	})
}
