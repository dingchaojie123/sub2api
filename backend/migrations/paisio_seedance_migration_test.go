package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration216AddsPaisioSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("216_user_platform_quotas_add_paisio_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'paisio-seedance'")
	require.Contains(t, sql, "'miaowu-seedance'")
	require.Contains(t, sql, "'seedance-hub'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
