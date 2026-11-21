package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration229AddsV6KSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("229_user_platform_quotas_add_v6k_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'v6k-seedance'")
	require.Contains(t, sql, "'naiwa-seedance'")
	require.Contains(t, sql, "'kyy-wan'")
	require.Contains(t, sql, "'seedance-hub'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
