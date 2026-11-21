package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration223AddsHuahuoSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("223_user_platform_quotas_add_huahuo_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'huahuo-seedance'")
	require.Contains(t, sql, "'vshi-seedance'")
	require.Contains(t, sql, "'seedance-hub'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
