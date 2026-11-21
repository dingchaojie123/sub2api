package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration215AddsMiaowuSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("215_user_platform_quotas_add_miaowu_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'miaowu-seedance'")
	require.Contains(t, sql, "'jijiu-seedance'")
	require.Contains(t, sql, "'seedance-hub'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
