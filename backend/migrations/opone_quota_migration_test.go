package migrations

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration206AllowsOPONEPlatformQuota(t *testing.T) {
	content, err := FS.ReadFile("206_user_platform_quotas_add_opone.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check")
	require.Contains(t, sql, "'guangying', 'huli-ai', 'opone', 'bytedance'")
}

func TestMigration206RetainsDeployedChecksum(t *testing.T) {
	content, err := FS.ReadFile("206_user_platform_quotas_add_opone.sql")
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
	require.Equal(t, "6e3ee3faa33839ba95d5610a547bacf3345d478deb7ca92848d5b09849b7c945", fmt.Sprintf("%x", sum))
}
