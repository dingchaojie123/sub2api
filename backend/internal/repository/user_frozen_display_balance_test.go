//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrozenDisplayBalanceOrBalance(t *testing.T) {
	// 有积分余额的账号：冻结积分用 frozen_display_balance，不是真实余额口径的 frozen_balance。
	require.InDelta(t, 19.0, frozenDisplayBalanceOrBalance(19, 14.61), 1e-9)
	// 没有积分余额的老账号：冻结时不写 frozen_display_balance，回退到 frozen_balance。
	require.InDelta(t, 3.0, frozenDisplayBalanceOrBalance(0, 3), 1e-9)
	// 没有冻结。
	require.InDelta(t, 0.0, frozenDisplayBalanceOrBalance(0, 0), 1e-9)
}
