package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration202ReplacesLegacyDefaultVideoKeys(t *testing.T) {
	sqlBytes, err := FS.ReadFile("202_replace_default_video_api_keys.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	require.Contains(t, sql, "bound_group.name = '视频【聚合】'")
	require.Contains(t, sql, "customer.role = 'user'")
	require.Contains(t, sql, "DELETE FROM user_default_api_keys")
	require.Contains(t, sql, "defaults.purpose = 'video'")
	require.Contains(t, sql, "UPDATE api_keys AS api_key")
	require.Contains(t, sql, "key = '__deleted__legacy_video_group__' || api_key.id::TEXT")
	require.Contains(t, sql, "api_key.deleted_at IS NULL")
	require.NotContains(t, sql, "DELETE FROM api_keys")
}

func TestMigration203FixesTikTokSeedanceOneModelMappings(t *testing.T) {
	sqlBytes, err := FS.ReadFile("203_fix_tiktok_seedance_one_model_mapping.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	require.Contains(t, sql, "bound_group.name = 'TikTok--Seedance'")
	require.Contains(t, sql, "account.platform IN ('seedance', 'ctmo-seedance')")
	require.Contains(t, sql, "credentials->'model_mapping'->>'seedance2.5-one-720p' = 'SD2.5-30-10-10'")
	require.Contains(t, sql, "credentials->'model_mapping'->>'seedance2.5-one-1080p' = 'SD2.5-30-10-10'")
	require.Contains(t, sql, `'"seedance2.5-one-720p"'::jsonb`)
	require.Contains(t, sql, `'"seedance2.5-one-1080p"'::jsonb`)
	require.NotContains(t, sql, "DELETE FROM accounts")
}

func TestMigration207ReplacesTikTokDefaultVideoKeys(t *testing.T) {
	sqlBytes, err := FS.ReadFile("207_replace_tiktok_default_video_api_keys.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	require.Contains(t, sql, "bound_group.name = 'TikTok--Seedance'")
	require.Contains(t, sql, "customer.role = 'user'")
	require.Contains(t, sql, "DELETE FROM user_default_api_keys")
	require.Contains(t, sql, "defaults.purpose = 'video'")
	require.NotContains(t, sql, "UPDATE api_keys")
	require.NotContains(t, sql, "DELETE FROM api_keys")
}
