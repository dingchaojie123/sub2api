package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	MediaQuoteTaskImage = "image"
	MediaQuoteTaskVideo = "video"
)

var (
	ErrMediaQuoteInvalidRequest     = infraerrors.BadRequest("INVALID_MEDIA_QUOTE_REQUEST", "invalid media quote request")
	ErrMediaQuoteUnsupported        = infraerrors.BadRequest("UNSUPPORTED_MEDIA_QUOTE_TASK", "only image and video quotes are supported")
	ErrMediaQuoteAPIKeyNotFound     = infraerrors.NotFound("MEDIA_QUOTE_API_KEY_NOT_FOUND", "API key not found")
	ErrMediaQuoteBillingSource      = infraerrors.Conflict("MEDIA_QUOTE_BILLING_SOURCE_UNAVAILABLE", "billing source is unavailable")
	ErrMediaQuoteNotApplicable      = infraerrors.Conflict("MEDIA_QUOTE_BILLING_NOT_APPLICABLE", "media credit quote is not applicable to this billing mode")
	ErrMediaQuotePricingUnavailable = infraerrors.Conflict("MEDIA_QUOTE_PRICING_UNAVAILABLE", "media pricing is unavailable")
)

type MediaQuoteRequest struct {
	APIKeyID        int64  `json:"api_key_id"`
	TaskType        string `json:"task_type"`
	Model           string `json:"model"`
	Quantity        int    `json:"quantity"`
	Resolution      string `json:"resolution"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}

type MediaQuoteMoney struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type MediaQuoteBillingSource struct {
	Type           string `json:"type"`
	OrganizationID *int64 `json:"organization_id,omitempty"`
}

type MediaQuoteConversionSnapshot struct {
	CreditsPerSettlementUnit float64 `json:"credits_per_settlement_unit"`
}

type MediaQuotePricingSnapshot struct {
	UnitPrice       float64 `json:"unit_price"`
	RateMultiplier  float64 `json:"rate_multiplier"`
	Quantity        int     `json:"quantity"`
	Resolution      string  `json:"resolution"`
	DurationSeconds *int    `json:"duration_seconds,omitempty"`
	Formula         string  `json:"formula"`
	PricingSource   string  `json:"pricing_source"`
	BillingMode     string  `json:"billing_mode"`
}

type MediaQuote struct {
	QuoteID          string                       `json:"quote_id"`
	Status           string                       `json:"status"`
	TaskType         string                       `json:"task_type"`
	Model            string                       `json:"model"`
	BillingSource    MediaQuoteBillingSource      `json:"billing_source"`
	SettlementAmount MediaQuoteMoney              `json:"settlement_amount"`
	EstimatedCredits float64                      `json:"estimated_credits"`
	CreditUnit       string                       `json:"credit_unit"`
	Conversion       MediaQuoteConversionSnapshot `json:"conversion_snapshot"`
	Pricing          MediaQuotePricingSnapshot    `json:"pricing_snapshot"`
	ExpiresAt        time.Time                    `json:"expires_at"`
}

type MediaQuoteService struct {
	apiKeys         APIKeyRepository
	organizations   OrganizationRepository
	userGroupRates  UserGroupRateRepository
	billing         *BillingService
	pricingResolver *ModelPricingResolver
	cfg             *config.Config
	now             func() time.Time
}

func NewMediaQuoteService(apiKeys APIKeyRepository, organizations OrganizationRepository, userGroupRates UserGroupRateRepository, billing *BillingService, pricingResolver *ModelPricingResolver, cfg *config.Config) *MediaQuoteService {
	return &MediaQuoteService{apiKeys: apiKeys, organizations: organizations, userGroupRates: userGroupRates, billing: billing, pricingResolver: pricingResolver, cfg: cfg, now: time.Now}
}

func (s *MediaQuoteService) Quote(ctx context.Context, userID int64, in MediaQuoteRequest) (*MediaQuote, error) {
	in.TaskType = strings.ToLower(strings.TrimSpace(in.TaskType))
	in.Model = strings.TrimSpace(in.Model)
	if in.APIKeyID <= 0 || in.Model == "" || in.Quantity <= 0 || in.Quantity > 1000 {
		return nil, ErrMediaQuoteInvalidRequest
	}
	if in.TaskType != MediaQuoteTaskImage && in.TaskType != MediaQuoteTaskVideo {
		return nil, ErrMediaQuoteUnsupported
	}

	apiKey, err := s.apiKeys.GetByID(ctx, in.APIKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != userID {
		return nil, ErrMediaQuoteAPIKeyNotFound
	}
	if apiKey.Status != StatusAPIKeyActive || apiKey.Group == nil || !apiKey.Group.IsActive() {
		return nil, ErrMediaQuoteBillingSource
	}
	if apiKey.User == nil || apiKey.Group.IsSubscriptionType() || s.billing == nil {
		return nil, ErrMediaQuoteNotApplicable
	}

	baseMultiplier := 1.0
	if s.cfg != nil && s.cfg.Default.RateMultiplier >= 0 {
		baseMultiplier = s.cfg.Default.RateMultiplier
	}
	if apiKey.GroupID != nil {
		baseMultiplier = apiKey.Group.RateMultiplier
		if s.userGroupRates != nil {
			userRate, rateErr := s.userGroupRates.GetByUserAndGroup(ctx, userID, *apiKey.GroupID)
			if rateErr != nil {
				return nil, rateErr
			}
			if userRate != nil {
				baseMultiplier = *userRate
			}
		}
	}

	resolution := ""
	durationSeconds := (*int)(nil)
	multiplier := baseMultiplier
	var cost *CostBreakdown
	pricingSource := "default"
	unitPrice := 0.0
	if in.TaskType == MediaQuoteTaskImage {
		if _, ok := ClassifyImageBillingTier(in.Resolution); !ok {
			return nil, ErrMediaQuoteInvalidRequest
		}
		resolution = NormalizeImageBillingTierOrDefault(in.Resolution)
		multiplier = resolveImageRateMultiplier(apiKey, baseMultiplier)
		groupConfig := imagePriceConfigFromAPIKey(apiKey)
		cost = s.billing.CalculateImageCost(in.Model, resolution, in.Quantity, groupConfig, multiplier)
		if apiKeyHasConfiguredImagePrice(apiKey, resolution) {
			pricingSource = "group"
		} else if resolved := s.resolveChannelImagePricing(ctx, apiKey, in.Model); resolved != nil {
			resolvedCost, resolveErr := s.billing.CalculateCostUnified(CostInput{
				Ctx:            ctx,
				Model:          in.Model,
				GroupID:        apiKey.GroupID,
				RequestCount:   in.Quantity,
				SizeTier:       resolution,
				RateMultiplier: multiplier,
				Resolver:       s.pricingResolver,
				Resolved:       resolved,
			})
			// The actual usage path falls back to the established media rate card
			// when a channel pricing entry cannot be calculated.
			if resolveErr == nil && resolvedCost != nil {
				cost = resolvedCost
				pricingSource = resolved.Source
			}
		}
		unitPrice = cost.TotalCost / float64(in.Quantity)
	} else {
		var valid bool
		resolution, valid = normalizeMediaQuoteVideoSpec(
			apiKey.Group.Platform,
			in.Model,
			in.Resolution,
			in.DurationSeconds,
		)
		if !valid {
			return nil, ErrMediaQuoteInvalidRequest
		}
		duration := in.DurationSeconds
		durationSeconds = &duration
		multiplier = resolveVideoRateMultiplier(apiKey, baseMultiplier)
		channelQuote, channelPriced, quoteErr := s.resolveChannelVideoQuote(ctx, apiKey, in, resolution)
		if quoteErr != nil {
			return nil, ErrMediaQuotePricingUnavailable.WithCause(quoteErr)
		}
		if channelPriced {
			billingMode := BillingModeVideo
			if channelQuote.Formula == PPVideoBillingFormulaPerRequest {
				billingMode = BillingModePerRequest
			}
			cost = &CostBreakdown{
				TotalCost:   channelQuote.Cost,
				OutputCost:  channelQuote.Cost,
				ActualCost:  channelQuote.Cost * multiplier,
				BillingMode: string(billingMode),
			}
			unitPrice = channelQuote.UnitPrice
			pricingSource = PricingSourceChannel
		} else {
			cost = s.billing.CalculateVideoCost(in.Model, resolution, in.Quantity, duration, videoPriceConfigFromAPIKey(apiKey), multiplier)
			unitPrice = cost.TotalCost / (float64(in.Quantity) * float64(duration))
			if apiKeyHasConfiguredVideoPrice(apiKey, resolution) {
				pricingSource = "group"
			}
		}
	}

	billingSource, conversion, err := s.resolveConversion(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	quote := &MediaQuote{
		Status:           "estimate",
		TaskType:         in.TaskType,
		Model:            in.Model,
		BillingSource:    billingSource,
		SettlementAmount: MediaQuoteMoney{Value: cost.ActualCost, Currency: "USD"},
		EstimatedCredits: cost.ActualCost * conversion.CreditsPerSettlementUnit,
		CreditUnit:       "credit",
		Conversion:       conversion,
		Pricing: MediaQuotePricingSnapshot{
			UnitPrice: unitPrice, RateMultiplier: multiplier, Quantity: in.Quantity,
			Resolution: resolution, DurationSeconds: durationSeconds,
			Formula: mediaQuoteFormula(in.TaskType, BillingMode(cost.BillingMode)), PricingSource: pricingSource,
			BillingMode: cost.BillingMode,
		},
		ExpiresAt: now.Add(5 * time.Minute),
	}
	quote.QuoteID = buildMediaQuoteID(userID, in.APIKeyID, quote, now)
	return quote, nil
}

// resolveChannelVideoQuote mirrors the provider-specific pricing path used when
// the video task is submitted. These platforms require model-level channel
// pricing; falling back to the legacy group rate card would produce a quote
// that cannot match the eventual balance hold.
func (s *MediaQuoteService) resolveChannelVideoQuote(
	ctx context.Context,
	apiKey *APIKey,
	in MediaQuoteRequest,
	resolution string,
) (PPVideoBillingQuote, bool, error) {
	if apiKey == nil || apiKey.Group == nil {
		return PPVideoBillingQuote{}, false, nil
	}
	platform := strings.TrimSpace(apiKey.Group.Platform)
	if platform != Platform88APIVideo && platform != PlatformCTMOSeedance && platform != PlatformXingAPISeedance && platform != PlatformGuangying && platform != PlatformOPONE {
		return PPVideoBillingQuote{}, false, nil
	}

	gateway := &OpenAIGatewayService{resolver: s.pricingResolver, billingService: s.billing}
	meta := PPVideoBillingMetadata{
		Platform:                      platform,
		Model:                         in.Model,
		RequestedDurationMilliseconds: int64(in.DurationSeconds) * 1000,
		RequestedDurationSeconds:      in.DurationSeconds,
		VideoCount:                    in.Quantity,
		VideoResolution:               resolution,
	}
	if platform == Platform88APIVideo {
		quote, err := gateway.calculate88APIVideoCost(ctx, meta, apiKey)
		return quote, true, err
	}
	quote, err := gateway.calculateCTMOSeedanceCost(ctx, meta, apiKey)
	return quote, true, err
}

func (s *MediaQuoteService) resolveChannelImagePricing(ctx context.Context, apiKey *APIKey, model string) *ResolvedPricing {
	if s == nil || s.pricingResolver == nil || apiKey == nil || apiKey.GroupID == nil {
		return nil
	}
	resolved := s.pricingResolver.Resolve(ctx, PricingInput{Model: model, GroupID: apiKey.GroupID})
	if resolved == nil || resolved.Source != PricingSourceChannel ||
		(resolved.Mode != BillingModePerRequest && resolved.Mode != BillingModeImage) {
		return nil
	}
	return resolved
}

// resolveConversion 解析计费来源（个人 / 组织）。
// 价格本身即积分价：预估积分等于结算金额，实际扣费时积分余额也只扣这个数，
// 与账号的充值档位无关，所以换算系数恒为 1。
func (s *MediaQuoteService) resolveConversion(ctx context.Context, apiKey *APIKey) (MediaQuoteBillingSource, MediaQuoteConversionSnapshot, error) {
	source := MediaQuoteBillingSource{Type: APIKeyBillingSourcePersonal}
	conversion := MediaQuoteConversionSnapshot{CreditsPerSettlementUnit: 1}
	var billingSource *APIKeyBillingSource
	if s.organizations != nil {
		sources, err := s.organizations.GetAPIKeyBillingSources(ctx, []int64{apiKey.ID})
		if err != nil {
			return source, MediaQuoteConversionSnapshot{}, err
		}
		if resolved, ok := sources[apiKey.ID]; ok {
			billingSource = &resolved
		}
	}
	if billingSource != nil && billingSource.Type == APIKeyBillingSourceOrganization {
		if billingSource.Status != APIKeyBillingSourceActive || billingSource.OrganizationID == nil {
			return source, MediaQuoteConversionSnapshot{}, ErrMediaQuoteBillingSource
		}
		if _, err := s.organizations.GetForUser(ctx, *billingSource.OrganizationID, apiKey.UserID); err != nil {
			return source, MediaQuoteConversionSnapshot{}, ErrMediaQuoteBillingSource
		}
		source.Type = APIKeyBillingSourceOrganization
		source.OrganizationID = billingSource.OrganizationID
	}
	return source, conversion, nil
}

func isSupportedVideoQuoteResolution(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480", "480p", "sd", "720", "720p", "hd", "1080", "1080p", "full_hd", "full-hd", "fhd":
		return true
	default:
		return false
	}
}

func normalizeMediaQuoteVideoSpec(platform, model, resolution string, durationSeconds int) (string, bool) {
	if strings.TrimSpace(platform) == PlatformOPONE {
		canonicalModel, ok := oponeCanonicalSeedanceModel(model)
		if !ok {
			return "", false
		}
		normalizedResolution := normalizePPVideoResolution(resolution)
		minSeconds, maxSeconds, _, _ := oponeDurationSpec(canonicalModel)
		if durationSeconds < minSeconds || durationSeconds > maxSeconds ||
			!oponeResolutionAllowed(canonicalModel, normalizedResolution) {
			return "", false
		}
		return normalizedResolution, true
	}
	if !isSupportedVideoQuoteResolution(resolution) ||
		durationSeconds < VideoBillingMinDurationSeconds ||
		durationSeconds > VideoBillingMaxDurationSeconds {
		return "", false
	}
	return NormalizeVideoBillingResolutionOrDefault(resolution), true
}

func mediaQuoteFormula(taskType string, billingMode BillingMode) string {
	if taskType == MediaQuoteTaskVideo {
		if billingMode == BillingModePerRequest {
			return "unit_price_per_request * quantity * rate_multiplier"
		}
		return "unit_price_per_second * duration_seconds * quantity * rate_multiplier"
	}
	return "unit_price_per_image * quantity * rate_multiplier"
}

func buildMediaQuoteID(userID, apiKeyID int64, quote *MediaQuote, now time.Time) string {
	payload, _ := json.Marshal(struct {
		UserID   int64
		APIKeyID int64
		Quote    *MediaQuote
		IssuedAt int64
	}{userID, apiKeyID, quote, now.Unix()})
	sum := sha256.Sum256(payload)
	return "quote_" + hex.EncodeToString(sum[:12])
}
