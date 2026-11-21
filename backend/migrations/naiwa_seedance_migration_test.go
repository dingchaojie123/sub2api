package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration226AddsNaiwaSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("226_user_platform_quotas_add_naiwa_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'naiwa-seedance'")
	require.Contains(t, sql, "'xm-seedance'")
	require.Contains(t, sql, "'zexi-seedance'")
	require.Contains(t, sql, "'seedance-hub'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
