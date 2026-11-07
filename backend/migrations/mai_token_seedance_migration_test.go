package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration210AddsMaiTokenBillingSecondsAndQuota(t *testing.T) {
	raw, err := FS.ReadFile("210_pp_video_provider_billing_seconds.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "provider_billing_seconds BIGINT NOT NULL DEFAULT 0")
	require.Contains(t, sql, "provider_billing_seconds >= 0")
	require.Contains(t, sql, "'mai-token-seedance'")
	require.Contains(t, sql, "SELECT DISTINCT platform")
}
