package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration221AddsHongpeiSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("221_user_platform_quotas_add_hongpei_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'hongpei-seedance'")
	require.Contains(t, sql, "'fengyue-seedance'")
	require.Contains(t, sql, "'seedance-hub'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
