-- redeem_codes: 实付金额。
-- 折扣兑换码的面值（value）是积分，pay_amount 是客户实际支付的金额。
-- NULL 表示普通兑换码，兑换行为与此前完全一致。
-- 该列只通过原生 SQL 读写，ent schema 不感知（迁移由 SQL runner 执行，不依赖 ent 自动建表）。
ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS pay_amount DECIMAL(20,8) NULL;

COMMENT ON COLUMN redeem_codes.pay_amount IS '实付金额；非空时兑换按 实付金额 计入真实余额/累计充值/返利，按 value 计入积分余额';
