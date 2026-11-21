package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration217AddsApophisSeedanceQuotaPlatform(t *testing.T) {
	raw, err := FS.ReadFile("217_user_platform_quotas_add_apophis_seedance.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "'apophis-seedance'")
	require.Contains(t, sql, "'paisio-seedance'")
	require.Contains(t, sql, "'seedance-hub'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
	require.Contains(t, sql, "user_platform_quotas_platform_check")
}
