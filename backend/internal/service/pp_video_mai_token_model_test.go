package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMaiTokenSeedancePublicModelMapsBothWays(t *testing.T) {
	t.Parallel()

	for _, name := range []string{MaiTokenSeedancePublicModel, MaiTokenSeedanceModel, " " + MaiTokenSeedanceModel + " "} {
		publicModel, ok := ppVideoMaiTokenPublicModel(name)
		require.True(t, ok, name)
		require.Equal(t, MaiTokenSeedancePublicModel, publicModel, name)
		require.Equal(t, []string{MaiTokenSeedancePublicModel, MaiTokenSeedanceModel}, ppVideoMaiTokenModelAliases(name), name)
	}
	for _, name := range []string{"", "SDTJ-2.5-480P", "seedance-2.5", "Seedance 2.5-全参720P"} {
		_, ok := ppVideoMaiTokenPublicModel(name)
		require.False(t, ok, name)
		require.Nil(t, ppVideoMaiTokenModelAliases(name), name)
	}
}

func TestMaiTokenSeedanceNativeRequestAcceptsBothModelNames(t *testing.T) {
	t.Parallel()

	preparedBodies := make([][]byte, 0, 2)
	for _, model := range []string{MaiTokenSeedancePublicModel, MaiTokenSeedanceModel} {
		body := []byte(`{"model":"` + model + `","prompt":"lake","duration":4}`)
		prepared, public, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, body)
		require.NoError(t, err, model)
		require.Equal(t, MaiTokenSeedancePublicModel, public.Model, model)
		require.Equal(t, MaiTokenSeedanceModel, public.UpstreamModel, model)
		// The upstream only ever receives its own model name.
		require.Equal(t, MaiTokenSeedanceModel, gjson.GetBytes(prepared, "model").String(), model)
		preparedBodies = append(preparedBodies, prepared)
	}
	require.JSONEq(t, string(preparedBodies[0]), string(preparedBodies[1]))
}

func TestTranslatePPVideoUnifiedRequestAcceptsTheUpstreamMaiTokenName(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformMaiTokenSeedance, []byte(`{"model":"`+MaiTokenSeedanceModel+`","prompt":"p"}`))

	require.NoError(t, err)
	require.Equal(t, MaiTokenSeedancePublicModel, gjson.GetBytes(translated, "model").String())
}

func TestMaiTokenSeedanceModelListShowsOnlyThePublicName(t *testing.T) {
	t.Parallel()

	models := []string{MaiTokenSeedanceModel, MaiTokenSeedancePublicModel, "seedance-2.5", "gpt-image-2"}
	want := []string{MaiTokenSeedancePublicModel}

	require.Equal(t, want, filterPPVideoPublicModelsForPlatform(PlatformMaiTokenSeedance, models))
	require.Equal(t, want, FilterPPVideoPublicModelsForPlatform(PlatformMaiTokenSeedance, []string{MaiTokenSeedanceModel}))
}

// Accounts configured with either name serve requests for both names, so renaming
// the public model needs no change to existing account settings.
func TestMaiTokenSeedanceAccountsServeBothModelNames(t *testing.T) {
	t.Parallel()

	for _, configured := range []string{MaiTokenSeedanceModel, MaiTokenSeedancePublicModel} {
		account := &Account{
			Platform: PlatformMaiTokenSeedance,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"model_mapping": map[string]any{configured: MaiTokenSeedanceModel},
			},
		}

		require.True(t, account.IsModelSupported(MaiTokenSeedancePublicModel), configured)
		require.True(t, account.IsModelSupported(MaiTokenSeedanceModel), configured)
		require.False(t, account.IsModelSupported("seedance-2.5"), configured)
	}
}

func TestMaiTokenSeedancePublicResponseUsesThePublicModelName(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"id":"task-1","status":"queued"}`)
	for _, recorded := range []string{MaiTokenSeedanceModel, MaiTokenSeedancePublicModel} {
		parsed, err := ParsePPVideoResponse(PlatformMaiTokenSeedance, raw)
		require.NoError(t, err)

		public := NormalizePPVideoPublicResponse(PlatformMaiTokenSeedance, raw, PPVideoPublicRequest{Model: recorded}, parsed)

		require.Equal(t, MaiTokenSeedancePublicModel, gjson.GetBytes(public, "model").String(), recorded)
	}
}

func TestPPVideoPricingModelAliasesCoverBothMaiTokenNames(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{MaiTokenSeedancePublicModel, MaiTokenSeedanceModel}, ppVideoPricingModelAliases(PlatformMaiTokenSeedance, MaiTokenSeedanceModel))
	require.Equal(t, []string{MaiTokenSeedancePublicModel, MaiTokenSeedanceModel}, ppVideoPricingModelAliases(PlatformMaiTokenSeedance, MaiTokenSeedancePublicModel))
	require.Equal(t, ppVideoGuangyingModelAliases("Seedance 2.5-全参720P"), ppVideoPricingModelAliases(PlatformGuangying, "Seedance 2.5-全参720P"))
	require.Nil(t, ppVideoPricingModelAliases(PlatformWan3, "wan3.0-video"))
}

func TestPPVideoPublicModelNameOnlyRenamesMaiToken(t *testing.T) {
	t.Parallel()

	require.Equal(t, MaiTokenSeedancePublicModel, PPVideoPublicModelName(PlatformMaiTokenSeedance, MaiTokenSeedanceModel))
	require.Equal(t, MaiTokenSeedancePublicModel, PPVideoPublicModelName(PlatformMaiTokenSeedance, MaiTokenSeedancePublicModel))
	require.Equal(t, "other-model", PPVideoPublicModelName(PlatformMaiTokenSeedance, "other-model"))
	require.Equal(t, MaiTokenSeedanceModel, PPVideoPublicModelName(PlatformWan3, MaiTokenSeedanceModel))
}

// The prepared request body carries the upstream name, which is what billing sees; a
// channel price configured under either name must still be found.
func TestCalculatePPVideoCostPricesMaiTokenUnderEitherModelName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		configured []string
	}{
		{name: "public name only", configured: []string{MaiTokenSeedancePublicModel}},
		{name: "upstream name only", configured: []string{MaiTokenSeedanceModel}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			groupID := int64(105)
			channelPrice := 1.2
			apiKey := &APIKey{
				GroupID: &groupID,
				Group:   &Group{ID: groupID, Platform: PlatformMaiTokenSeedance, RateMultiplier: 1},
			}
			svc := &OpenAIGatewayService{
				billingService: newTestBillingService(),
				resolver: newPPVideoResolverWithChannel(t, groupID, PlatformMaiTokenSeedance, []ChannelModelPricing{{
					Platform:        PlatformMaiTokenSeedance,
					Models:          tc.configured,
					BillingMode:     BillingModeVideo,
					PerRequestPrice: &channelPrice,
				}}),
			}

			cost, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
				Platform:                      PlatformMaiTokenSeedance,
				Model:                         MaiTokenSeedanceModel,
				RequestedDurationMilliseconds: 5000,
				VideoResolution:               VideoBillingResolution480P,
				VideoCount:                    1,
			})

			require.NoError(t, err)
			require.InDelta(t, channelPrice, cost.BillingUnitPrice, 1e-12)
			require.InDelta(t, 6, cost.TotalCost, 1e-12)
		})
	}
}
