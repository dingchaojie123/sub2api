-- Seedance aggregate groups: per-group routing override of a model line.
--
-- A row (group_id, line_name, target_line_name) makes requests that ask the line `line_name`
-- inside group `group_id` be served by the line `target_line_name` (another upstream platform)
-- instead. The client keeps sending, being billed for and seeing `line_name`; only the upstream
-- that runs the request changes. No row means the line serves itself.
--
-- Rows are written and read by the application only (admin: groups page, "线路路由"). Names are
-- the public names of the lines in service/pp_video_seedance_hub.go and are validated there,
-- not here, because the line table lives in code.
CREATE TABLE IF NOT EXISTS pp_video_line_routes (
    group_id         BIGINT       NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    line_name        VARCHAR(255) NOT NULL,
    target_line_name VARCHAR(255) NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, line_name),
    CONSTRAINT pp_video_line_routes_not_self CHECK (line_name <> target_line_name)
);
