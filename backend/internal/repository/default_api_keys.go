package repository

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.DefaultAPIKeyRepository = (*apiKeyRepository)(nil)
var _ service.DefaultAPIKeyUpdater = (*apiKeyRepository)(nil)

func (r *apiKeyRepository) SetDefaultAPIKey(ctx context.Context, userID int64, purpose string, apiKeyID, groupID int64) (*service.APIKey, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	txCtx := dbent.NewTxContext(ctx, tx)
	release, err := lockRepositoryScopedKeys(txCtx, client, client, fmt.Sprintf("default-api-keys:%d", userID))
	if err != nil {
		return nil, err
	}
	defer release()
	query := client.APIKey.Query().Where(apikey.IDEQ(apiKeyID), apikey.UserIDEQ(userID), apikey.DeletedAtIsNil()).WithGroup()
	// Keep deletion and group changes from racing the association write.
	if client.Driver().Dialect() == dialect.Postgres {
		query.ForUpdate()
	}
	key, err := query.Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrAPIKeyNotFound
		}
		return nil, err
	}
	if key.GroupID == nil || *key.GroupID != groupID {
		return nil, service.ErrDefaultAPIKeyGroupMismatch
	}
	activeGroup, err := client.Group.Query().Where(group.IDEQ(groupID), group.DeletedAtIsNil(), group.StatusEQ(service.StatusActive)).Exist(txCtx)
	if err != nil {
		return nil, err
	}
	if !activeGroup {
		return nil, service.ErrGroupNotAllowed
	}
	result := apiKeyEntityToService(key)
	if !result.IsActive() || result.IsExpired() || result.IsQuotaExhausted() {
		return nil, service.ErrDefaultAPIKeyUnavailable
	}
	rows, err := client.QueryContext(txCtx, `SELECT purpose FROM user_default_api_keys WHERE user_id = $1 AND api_key_id = $2 AND purpose <> $3 LIMIT 1`, userID, apiKeyID, purpose)
	if err != nil {
		return nil, err
	}
	alreadyAssigned := rows.Next()
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if alreadyAssigned {
		return nil, service.ErrDefaultAPIKeyGroupMismatch
	}
	if _, err := client.ExecContext(txCtx, `INSERT INTO user_default_api_keys (user_id, purpose, api_key_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, purpose) DO UPDATE SET api_key_id = excluded.api_key_id`, userID, purpose, apiKeyID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *apiKeyRepository) ListDefaultAPIKeys(ctx context.Context, userID int64) ([]service.DefaultAPIKey, error) {
	rows, err := r.client.QueryContext(ctx, `SELECT purpose, api_key_id FROM user_default_api_keys WHERE user_id = $1 ORDER BY CASE purpose WHEN 'text' THEN 1 WHEN 'image' THEN 2 WHEN 'video' THEN 3 WHEN 'audio' THEN 4 ELSE 5 END`, userID)
	if err != nil {
		return nil, err
	}
	items := make([]service.DefaultAPIKey, 0, 4)
	ids := make([]int64, 0, 4)
	byID := make(map[int64]int)
	for rows.Next() {
		var purpose string
		var id sql.NullInt64
		if err := rows.Scan(&purpose, &id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if id.Valid {
			ids = append(ids, id.Int64)
			byID[id.Int64] = len(items)
		}
		items = append(items, service.DefaultAPIKey{Purpose: purpose})
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil || len(ids) == 0 {
		return items, err
	}
	keys, err := r.activeQuery().Where(apikey.UserIDEQ(userID), apikey.IDIn(ids...)).WithGroup().All(ctx)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		items[byID[key.ID]].APIKey = apiKeyEntityToService(key)
	}
	return items, nil
}

func (r *apiKeyRepository) CreateDefaultAPIKeys(ctx context.Context, userID int64, keys []service.DefaultAPIKey) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	txCtx := dbent.NewTxContext(ctx, tx)
	release, err := lockRepositoryScopedKeys(txCtx, client, client, fmt.Sprintf("default-api-keys:%d", userID))
	if err != nil {
		return err
	}
	defer release()
	if _, err := client.User.Query().Where(user.IDEQ(userID), user.DeletedAtIsNil(), user.StatusEQ(service.StatusActive)).Only(txCtx); err != nil {
		return err
	}
	txRepo := &apiKeyRepository{client: client, sql: client}
	existing, err := txRepo.ListDefaultAPIKeys(txCtx, userID)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(existing))
	assignedIDs := make([]int64, 0, len(existing)+len(keys))
	for _, item := range existing {
		seen[item.Purpose] = true
		if item.APIKey != nil {
			assignedIDs = append(assignedIDs, item.APIKey.ID)
		}
	}
	for _, item := range keys {
		if seen[item.Purpose] {
			continue
		}
		if item.APIKey == nil || item.APIKey.UserID != userID {
			return fmt.Errorf("invalid default API key owner")
		}
		selected := item.APIKey
		if selected.GroupID != nil {
			// Adopt an existing key without changing its credentials or restrictions.
			query := txRepo.activeQuery().Where(apikey.UserIDEQ(userID), apikey.GroupIDEQ(*selected.GroupID)).Order(dbent.Asc(apikey.FieldID))
			if len(assignedIDs) > 0 {
				query.Where(apikey.IDNotIn(assignedIDs...))
			}
			if client.Driver().Dialect() == dialect.Postgres {
				query.ForUpdate()
			}
			key, err := query.Clone().Where(apikey.NameEQ(selected.Name)).First(txCtx)
			if dbent.IsNotFound(err) {
				key, err = query.First(txCtx)
			}
			if err != nil && !dbent.IsNotFound(err) {
				return err
			}
			if key != nil {
				selected = apiKeyEntityToService(key)
			}
		}
		if selected.ID == 0 {
			if err := txRepo.Create(txCtx, selected); err != nil {
				return err
			}
		}
		if _, err := client.ExecContext(txCtx, `INSERT INTO user_default_api_keys (user_id, purpose, api_key_id) VALUES ($1, $2, $3)`, userID, item.Purpose, selected.ID); err != nil {
			return err
		}
		seen[item.Purpose] = true
		assignedIDs = append(assignedIDs, selected.ID)
	}
	return tx.Commit()
}
