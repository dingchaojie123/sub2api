package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration225AddsXMSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("225_user_platform_quotas_add_xm_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'xm-seedance'")
	require.Contains(t, sql, "'zexi-seedance'")
	require.Contains(t, sql, "'huahuo-seedance'")
	require.Contains(t, sql, "'vshi-seedance'")
	require.Contains(t, sql, "'seedance-hub'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
