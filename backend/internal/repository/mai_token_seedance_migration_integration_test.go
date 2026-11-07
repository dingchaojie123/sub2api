//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMaiTokenSeedanceQuotaMigrationPreservesExistingPlatforms(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	_, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE user_platform_quotas (platform VARCHAR(32) NOT NULL) ON COMMIT DROP;
		CREATE TEMP TABLE pp_video_tasks (id BIGSERIAL PRIMARY KEY) ON COMMIT DROP;
		ALTER TABLE user_platform_quotas
			ADD CONSTRAINT user_platform_quotas_platform_check
			CHECK (platform IN ('openai', 'legacy-provider'));
		INSERT INTO user_platform_quotas (platform) VALUES ('legacy-provider');
	`)
	require.NoError(t, err)

	migrationSQL, err := migrations.FS.ReadFile("210_pp_video_provider_billing_seconds.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, `UPDATE user_platform_quotas SET platform = platform WHERE platform = 'legacy-provider'`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO user_platform_quotas (platform) VALUES ('mai-token-seedance')`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO pp_video_tasks (provider_billing_seconds) VALUES (11)`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO pp_video_tasks (provider_billing_seconds) VALUES (-1)`)
	require.Error(t, err)
}
