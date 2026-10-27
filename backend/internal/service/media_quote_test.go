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
	require.InDelta(t, 12, quote.EstimatedCredits, 0.0000001)
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
	require.InDelta(t, 20, quote.EstimatedCredits, 0.0000001)
	require.NotNil(t, quote.Pricing.DurationSeconds)
	require.Equal(t, 5, *quote.Pricing.DurationSeconds)
	require.Equal(t, "group", quote.Pricing.PricingSource)
	require.Equal(t, string(BillingModeVideo), quote.Pricing.BillingMode)
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
	require.InDelta(t, 12, quote.EstimatedCredits, 0.0000001)
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

func TestMediaQuoteServiceUsesOrganizationDisplayConversion(t *testing.T) {
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
	require.InDelta(t, 20, quote.EstimatedCredits, 0.0000001)
}
