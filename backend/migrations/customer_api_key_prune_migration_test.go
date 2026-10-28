package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration200PrunesOnlyNonRetainedCustomerAPIKeys(t *testing.T) {
	sqlBytes, err := FS.ReadFile("200_prune_legacy_customer_api_keys.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	for _, name := range []string{
		"OC--ChatGPT【文本模型，官key】",
		"Claude Code 【官方】",
		"视频【聚合】",
		"OCY--MiniMax-H3【视频】",
		"GPT图片【聚合】",
		"Gemini图片【聚合】",
		"OC--Qwen-TTS【音频】",
	} {
		require.Contains(t, sql, "('"+name+"')")
	}

	require.Equal(t, 2, strings.Count(sql, "customer.role = 'user'"))
	require.Equal(t, 2, strings.Count(sql, "bound_group.id = api_key.group_id"))
	require.Equal(t, 2, strings.Count(sql, "bound_group.deleted_at IS NULL"))
	require.Contains(t, sql, "DELETE FROM user_default_api_keys")
	require.Contains(t, sql, "UPDATE api_keys AS api_key")
	require.Contains(t, sql, "key = '__deleted__legacy_group_prune__' || api_key.id::TEXT")
	require.Contains(t, sql, "api_key.deleted_at IS NULL")
	require.NotContains(t, sql, "DELETE FROM api_keys")
}
