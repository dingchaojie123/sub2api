ALTER TABLE users
    ADD COLUMN IF NOT EXISTS display_balance DECIMAL(20,8) NOT NULL DEFAULT 0;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS frozen_display_balance DECIMAL(20,8) NOT NULL DEFAULT 0;

UPDATE users
SET display_balance = balance
WHERE display_balance = 0
  AND balance <> 0;

UPDATE users
SET frozen_display_balance = frozen_balance
WHERE frozen_display_balance = 0
  AND frozen_balance <> 0;

COMMENT ON COLUMN users.display_balance IS '前台展示余额；真实扣费仍使用 balance';
COMMENT ON COLUMN users.frozen_display_balance IS '前台展示冻结余额；真实扣费冻结仍使用 frozen_balance';
