package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 聚合分组的「线路路由」存取：pp_video_line_routes（迁移 228）。
// 调用方（管理端路由 / 视频提交）负责鉴权与名字校验；这里只做存取。

func (r *usageBillingRepository) ListPPVideoLineRoutes(ctx context.Context, groupID int64) (map[string]string, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT line_name, target_line_name FROM pp_video_line_routes WHERE group_id = $1`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	routes := make(map[string]string)
	for rows.Next() {
		var line, target string
		if err := rows.Scan(&line, &target); err != nil {
			return nil, err
		}
		routes[line] = target
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return routes, nil
}

func (r *usageBillingRepository) SetPPVideoLineRoute(ctx context.Context, groupID int64, lineName, targetLineName string) error {
	if r == nil || r.db == nil {
		return sql.ErrConnDone
	}
	// $2 and $3 are only ever compared / stored as text; the casts keep PostgreSQL from having to
	// infer a type for a parameter that appears twice (the 2026-11 submit incident).
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pp_video_line_routes (group_id, line_name, target_line_name)
		VALUES ($1, $2::text, $3::text)
		ON CONFLICT (group_id, line_name)
		DO UPDATE SET target_line_name = EXCLUDED.target_line_name, updated_at = NOW()
	`, groupID, strings.TrimSpace(lineName), strings.TrimSpace(targetLineName))
	return err
}

func (r *usageBillingRepository) DeletePPVideoLineRoute(ctx context.Context, groupID int64, lineName string) error {
	if r == nil || r.db == nil {
		return sql.ErrConnDone
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM pp_video_line_routes WHERE group_id = $1 AND line_name = $2::text`,
		groupID, strings.TrimSpace(lineName))
	return err
}

func (r *usageBillingRepository) GetPPVideoLineRouteGroupPlatform(ctx context.Context, groupID int64) (string, error) {
	if r == nil || r.db == nil {
		return "", sql.ErrConnDone
	}
	var platform string
	err := r.db.QueryRowContext(ctx,
		`SELECT platform FROM groups WHERE id = $1 AND deleted_at IS NULL`, groupID).Scan(&platform)
	if errors.Is(err, sql.ErrNoRows) {
		return "", service.ErrPPVideoLineRouteGroupNotFound
	}
	if err != nil {
		return "", err
	}
	return platform, nil
}

// ppVideoLineRoutesSwitchKey is the row of the settings table that holds the global switch of line
// routes ("true" = on). The row is absent until an operator first turns the switch on.
const ppVideoLineRoutesSwitchKey = "pp_video_line_routes_enabled"

func (r *usageBillingRepository) GetPPVideoLineRoutesEnabled(ctx context.Context) (bool, error) {
	if r == nil || r.db == nil {
		return false, sql.ErrConnDone
	}
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1::text`, ppVideoLineRoutesSwitchKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(value), "true"), nil
}

func (r *usageBillingRepository) SetPPVideoLineRoutesEnabled(ctx context.Context, enabled bool) error {
	if r == nil || r.db == nil {
		return sql.ErrConnDone
	}
	value := "false"
	if enabled {
		value = "true"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES ($1::text, $2::text, NOW())
		ON CONFLICT (key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, ppVideoLineRoutesSwitchKey, value)
	return err
}
