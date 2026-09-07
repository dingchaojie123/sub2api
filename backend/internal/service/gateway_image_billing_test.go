//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayServiceAPIKeyWithFreshGroupMediaPricing(t *testing.T) {
	t.Parallel()

	groupID := int64(42)
	price1K := 0.012
	freshGroup := &Group{
		ID:                 groupID,
		ImageRateMultiplier: 1,
		VideoRateMultiplier: 1,
		ImagePrice1K:        &price1K,
	}
	repo := &mockGroupRepoForGateway{
		groups: map[int64]*Group{groupID: freshGroup},
	}
	staleAPIKey := &APIKey{
		GroupID: &groupID,
		Group:   &Group{ID: groupID},
	}
	svc := &GatewayService{groupRepo: repo}

	refreshed := svc.apiKeyWithFreshGroupMediaPricing(context.Background(), staleAPIKey)

	require.NotSame(t, staleAPIKey, refreshed)
	require.Same(t, freshGroup, refreshed.Group)
	require.Equal(t, 1, repo.getByIDLiteCalls)
}
