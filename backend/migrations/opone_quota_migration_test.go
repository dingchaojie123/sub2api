package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration206AllowsOPONEPlatformQuota(t *testing.T) {
	content, err := FS.ReadFile("206_user_platform_quotas_add_opone.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check")
	require.Contains(t, sql, "'guangying', 'opone', 'bytedance'")
}
