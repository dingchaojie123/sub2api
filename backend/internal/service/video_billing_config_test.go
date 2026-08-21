//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVideoBillingModeIsValidAndRequiresPrice(t *testing.T) {
	price := 0.5

	require.True(t, BillingModeVideo.IsValid())
	require.NoError(t, validatePricingBillingMode([]ChannelModelPricing{{
		Platform:        PlatformJimeng,
		Models:          []string{JimengVideoRoutingModel},
		BillingMode:     BillingModeVideo,
		PerRequestPrice: &price,
	}}))
	require.Error(t, validatePricingBillingMode([]ChannelModelPricing{{
		Platform:    PlatformJimeng,
		Models:      []string{JimengVideoRoutingModel},
		BillingMode: BillingModeVideo,
	}}))
}
