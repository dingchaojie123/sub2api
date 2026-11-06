//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestXingAPISeedanceQuotaMigrationPreservesExistingPlatforms(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	_, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE user_platform_quotas (
			platform VARCHAR(32) NOT NULL
		) ON COMMIT DROP;
		ALTER TABLE user_platform_quotas
			ADD CONSTRAINT user_platform_quotas_platform_check
			CHECK (platform IN ('openai', 'legacy-provider'));
		INSERT INTO user_platform_quotas (platform) VALUES ('legacy-provider');
	`)
	require.NoError(t, err)

	migrationSQL, err := migrations.FS.ReadFile("208_user_platform_quotas_add_xingapi_seedance.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, `UPDATE user_platform_quotas SET platform = platform WHERE platform = 'legacy-provider'`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO user_platform_quotas (platform) VALUES ('xingapi-seedance')`)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, "SAVEPOINT before_unknown_platform")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO user_platform_quotas (platform) VALUES ('brand-new-unknown')`)
	require.Error(t, err)
	_, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT before_unknown_platform")
	require.NoError(t, rollbackErr)
}
