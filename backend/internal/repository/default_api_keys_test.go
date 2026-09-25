package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func setupDefaultKeysRepo(t *testing.T) (*apiKeyRepository, int64) {
	t.Helper()
	repo, client := newAPIKeyRepoSQLite(t)
	ddl, err := migrations.FS.ReadFile("192_user_default_api_keys.sql")
	require.NoError(t, err)
	// SQLite cannot ALTER CHECK constraints; mirror the final constraint from migration 193.
	finalDDL := strings.ReplaceAll(string(ddl), "'text', 'image', 'video'", "'text', 'image', 'video', 'audio'")
	_, err = client.ExecContext(context.Background(), finalDDL)
	require.NoError(t, err)
	user := mustCreateAPIKeyRepoUser(t, context.Background(), client, "defaults@example.com")
	return repo, user.ID
}

func defaultKeysInputs(userID int64) []service.DefaultAPIKey {
	items := make([]service.DefaultAPIKey, 0, 4)
	for _, purpose := range []string{"text", "image", "video", "audio"} {
		items = append(items, service.DefaultAPIKey{Purpose: purpose, APIKey: &service.APIKey{
			UserID: userID, Key: "sk-test-default-" + purpose, Name: purpose, Status: service.StatusActive,
		}})
	}
	return items
}

func TestDefaultAPIKeysRepositoryConcurrentRetry(t *testing.T) {
	repo, userID := setupDefaultKeysRepo(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- repo.CreateDefaultAPIKeys(ctx, userID, defaultKeysInputs(userID)) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	items, err := repo.ListDefaultAPIKeys(ctx, userID)
	require.NoError(t, err)
	require.Len(t, items, 4)
	require.Equal(t, "text", items[0].Purpose)
	require.Equal(t, "image", items[1].Purpose)
	require.Equal(t, "video", items[2].Purpose)
	require.Equal(t, "audio", items[3].Purpose)
	count, err := repo.client.APIKey.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 4, count)
}

func TestDefaultAPIKeysRepositoryAdoptsExistingWithoutChangingRestrictions(t *testing.T) {
	repo, userID := setupDefaultKeysRepo(t)
	ctx := context.Background()
	inputs := defaultKeysInputs(userID)
	expectedIDs := make([]int64, 4)
	expired := time.Now().Add(-time.Hour).Truncate(time.Second)
	for i := range inputs {
		group, err := repo.client.Group.Create().SetName(inputs[i].APIKey.Name).Save(ctx)
		require.NoError(t, err)
		inputs[i].APIKey.GroupID = &group.ID
		first, err := repo.client.APIKey.Create().SetUserID(userID).SetName("renamed").SetKey(fmt.Sprintf("sk-renamed-%d", i)).SetGroupID(group.ID).Save(ctx)
		require.NoError(t, err)
		expectedIDs[i] = first.ID
		if i == 0 {
			// Exact name takes precedence even if it is newer and restricted.
			named, err := repo.client.APIKey.Create().SetUserID(userID).SetName(inputs[i].APIKey.Name).SetKey("sk-restricted").SetGroupID(group.ID).SetStatus("inactive").SetExpiresAt(expired).SetQuota(10).SetQuotaUsed(10).Save(ctx)
			require.NoError(t, err)
			expectedIDs[i] = named.ID
		} else {
			_, err := repo.client.APIKey.Create().SetUserID(userID).SetName(inputs[i].APIKey.Name).SetKey(fmt.Sprintf("sk-deleted-%d", i)).SetGroupID(group.ID).SetDeletedAt(time.Now()).Save(ctx)
			require.NoError(t, err)
			// A matching name without the corresponding group cannot be adopted.
			_, err = repo.client.APIKey.Create().SetUserID(userID).SetName(inputs[i].APIKey.Name).SetKey(fmt.Sprintf("sk-wrong-group-%d", i)).Save(ctx)
			require.NoError(t, err)
		}
	}
	before, err := repo.client.APIKey.Query().Count(ctx)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- repo.CreateDefaultAPIKeys(ctx, userID, inputs) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	items, err := repo.ListDefaultAPIKeys(ctx, userID)
	require.NoError(t, err)
	require.Len(t, items, 4)
	for i, item := range items {
		require.Equal(t, expectedIDs[i], item.APIKey.ID)
	}
	require.Equal(t, "sk-restricted", items[0].APIKey.Key)
	require.Equal(t, "inactive", items[0].APIKey.Status)
	require.True(t, expired.Equal(*items[0].APIKey.ExpiresAt))
	require.Equal(t, float64(10), items[0].APIKey.Quota)
	require.Equal(t, float64(10), items[0].APIKey.QuotaUsed)
	after, err := repo.client.APIKey.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestDefaultAPIKeysRepositoryRollback(t *testing.T) {
	repo, userID := setupDefaultKeysRepo(t)
	ctx := context.Background()
	inputs := defaultKeysInputs(userID)
	inputs[3].APIKey.Key = inputs[0].APIKey.Key
	require.Error(t, repo.CreateDefaultAPIKeys(ctx, userID, inputs))
	count, err := repo.client.APIKey.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	items, err := repo.ListDefaultAPIKeys(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, items)
	require.NoError(t, repo.CreateDefaultAPIKeys(ctx, userID, defaultKeysInputs(userID)))
}

func TestDefaultAPIKeysRepositoryIsolationAndDeletion(t *testing.T) {
	repo, userID := setupDefaultKeysRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateDefaultAPIKeys(ctx, userID, defaultKeysInputs(userID)))
	items, err := repo.ListDefaultAPIKeys(ctx, userID+1)
	require.NoError(t, err)
	require.Empty(t, items)
	items, err = repo.ListDefaultAPIKeys(ctx, userID)
	require.NoError(t, err)
	_, err = repo.client.APIKey.UpdateOneID(items[0].APIKey.ID).SetName("renamed").Save(ctx)
	require.NoError(t, err)
	_, err = repo.client.APIKey.UpdateOneID(items[1].APIKey.ID).SetDeletedAt(time.Now()).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.client.APIKey.DeleteOneID(items[2].APIKey.ID).Exec(ctx))
	require.NoError(t, repo.CreateDefaultAPIKeys(ctx, userID, defaultKeysInputs(userID)))
	items, err = repo.ListDefaultAPIKeys(ctx, userID)
	require.NoError(t, err)
	require.Len(t, items, 4)
	require.Equal(t, "renamed", items[0].APIKey.Name)
	require.Nil(t, items[1].APIKey)
	require.Nil(t, items[2].APIKey)
	count, err := repo.client.APIKey.Query().Where(apikey.DeletedAtIsNil()).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestDefaultAPIKeysRepositoryAddsAudioWithoutReplacingExistingKeys(t *testing.T) {
	repo, userID := setupDefaultKeysRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateDefaultAPIKeys(ctx, userID, defaultKeysInputs(userID)[:3]))
	existing, err := repo.ListDefaultAPIKeys(ctx, userID)
	require.NoError(t, err)
	require.Len(t, existing, 3)
	require.NoError(t, repo.CreateDefaultAPIKeys(ctx, userID, defaultKeysInputs(userID)))
	items, err := repo.ListDefaultAPIKeys(ctx, userID)
	require.NoError(t, err)
	require.Len(t, items, 4)
	for i := range existing {
		require.Equal(t, existing[i].APIKey.ID, items[i].APIKey.ID)
		require.Equal(t, existing[i].APIKey.Key, items[i].APIKey.Key)
	}
	require.Equal(t, "audio", items[3].Purpose)
}

func TestDefaultAPIKeysRepositoryConcurrentReassignment(t *testing.T) {
	repo, userID := setupDefaultKeysRepo(t)
	ctx := context.Background()
	group, err := repo.client.Group.Create().SetName("OC--ChatGPT【文本模型】").Save(ctx)
	require.NoError(t, err)
	keys := make([]*service.APIKey, 2)
	for i := range keys {
		keys[i] = &service.APIKey{UserID: userID, Key: fmt.Sprintf("sk-reassign-%d", i), Name: "candidate", GroupID: &group.ID, Status: service.StatusActive}
		require.NoError(t, repo.Create(ctx, keys[i]))
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(keyID int64) {
			defer wg.Done()
			_, err := repo.SetDefaultAPIKey(ctx, userID, "text", keyID, group.ID)
			errs <- err
		}(keys[i%2].ID)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	items, err := repo.ListDefaultAPIKeys(ctx, userID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Contains(t, []int64{keys[0].ID, keys[1].ID}, items[0].APIKey.ID)
	for _, key := range keys {
		stored, err := repo.GetByID(ctx, key.ID)
		require.NoError(t, err)
		require.Equal(t, key.Key, stored.Key)
		require.Equal(t, service.StatusActive, stored.Status)
	}
	_, err = repo.SetDefaultAPIKey(ctx, userID+1, "text", keys[0].ID, group.ID)
	require.ErrorIs(t, err, service.ErrAPIKeyNotFound)
	_, err = repo.SetDefaultAPIKey(ctx, userID, "text", keys[0].ID, group.ID+1)
	require.ErrorIs(t, err, service.ErrDefaultAPIKeyGroupMismatch)
	_, err = repo.client.APIKey.UpdateOneID(keys[0].ID).SetStatus("inactive").Save(ctx)
	require.NoError(t, err)
	_, err = repo.SetDefaultAPIKey(ctx, userID, "text", keys[0].ID, group.ID)
	require.ErrorIs(t, err, service.ErrDefaultAPIKeyUnavailable)
}
