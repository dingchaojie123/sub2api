package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type mediaQuoteAPIKeyRepo struct {
	APIKeyRepository
	key *APIKey
}

func (r *mediaQuoteAPIKeyRepo) GetByID(context.Context, int64) (*APIKey, error) { return r.key, nil }

type mediaQuoteOrganizationRepo struct {
	OrganizationRepository
	organization *Organization
	source       APIKeyBillingSource
}

type mediaQuoteChannelRepo struct {
	ChannelRepository
	channels  []Channel
	platforms map[int64]string
}

func (r *mediaQuoteChannelRepo) ListAll(context.Context) ([]Channel, error) {
	return r.channels, nil
}

func (r *mediaQuoteChannelRepo) GetGroupPlatforms(context.Context, []int64) (map[int64]string, error) {
	return r.platforms, nil
}

func (r *mediaQuoteOrganizationRepo) GetForUser(context.Context, int64, int64) (*Organization, error) {
	return r.organization, nil
}

func (r *mediaQuoteOrganizationRepo) GetAPIKeyBillingSources(context.Context, []int64) (map[int64]APIKeyBillingSource, error) {
	return map[int64]APIKeyBillingSource{7: r.source}, nil
}

func newMediaQuoteServiceForTest(key *APIKey) *MediaQuoteService {
	svc := NewMediaQuoteService(&mediaQuoteAPIKeyRepo{key: key}, nil, nil, NewBillingService(&config.Config{}, nil), nil, &config.Config{})
	svc.now = func() time.Time { return time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC) }
	return svc
}

func mediaQuoteTestKey() *APIKey {
	image1K, video720 := 0.2, 0.1
	groupID := int64(3)
	return &APIKey{
		ID: 7, UserID: 42, Status: StatusAPIKeyActive, GroupID: &groupID,
		User:          &User{ID: 42, Balance: 10, DisplayBalance: 100},
		Group:         &Group{ID: groupID, Status: StatusActive, RateMultiplier: 2, ImagePrice1K: &image1K, VideoPrice720P: &video720},
		BillingSource: &APIKeyBillingSource{Type: APIKeyBillingSourcePersonal, Status: APIKeyBillingSourceActive},
	}
}

func TestMediaQuoteServiceImage(t *testing.T) {
	quote, err := newMediaQuoteServiceForTest(mediaQuoteTestKey()).Quote(context.Background(), 42, MediaQuoteRequest{
		APIKeyID: 7, TaskType: "image", Model: "image-model", Quantity: 3, Resolution: "1k",
	})
	require.NoError(t, err)
	require.Equal(t, "estimate", quote.Status)
	require.Equal(t, "1K", quote.Pricing.Resolution)
	require.InDelta(t, 1.2, quote.SettlementAmount.Value, 0.0000001)
	require.InDelta(t, 1.2, quote.EstimatedCredits, 0.0000001)
	require.Equal(t, "unit_price_per_image * quantity * rate_multiplier", quote.Pricing.Formula)
	require.Equal(t, "group", quote.Pricing.PricingSource)
	require.Equal(t, string(BillingModeImage), quote.Pricing.BillingMode)
}

func TestMediaQuoteServiceVideo(t *testing.T) {
	quote, err := newMediaQuoteServiceForTest(mediaQuoteTestKey()).Quote(context.Background(), 42, MediaQuoteRequest{
		APIKeyID: 7, TaskType: "video", Model: "video-model", Quantity: 2, Resolution: "720p", DurationSeconds: 5,
	})
	require.NoError(t, err)
	require.Equal(t, "720p", quote.Pricing.Resolution)
	require.InDelta(t, 2, quote.SettlementAmount.Value, 0.0000001)
	require.InDelta(t, 2, quote.EstimatedCredits, 0.0000001)
	require.NotNil(t, quote.Pricing.DurationSeconds)
	require.Equal(t, 5, *quote.Pricing.DurationSeconds)
	require.Equal(t, "group", quote.Pricing.PricingSource)
	require.Equal(t, string(BillingModeVideo), quote.Pricing.BillingMode)
}

func TestMediaQuoteServiceVideoUsesChannelBillingMode(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		billingMode      BillingMode
		unitPrice        float64
		wantSettlement   float64
		wantCredits      float64
		wantFormula      string
		wantSnapshotMode string
	}{
		{
			name:             "per request does not multiply duration",
			model:            "seedance-2.0-930",
			billingMode:      BillingModePerRequest,
			unitPrice:        5.6,
			wantSettlement:   22.4,
			wantCredits:      22.4,
			wantFormula:      "unit_price_per_request * quantity * rate_multiplier",
			wantSnapshotMode: string(BillingModePerRequest),
		},
		{
			name:             "per second multiplies duration",
			model:            "seedance2.5-pro-720p",
			billingMode:      BillingModeVideo,
			unitPrice:        1.4,
			wantSettlement:   28,
			wantCredits:      28,
			wantFormula:      "unit_price_per_second * duration_seconds * quantity * rate_multiplier",
			wantSnapshotMode: string(BillingModeVideo),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := mediaQuoteTestKey()
			key.Group.Platform = PlatformCTMOSeedance
			repo := &mediaQuoteChannelRepo{
				channels: []Channel{{
					ID: 9, Name: "ctmo-video", Status: StatusActive, GroupIDs: []int64{*key.GroupID},
					ModelPricing: []ChannelModelPricing{{
						ID: 10, ChannelID: 9, Platform: PlatformCTMOSeedance, Models: []string{tt.model},
						BillingMode: tt.billingMode, PerRequestPrice: &tt.unitPrice,
					}},
				}},
				platforms: map[int64]string{*key.GroupID: PlatformCTMOSeedance},
			}
			billing := NewBillingService(&config.Config{}, nil)
			resolver := NewModelPricingResolver(NewChannelService(repo, nil, nil, nil), billing)
			svc := NewMediaQuoteService(&mediaQuoteAPIKeyRepo{key: key}, nil, nil, billing, resolver, &config.Config{})

			quote, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{
				APIKeyID: 7, TaskType: "video", Model: tt.model, Quantity: 2,
				Resolution: "720p", DurationSeconds: 5,
			})

			require.NoError(t, err)
			require.InDelta(t, tt.unitPrice, quote.Pricing.UnitPrice, 0.0000001)
			require.InDelta(t, tt.wantSettlement, quote.SettlementAmount.Value, 0.0000001)
			require.InDelta(t, tt.wantCredits, quote.EstimatedCredits, 0.0000001)
			require.Equal(t, tt.wantFormula, quote.Pricing.Formula)
			require.Equal(t, tt.wantSnapshotMode, quote.Pricing.BillingMode)
			require.Equal(t, PricingSourceChannel, quote.Pricing.PricingSource)
		})
	}
}

func TestMediaQuoteServiceVideoRejectsMissingRequiredChannelPricing(t *testing.T) {
	key := mediaQuoteTestKey()
	key.Group.Platform = PlatformCTMOSeedance
	svc := newMediaQuoteServiceForTest(key)

	_, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{
		APIKeyID: 7, TaskType: "video", Model: "seedance-2.0-930", Quantity: 1,
		Resolution: "720p", DurationSeconds: 5,
	})

	require.ErrorIs(t, err, ErrMediaQuotePricingUnavailable)
}

func TestNormalizeMediaQuoteVideoSpecDoesNotApplyTheXAILimitToChannelPricedPlatforms(t *testing.T) {
	for _, platform := range []string{PlatformGuangying, PlatformXingAPISeedance, PlatformCTMOSeedance, Platform88APIVideo} {
		for _, tc := range []struct {
			name       string
			resolution string
			duration   int
			want       string
			valid      bool
		}{
			{name: "16 seconds", resolution: "720p", duration: 16, want: "720p", valid: true},
			{name: "30 seconds", resolution: "720p", duration: 30, want: "720p", valid: true},
			{name: "the sanity cap itself", resolution: "1080p", duration: 120, want: "1080p", valid: true},
			{name: "above the sanity cap", resolution: "720p", duration: 121, valid: false},
			{name: "zero seconds", resolution: "720p", duration: 0, valid: false},
			{name: "unknown resolution", resolution: "8k", duration: 5, valid: false},
		} {
			resolution, valid := normalizeMediaQuoteVideoSpec(platform, "any-model", tc.resolution, tc.duration)
			require.Equal(t, tc.valid, valid, "%s %s", platform, tc.name)
			require.Equal(t, tc.want, resolution, "%s %s", platform, tc.name)
		}
	}

	// Every other platform keeps the xAI rule it always had.
	for _, platform := range []string{"", PlatformGrok, PlatformSeedance} {
		_, valid := normalizeMediaQuoteVideoSpec(platform, "any-model", "720p", 15)
		require.True(t, valid, "platform=%q", platform)
		_, valid = normalizeMediaQuoteVideoSpec(platform, "any-model", "720p", 16)
		require.False(t, valid, "platform=%q", platform)
	}
}

// The request that failed in production: a Guangying model quoted for 16 seconds.
func TestMediaQuoteServiceQuotesGuangyingBeyondFifteenSeconds(t *testing.T) {
	key := mediaQuoteTestKey()
	key.Group.Platform = PlatformGuangying
	const model = "限时限量-Seedance 2.5 满参"
	price := 19.0
	repo := &mediaQuoteChannelRepo{
		channels: []Channel{{
			ID: 15, Name: "guangying-video", Status: StatusActive, GroupIDs: []int64{*key.GroupID},
			ModelPricing: []ChannelModelPricing{{
				ID: 16, ChannelID: 15, Platform: PlatformGuangying, Models: []string{model},
				BillingMode: BillingModePerRequest, PerRequestPrice: &price,
			}},
		}},
		platforms: map[int64]string{*key.GroupID: PlatformGuangying},
	}
	billing := NewBillingService(&config.Config{}, nil)
	resolver := NewModelPricingResolver(NewChannelService(repo, nil, nil, nil), billing)
	svc := NewMediaQuoteService(&mediaQuoteAPIKeyRepo{key: key}, nil, nil, billing, resolver, &config.Config{})

	quote, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{
		APIKeyID: 7, TaskType: "video", Model: model, Quantity: 1,
		Resolution: "720p", DurationSeconds: 16,
	})

	require.NoError(t, err)
	require.NotNil(t, quote.Pricing.DurationSeconds)
	require.Equal(t, 16, *quote.Pricing.DurationSeconds, "the duration is reported as asked, not clamped")
	require.InDelta(t, price, quote.Pricing.UnitPrice, 0.0000001)
	// Per request: 19 * quantity 1 * the test group's rate multiplier of 2.
	require.InDelta(t, 38, quote.SettlementAmount.Value, 0.0000001)
	require.Equal(t, PricingSourceChannel, quote.Pricing.PricingSource)
	require.Equal(t, string(BillingModePerRequest), quote.Pricing.BillingMode)
}

func TestNormalizeMediaQuoteVideoSpecUsesMaiTokenLimits(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		resolution string
		duration   int
		want       string
		valid      bool
	}{
		{name: "public name at the minimum", model: MaiTokenSeedancePublicModel, resolution: "480p", duration: 4, want: "480p", valid: true},
		{name: "upstream name is accepted too", model: MaiTokenSeedanceModel, resolution: "480p", duration: 10, want: "480p", valid: true},
		{name: "longer than the generic 15 second cap", model: MaiTokenSeedancePublicModel, resolution: "480p", duration: 30, want: "480p", valid: true},
		{name: "resolution may be omitted", model: MaiTokenSeedancePublicModel, resolution: "", duration: 5, want: "480p", valid: true},
		{name: "resolution spelling is normalised", model: MaiTokenSeedancePublicModel, resolution: "480P", duration: 5, want: "480p", valid: true},
		{name: "below the minimum", model: MaiTokenSeedancePublicModel, resolution: "480p", duration: 3, valid: false},
		{name: "above the maximum", model: MaiTokenSeedancePublicModel, resolution: "480p", duration: 31, valid: false},
		{name: "only 480p exists", model: MaiTokenSeedancePublicModel, resolution: "720p", duration: 5, valid: false},
		{name: "unknown model", model: "seedance-2.5", resolution: "480p", duration: 5, valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution, valid := normalizeMediaQuoteVideoSpec(PlatformMaiTokenSeedance, tt.model, tt.resolution, tt.duration)
			require.Equal(t, tt.valid, valid)
			require.Equal(t, tt.want, resolution)
		})
	}
}

// The quote reads the same per-second channel price as the balance hold taken when the
// task is submitted, whichever of the two model names the price was configured under.
func TestMediaQuoteServicePricesMaiTokenPerSecondFromChannel(t *testing.T) {
	for _, configuredModel := range []string{MaiTokenSeedancePublicModel, MaiTokenSeedanceModel} {
		for _, requestedModel := range []string{MaiTokenSeedancePublicModel, MaiTokenSeedanceModel} {
			t.Run("priced as "+configuredModel+" requested as "+requestedModel, func(t *testing.T) {
				key := mediaQuoteTestKey()
				key.Group.Platform = PlatformMaiTokenSeedance
				pricePerSecond := 0.9
				repo := &mediaQuoteChannelRepo{
					channels: []Channel{{
						ID: 13, Name: "mai-video", Status: StatusActive, GroupIDs: []int64{*key.GroupID},
						ModelPricing: []ChannelModelPricing{{
							ID: 14, ChannelID: 13, Platform: PlatformMaiTokenSeedance, Models: []string{configuredModel},
							BillingMode: BillingModeVideo, PerRequestPrice: &pricePerSecond,
						}},
					}},
					platforms: map[int64]string{*key.GroupID: PlatformMaiTokenSeedance},
				}
				billing := NewBillingService(&config.Config{}, nil)
				resolver := NewModelPricingResolver(NewChannelService(repo, nil, nil, nil), billing)
				svc := NewMediaQuoteService(&mediaQuoteAPIKeyRepo{key: key}, nil, nil, billing, resolver, &config.Config{})

				// 20 seconds is beyond the generic 15 second cap on purpose.
				quote, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{
					APIKeyID: 7, TaskType: "video", Model: requestedModel, Quantity: 1,
					Resolution: "480p", DurationSeconds: 20,
				})

				require.NoError(t, err)
				require.InDelta(t, pricePerSecond, quote.Pricing.UnitPrice, 0.0000001)
				// 0.9 per second * 20 seconds * the test group's rate multiplier of 2.
				require.InDelta(t, 36, quote.SettlementAmount.Value, 0.0000001)
				require.InDelta(t, 36, quote.EstimatedCredits, 0.0000001)
				require.Equal(t, "unit_price_per_second * duration_seconds * quantity * rate_multiplier", quote.Pricing.Formula)
				require.Equal(t, string(BillingModeVideo), quote.Pricing.BillingMode)
				require.Equal(t, PricingSourceChannel, quote.Pricing.PricingSource)
				require.Equal(t, "480p", quote.Pricing.Resolution)
			})
		}
	}
}

func TestMediaQuoteServiceMaiTokenRejectsMissingChannelPricingAndBadSpec(t *testing.T) {
	key := mediaQuoteTestKey()
	key.Group.Platform = PlatformMaiTokenSeedance
	svc := newMediaQuoteServiceForTest(key)

	_, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{
		APIKeyID: 7, TaskType: "video", Model: MaiTokenSeedancePublicModel, Quantity: 1,
		Resolution: "480p", DurationSeconds: 5,
	})
	require.ErrorIs(t, err, ErrMediaQuotePricingUnavailable)

	_, err = svc.Quote(context.Background(), 42, MediaQuoteRequest{
		APIKeyID: 7, TaskType: "video", Model: MaiTokenSeedancePublicModel, Quantity: 1,
		Resolution: "480p", DurationSeconds: 31,
	})
	require.ErrorIs(t, err, ErrMediaQuoteInvalidRequest)
}

func TestNormalizeMediaQuoteVideoSpecUsesOPONEModelLimits(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		resolution string
		duration   int
		want       string
		valid      bool
	}{
		{name: "B1 fixed duration", model: "seedance-2.5-b1", resolution: "720P", duration: 30, want: "720p", valid: true},
		{name: "B1 rejects shorter duration", model: "seedance-2.5-b1", resolution: "720p", duration: 29, valid: false},
		{name: "S7 supports 30 seconds", model: "seedance-2.5-s7", resolution: "1080p", duration: 30, want: "1080p", valid: true},
		{name: "S8 supports 4 seconds", model: "seedance-2.5-s8", resolution: "480p", duration: 4, want: "480p", valid: true},
		{name: "Q supports 4K", model: "seedance-2.0-Q", resolution: "4K", duration: 15, want: "4k", valid: true},
		{name: "2.5 Q supports 30 seconds", model: "seedance-2.5-Q", resolution: "1080p", duration: 30, want: "1080p", valid: true},
		{name: "2.5 Qs supports 30 seconds", model: "seedance-2.5-Qs", resolution: "1080p", duration: 30, want: "1080p", valid: true},
		{name: "unknown OPONE model", model: "seedance-retired", resolution: "720p", duration: 5, valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution, valid := normalizeMediaQuoteVideoSpec(PlatformOPONE, tt.model, tt.resolution, tt.duration)
			require.Equal(t, tt.valid, valid)
			require.Equal(t, tt.want, resolution)
		})
	}
}

func TestMediaQuoteServiceSupportsOPONE4KChannelTier(t *testing.T) {
	key := mediaQuoteTestKey()
	key.Group.Platform = PlatformOPONE
	price := 0.5
	repo := &mediaQuoteChannelRepo{
		channels: []Channel{{
			ID: 11, Name: "opone-video", Status: StatusActive, GroupIDs: []int64{*key.GroupID},
			ModelPricing: []ChannelModelPricing{{
				ID: 12, ChannelID: 11, Platform: PlatformOPONE, Models: []string{"seedance-2.0-Q"},
				BillingMode: BillingModeVideo,
				Intervals:   []PricingInterval{{TierLabel: "4k", PerRequestPrice: &price}},
			}},
		}},
		platforms: map[int64]string{*key.GroupID: PlatformOPONE},
	}
	billing := NewBillingService(&config.Config{}, nil)
	resolver := NewModelPricingResolver(NewChannelService(repo, nil, nil, nil), billing)
	svc := NewMediaQuoteService(&mediaQuoteAPIKeyRepo{key: key}, nil, nil, billing, resolver, &config.Config{})

	quote, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{
		APIKeyID: 7, TaskType: "video", Model: "seedance-2.0-Q", Quantity: 2,
		Resolution: "4K", DurationSeconds: 15,
	})

	require.NoError(t, err)
	require.Equal(t, "4k", quote.Pricing.Resolution)
	require.InDelta(t, 30, quote.SettlementAmount.Value, 0.0000001)
	require.InDelta(t, 30, quote.EstimatedCredits, 0.0000001)
}

func TestMediaQuoteServiceImageUsesChannelPricingLikeActualBilling(t *testing.T) {
	key := mediaQuoteTestKey()
	key.Group.ImagePrice1K = nil
	price := 0.3
	repo := &mediaQuoteChannelRepo{
		channels: []Channel{{
			ID: 9, Name: "image-channel", Status: StatusActive, GroupIDs: []int64{*key.GroupID},
			ModelPricing: []ChannelModelPricing{{
				ID: 10, ChannelID: 9, Platform: "openai", Models: []string{"channel-image-model"},
				BillingMode: BillingModeImage,
				Intervals:   []PricingInterval{{TierLabel: "1K", PerRequestPrice: &price}},
			}},
		}},
		platforms: map[int64]string{*key.GroupID: "openai"},
	}
	billing := NewBillingService(&config.Config{}, nil)
	resolver := NewModelPricingResolver(NewChannelService(repo, nil, nil, nil), billing)
	svc := NewMediaQuoteService(&mediaQuoteAPIKeyRepo{key: key}, nil, nil, billing, resolver, &config.Config{})

	quote, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{
		APIKeyID: 7, TaskType: "image", Model: "channel-image-model", Quantity: 2, Resolution: "1K",
	})
	require.NoError(t, err)
	require.InDelta(t, price, quote.Pricing.UnitPrice, 0.0000001)
	require.InDelta(t, 1.2, quote.SettlementAmount.Value, 0.0000001)
	require.InDelta(t, 1.2, quote.EstimatedCredits, 0.0000001)
	require.Equal(t, PricingSourceChannel, quote.Pricing.PricingSource)
	require.Equal(t, string(BillingModeImage), quote.Pricing.BillingMode)
}

func TestMediaQuoteServiceRejectsUnsupportedAndForeignKey(t *testing.T) {
	svc := newMediaQuoteServiceForTest(mediaQuoteTestKey())
	_, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{APIKeyID: 7, TaskType: "text", Model: "m", Quantity: 1})
	require.ErrorIs(t, err, ErrMediaQuoteUnsupported)
	_, err = svc.Quote(context.Background(), 99, MediaQuoteRequest{APIKeyID: 7, TaskType: "image", Model: "m", Quantity: 1, Resolution: "1K"})
	require.ErrorIs(t, err, ErrMediaQuoteAPIKeyNotFound)
	_, err = svc.Quote(context.Background(), 42, MediaQuoteRequest{APIKeyID: 7, TaskType: "video", Model: "m", Quantity: 1, Resolution: "4K", DurationSeconds: 5})
	require.ErrorIs(t, err, ErrMediaQuoteInvalidRequest)
}

func TestMediaQuoteServiceDoesNotApplyOrganizationRatio(t *testing.T) {
	key := mediaQuoteTestKey()
	organizationID := int64(12)
	organizations := &mediaQuoteOrganizationRepo{
		organization: &Organization{ID: organizationID, Balance: 20, DisplayBalance: 1000},
		source:       APIKeyBillingSource{Type: APIKeyBillingSourceOrganization, OrganizationID: &organizationID, Status: APIKeyBillingSourceActive},
	}
	svc := NewMediaQuoteService(&mediaQuoteAPIKeyRepo{key: key}, organizations, nil, NewBillingService(&config.Config{}, nil), nil, &config.Config{})
	quote, err := svc.Quote(context.Background(), 42, MediaQuoteRequest{APIKeyID: 7, TaskType: "image", Model: "image-model", Quantity: 1, Resolution: "1K"})
	require.NoError(t, err)
	require.Equal(t, APIKeyBillingSourceOrganization, quote.BillingSource.Type)
	require.Equal(t, organizationID, *quote.BillingSource.OrganizationID)
	// 组织余额比值为 50，但预估积分只等于价格本身。
	require.InDelta(t, quote.SettlementAmount.Value, quote.EstimatedCredits, 0.0000001)
	require.InDelta(t, 1, quote.Conversion.CreditsPerSettlementUnit, 0.0000001)
}
