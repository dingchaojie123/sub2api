package handler

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveJimengVideoIdempotencyKey(t *testing.T) {
	t.Run("preserves explicit key", func(t *testing.T) {
		key, legacy := resolveJimengVideoIdempotencyKey("  client-key-1  ", "payload-hash")
		require.Equal(t, "client-key-1", key)
		require.False(t, legacy)
	})

	t.Run("falls back to payload hash when key is missing", func(t *testing.T) {
		key, legacy := resolveJimengVideoIdempotencyKey("", "  abc123  ")
		require.True(t, legacy)
		require.Equal(t, legacyJimengVideoIdempotencyKeyPrefix+"abc123", key)
		require.LessOrEqual(t, len(key), service.JimengVideoIdempotencyKeyMaxLength)
	})

	t.Run("missing key and empty hash stay empty", func(t *testing.T) {
		key, legacy := resolveJimengVideoIdempotencyKey(" ", " ")
		require.True(t, legacy)
		require.Equal(t, "", key)
	})

	t.Run("fallback key is stable", func(t *testing.T) {
		key1, _ := resolveJimengVideoIdempotencyKey("", strings.Repeat("a", 64))
		key2, _ := resolveJimengVideoIdempotencyKey("", strings.Repeat("a", 64))
		require.Equal(t, key1, key2)
	})
}
