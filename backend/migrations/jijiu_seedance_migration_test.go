package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration212AddsJijiuSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("212_user_platform_quotas_add_jijiu_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'jijiu-seedance'")
	require.Contains(t, sql, "'mai-token-seedance'")
	require.Contains(t, sql, "'xingapi-seedance'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
