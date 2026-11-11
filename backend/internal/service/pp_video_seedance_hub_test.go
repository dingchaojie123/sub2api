package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

var seedanceHubPlatformLetters = map[string]string{
	PlatformCTMOSeedance:     "Z",
	PlatformGuangying:        "G",
	PlatformXingAPISeedance:  "X",
	PlatformMaiTokenSeedance: "M",
	PlatformJijiuSeedance:    "J",
}

// seedanceHubTestResolutions returns the resolutions a line's name must list, when they are
// known from data other than the name itself.
func seedanceHubTestResolutions(t *testing.T, line SeedanceHubLine) ([]string, bool) {
	t.Helper()
	switch line.Platform {
	case PlatformJijiuSeedance:
		spec, ok := jijiuModelSpecFor(line.Model)
		require.True(t, ok, line.Name)
		return spec.Resolutions, true
	case PlatformMaiTokenSeedance:
		return []string{"480p"}, true
	case PlatformCTMOSeedance, PlatformXingAPISeedance:
		require.NotNil(t, line.Limits, line.Name)
		return line.Limits.Resolutions, true
	default:
		return nil, false
	}
}

func seedanceHubPlatformAcceptsModel(platform, model string) bool {
	switch platform {
	case PlatformCTMOSeedance:
		return ppVideoIsCTMOSeedanceModel(model)
	case PlatformGuangying:
		return ppVideoIsGuangyingModel(model)
	case PlatformXingAPISeedance:
		return ppVideoIsXingAPISeedanceModel(model)
	case PlatformMaiTokenSeedance:
		_, ok := ppVideoMaiTokenPublicModel(model)
		return ok
	case PlatformJijiuSeedance:
		_, ok := jijiuModelSpecFor(model)
		return ok
	default:
		return false
	}
}

// seedanceHubTestDuration is a length every line accepts.
func seedanceHubTestDuration(t *testing.T, line SeedanceHubLine) int {
	t.Helper()
	switch line.Platform {
	case PlatformJijiuSeedance:
		spec, ok := jijiuModelSpecFor(line.Model)
		require.True(t, ok, line.Name)
		return spec.MinSeconds
	case PlatformCTMOSeedance, PlatformXingAPISeedance:
		require.NotNil(t, line.Limits, line.Name)
		return line.Limits.MinSeconds
	default:
		return 5
	}
}

func TestSeedanceHubLineTable(t *testing.T) {
	t.Parallel()

	lines := SeedanceHubLines()
	require.Len(t, lines, 24)

	owner := map[string]string{}
	for _, line := range lines {
		require.True(t, IsSeedanceHubMemberPlatform(line.Platform), line.Name)
		letter := seedanceHubPlatformLetters[line.Platform]
		require.NotEmpty(t, letter, line.Name)
		require.True(t, seedanceHubPlatformAcceptsModel(line.Platform, line.Model), "%s: %q is not a model of %s", line.Name, line.Model, line.Platform)

		// Seedance{version}--{resolutions}--{model}--{billing}--{platform letter}
		parts := strings.Split(line.Name, "--")
		require.Len(t, parts, 5, line.Name)
		require.Equal(t, "Seedance"+line.Version, parts[0], line.Name)
		wantBilling := "按秒"
		if line.Billing == BillingModePerRequest {
			wantBilling = "按次"
		}
		require.Equal(t, wantBilling, parts[3], line.Name)
		require.Equal(t, letter, parts[4], line.Name)
		if resolutions, known := seedanceHubTestResolutions(t, line); known {
			require.Equal(t, strings.Join(resolutions, "/"), parts[1], line.Name)
		}

		// Every name a line answers to belongs to that line alone.
		for _, name := range line.allNames() {
			key := seedanceHubKey(name)
			if previous, claimed := owner[key]; claimed {
				require.Equal(t, line.Name, previous, "the name %q is claimed by two lines", name)
			}
			owner[key] = line.Name
		}
	}

	// The jijiu lines are rewritten to the public name of the model, the one clients of that
	// platform already send.
	for _, line := range lines {
		if line.Platform != PlatformJijiuSeedance {
			continue
		}
		spec, ok := jijiuModelSpecFor(line.Model)
		require.True(t, ok, line.Name)
		require.Equal(t, spec.publicName(), line.Model, line.Name)
	}
}

func TestSeedanceHubModelListOrder(t *testing.T) {
	t.Parallel()

	lines := SeedanceHubLines()
	require.Equal(t, seedanceHubVersion25, lines[0].Version)
	require.Equal(t, BillingModePerRequest, lines[0].Billing)
	for i := 1; i < len(lines); i++ {
		require.LessOrEqual(t, lines[i-1].rank(), lines[i].rank(), "%s before %s", lines[i-1].Name, lines[i].Name)
	}

	// Listed once although two names are given for the same line; names that are not a line
	// are dropped; the order is 2.5 per request, 2.5 per second, then 2.0 per request.
	got := SeedanceHubPublicModels([]string{
		"seedance-2.0-930",
		"Seedance2.0--720p--930--按次--Z",
		MaiTokenSeedanceModel,
		"gpt-4o",
		"seedance2.5-V",
	})
	require.Equal(t, []string{
		"Seedance2.5--720p--V--按次--X",
		"Seedance2.5--480p--全参--按秒--M",
		"Seedance2.0--720p--930--按次--Z",
	}, got)
	require.Empty(t, SeedanceHubPublicModels(nil))
}

func TestSeedanceHubLineLookup(t *testing.T) {
	t.Parallel()

	line, ok := SeedanceHubLineByName(" seedance2.0--720p--930--按次--z ")
	require.True(t, ok)
	require.Equal(t, "seedance-2.0-930", line.Model)

	// A platform's own name finds the same line, and only that platform serves it.
	line, ok = SeedanceHubLineFor(PlatformCTMOSeedance, "SEEDANCE-2.0-930")
	require.True(t, ok)
	require.Equal(t, "Seedance2.0--720p--930--按次--Z", line.Name)
	_, ok = SeedanceHubLineFor(PlatformXingAPISeedance, "seedance-2.0-930")
	require.False(t, ok)

	// Names the platforms already accept besides their public name: Mai Token's upstream name,
	// the upstream ids and old names of the guangying lines, the upstream names of the jijiu lines.
	for _, name := range []string{MaiTokenSeedanceModel, "gy-model-31", "Seedance 2.5-全参720P 按秒计费", "SD2.5特价30-10线路二"} {
		_, ok = SeedanceHubLineByName(name)
		require.True(t, ok, name)
	}
	_, ok = SeedanceHubLineByName("")
	require.False(t, ok)
	_, ok = SeedanceHubLineByName("seedance-2.0-max") // a discontinued model is not a line
	require.False(t, ok)
}

func TestResolveSeedanceHubRequest(t *testing.T) {
	t.Parallel()

	line, body, err := ResolveSeedanceHubRequest([]byte(`{"model":"Seedance2.0--720p--930--按次--Z","prompt":"p","duration":5,"images":["https://a/1.png"]}`))
	require.NoError(t, err)
	require.Equal(t, PlatformCTMOSeedance, line.Platform)
	require.Equal(t, "seedance-2.0-930", gjson.GetBytes(body, "model").String())
	require.Equal(t, "p", gjson.GetBytes(body, "prompt").String())
	require.Equal(t, int64(5), gjson.GetBytes(body, "duration").Int())
	require.Equal(t, "https://a/1.png", gjson.GetBytes(body, "images.0").String())

	// The platform's own name is a pinned line and is rewritten to its canonical spelling.
	line, body, err = ResolveSeedanceHubRequest([]byte(`{"model":"SEEDANCE-2.0-930","prompt":"p"}`))
	require.NoError(t, err)
	require.Equal(t, "Seedance2.0--720p--930--按次--Z", line.Name)
	require.Equal(t, "seedance-2.0-930", gjson.GetBytes(body, "model").String())

	_, _, err = ResolveSeedanceHubRequest([]byte(`{"prompt":"p"}`))
	require.ErrorContains(t, err, "model is required")
	_, _, err = ResolveSeedanceHubRequest([]byte(`{"model":"  "}`))
	require.ErrorContains(t, err, "model is required")
	_, _, err = ResolveSeedanceHubRequest([]byte(`{"model":"gpt-4o"}`))
	require.ErrorContains(t, err, "is not available in this group")
	_, _, err = ResolveSeedanceHubRequest([]byte(`not json`))
	require.ErrorContains(t, err, "valid JSON")
	_, _, err = ResolveSeedanceHubRequest(nil)
	require.Error(t, err)
}

// Every line runs through its platform's own v1 and v2 pipeline once its model is rewritten.
func TestSeedanceHubLinesRunThroughTheirPlatformPipeline(t *testing.T) {
	t.Parallel()

	for _, line := range SeedanceHubLines() {
		quoted, err := json.Marshal(line.Name)
		require.NoError(t, err)
		request := []byte(fmt.Sprintf(`{"model":%s,"prompt":"p","duration":%d}`, quoted, seedanceHubTestDuration(t, line)))

		resolved, body, err := ResolveSeedanceHubRequest(request)
		require.NoError(t, err, line.Name)
		require.Equal(t, line.Name, resolved.Name)

		// v1: the native body goes straight to the platform's request preparation.
		prepared, _, err := PreparePPVideoRequestBody(resolved.Platform, PPVideoOperationGeneric, body)
		require.NoError(t, err, line.Name)
		require.NotEmpty(t, prepared, line.Name)

		// v2: the unified body is translated for the platform first, then prepared the same way.
		require.True(t, IsPPVideoUnifiedPlatform(resolved.Platform), line.Name)
		translated, err := TranslatePPVideoUnifiedRequest(resolved.Platform, body)
		require.NoError(t, err, line.Name)
		_, _, err = PreparePPVideoRequestBody(resolved.Platform, PPVideoOperationGeneric, translated)
		require.NoError(t, err, line.Name)
	}
}

func TestSeedanceHubPlatformHelpers(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoGroupPlatform(PlatformSeedanceHub))
	require.False(t, IsPPVideoPlatform(PlatformSeedanceHub), "the aggregate group is a group platform, not an account platform")
	require.True(t, IsPPVideoGroupPlatform(PlatformGuangying))
	require.False(t, IsPPVideoGroupPlatform(PlatformOpenAI))

	for _, platform := range seedanceHubMemberPlatforms {
		require.True(t, IsSeedanceHubMemberPlatform(platform), platform)
		require.True(t, PPVideoGroupServesPlatform(PlatformSeedanceHub, platform), platform)
		require.True(t, PPVideoGroupServesPlatform(platform, platform), platform)
	}
	require.False(t, PPVideoGroupServesPlatform(PlatformSeedanceHub, PlatformOPONE))
	require.False(t, PPVideoGroupServesPlatform(PlatformGuangying, PlatformJijiuSeedance))
	require.False(t, IsSeedanceHubMemberPlatform(PlatformOPONE))
	require.True(t, IsAllowedQuotaPlatform(PlatformSeedanceHub))
}

func TestSeedanceHubPricesLinesUnderTheirPublicNames(t *testing.T) {
	t.Parallel()

	const (
		name930  = "Seedance2.0--720p--930--按次--Z"
		nameMai  = "Seedance2.5--480p--全参--按秒--M"
		nameD    = "Seedance2.5--480p/720p/1080p--全参D--按秒--J"
		modelD   = "稳定seedance-2.5全参D"
		platform = PlatformSeedanceHub
	)
	groupID := int64(520)
	perRequest, perSecond := 6.0, 0.9
	baseD, tier480D := 0.3, 0.2
	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
		resolver: newPPVideoResolverWithChannel(t, groupID, platform, []ChannelModelPricing{
			{Platform: platform, Models: []string{name930}, BillingMode: BillingModePerRequest, PerRequestPrice: &perRequest},
			{Platform: platform, Models: []string{nameMai}, BillingMode: BillingModeVideo, PerRequestPrice: &perSecond},
			{
				Platform: platform, Models: []string{nameD}, BillingMode: BillingModeVideo, PerRequestPrice: &baseD,
				Intervals: []PricingInterval{{TierLabel: "480p", PerRequestPrice: &tier480D}},
			},
		}),
	}
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{
		Platform:             platform,
		RateMultiplier:       1,
		VideoRateIndependent: true,
		VideoRateMultiplier:  1,
	}}
	cost := func(metaPlatform, model string, seconds int64, resolution string) (*CostBreakdown, error) {
		return svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
			Platform:                      metaPlatform,
			Model:                         model,
			RequestedDurationMilliseconds: seconds * 1000,
			RequestedDurationSeconds:      int(seconds),
			VideoCount:                    1,
			VideoResolution:               resolution,
		})
	}

	// A submission is metered as the platform's request, which carries the platform's own name
	// of the model; the price is the one configured under the line's public name. The handler
	// also passes the line's public name itself (so an account that maps the line to another
	// upstream model is still priced as the line the client asked for).
	for _, model := range []string{"seedance-2.0-930", name930} {
		got, err := cost(PlatformCTMOSeedance, model, 5, "720p")
		require.NoError(t, err, model)
		require.Equal(t, PPVideoBillingFormulaPerRequest, got.BillingFormula, model)
		require.InDelta(t, 6, got.TotalCost, 1e-9, model)
	}

	got, err := cost(PlatformMaiTokenSeedance, MaiTokenSeedanceModel, 10, "480p")
	require.NoError(t, err)
	require.Equal(t, PPVideoBillingFormulaPerSecond, got.BillingFormula)
	require.InDelta(t, 9, got.TotalCost, 1e-9)

	// Resolution tiers apply to the lines of the group.
	got, err = cost(PlatformJijiuSeedance, modelD, 10, "480p")
	require.NoError(t, err)
	require.InDelta(t, 2, got.TotalCost, 1e-9)
	got, err = cost(PlatformJijiuSeedance, modelD, 10, "720p")
	require.NoError(t, err)
	require.InDelta(t, 3, got.TotalCost, 1e-9)

	// A price estimate has no account: it carries the group's platform and either name.
	got, err = cost(PlatformSeedanceHub, name930, 5, "720p")
	require.NoError(t, err)
	require.InDelta(t, 6, got.TotalCost, 1e-9)
	got, err = cost(PlatformSeedanceHub, "seedance-2.0-930", 5, "720p")
	require.NoError(t, err)
	require.InDelta(t, 6, got.TotalCost, 1e-9)

	// A line with no price of its own is refused, and the message names the group.
	_, err = cost(PlatformCTMOSeedance, "seedance-2.0-903", 5, "720p")
	require.ErrorContains(t, err, "Seedance Hub")
	// A model that is not served by the platform that ran the request is not a line of the group.
	_, err = cost(PlatformCTMOSeedance, MaiTokenSeedanceModel, 5, "480p")
	require.ErrorContains(t, err, "not a line of this group")
	_, err = cost(PlatformCTMOSeedance, "seedance-2.0-max", 5, "720p")
	require.ErrorContains(t, err, "not a line of this group")
}

func TestSeedanceHubPriceIsNotTakenFromAnotherPlatformsEntry(t *testing.T) {
	t.Parallel()

	groupID := int64(521)
	price := 6.0
	const lineName = "Seedance2.0--720p--930--按次--Z"
	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
		// The entry is configured under the line's own public name, but for the platform's
		// own pricing platform instead of the hub: only that mismatch can refuse the price.
		resolver: newPPVideoResolverWithChannel(t, groupID, PlatformSeedanceHub, []ChannelModelPricing{
			{Platform: PlatformCTMOSeedance, Models: []string{lineName}, BillingMode: BillingModePerRequest, PerRequestPrice: &price},
		}),
	}
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{Platform: PlatformSeedanceHub, RateMultiplier: 1, VideoRateIndependent: true, VideoRateMultiplier: 1}}
	for _, model := range []string{lineName, "seedance-2.0-930"} {
		_, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
			Platform: PlatformCTMOSeedance, Model: model, RequestedDurationMilliseconds: 5000, RequestedDurationSeconds: 5, VideoCount: 1,
		})
		require.Error(t, err, model)
	}
}

func TestSeedanceHubMediaQuoteSpec(t *testing.T) {
	t.Parallel()

	// A line with documented limits is estimated with them.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.0--720p--930--按次--Z", "", 5)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.0--720p--930--按次--Z", "1080p", 5)
	require.False(t, ok, "resolution the line does not offer")
	// The same spellings the other platforms accept mean the same resolution here.
	for _, spelling := range []string{"720", "720P", "hd"} {
		resolution, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.0--720p--930--按次--Z", spelling, 5)
		require.True(t, ok, spelling)
		require.Equal(t, "720p", resolution, spelling)
	}
	_, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.0--720p--930--按次--Z", "720p", 4)
	require.False(t, ok, "below the line's shortest length")
	_, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.0--720p--930--按次--Z", "720p", 16)
	require.False(t, ok, "above the line's longest length")
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "seedance2.5-V", "", 30)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)

	// A line without its own limits is estimated with the limits of its platform.
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.5--480p--全参--按秒--M", "", 30)
	require.True(t, ok)
	require.Equal(t, VideoBillingResolution480P, resolution)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.5--480p--全参--按秒--M", "720p", 10)
	require.False(t, ok)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.0--720p--903z--按次--J", "", 5)
	require.False(t, ok, "903z is at least 10 seconds")

	_, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "gpt-4o", "", 5)
	require.False(t, ok)
}

func TestSeedanceHubCapabilities(t *testing.T) {
	t.Parallel()

	// 浙创 and 星光 lines carry their documented limits.
	c := PPVideoCapabilitiesForModel(PlatformSeedanceHub, "Seedance2.0--720p--930--按次--Z")
	require.Equal(t, []string{"v1", "v2"}, c.Protocols)
	require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 15}, c.Duration)
	require.Equal(t, []string{"720p"}, c.Resolutions)
	require.Equal(t, 9, *c.Assets.ReferenceImages)
	require.Equal(t, 3, *c.Assets.ReferenceVideos)
	require.Equal(t, 0, *c.Assets.ReferenceAudios)

	c = PPVideoCapabilitiesForModel(PlatformSeedanceHub, "Seedance2.5--720p--V--按次--X")
	require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 30}, c.Duration)
	require.Equal(t, 30, *c.Assets.ReferenceImages)
	require.Equal(t, 10, *c.Assets.ReferenceVideos)
	require.Equal(t, 10, *c.Assets.ReferenceAudios)

	// Mai Token and jijiu lines report what their platform's own limit table says.
	require.Equal(t,
		PPVideoCapabilitiesForModel(PlatformMaiTokenSeedance, MaiTokenSeedancePublicModel),
		PPVideoCapabilitiesForModel(PlatformSeedanceHub, "Seedance2.5--480p--全参--按秒--M"))
	spec, ok := jijiuModelSpecFor("SD2.0特价稳定903z")
	require.True(t, ok)
	j := PPVideoCapabilitiesForModel(PlatformSeedanceHub, "Seedance2.0--720p--903z--按次--J")
	require.Equal(t, &PPVideoDurationRange{Min: spec.MinSeconds, Max: spec.MaxSeconds}, j.Duration)

	// A line whose platform has no limit data, and a name that is not a line, only report protocols.
	for _, model := range []string{"Seedance2.5--720p--全参--按秒--G", "not-a-line"} {
		c = PPVideoCapabilitiesForModel(PlatformSeedanceHub, model)
		require.Equal(t, []string{"v1", "v2"}, c.Protocols, model)
		require.Nil(t, c.Duration, model)
		require.Nil(t, c.Assets, model)
	}
}

func TestTranslatePPVideoUnifiedRequestToCTMO(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformCTMOSeedance, []byte(`{
		"model": "seedance-2.0-930",
		"prompt": "人物微笑挥手",
		"duration": 5,
		"resolution": "720p",
		"ratio": "9:16",
		"assets": [
			{"type": "image", "name": "角色.png", "url": "https://cdn.example.com/a.png"},
			{"type": "image", "url": "https://cdn.example.com/b.png"},
			{"type": "video", "url": "https://cdn.example.com/c.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/d.mp3"}
		]
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "seedance-2.0-930",
		"prompt": "人物微笑挥手",
		"duration": 5,
		"resolution": "720p",
		"ratio": "9:16",
		"images": ["https://cdn.example.com/a.png", "https://cdn.example.com/b.png"],
		"videos": ["https://cdn.example.com/c.mp4"],
		"audios": ["https://cdn.example.com/d.mp3"]
	}`, string(translated))

	// The translated request behaves exactly like the equivalent native one.
	requireUnifiedMatchesNative(t, PlatformCTMOSeedance,
		`{"model":"seedance-2.0-930","prompt":"p","duration":5,"resolution":"720p","ratio":"9:16",
		  "assets":[{"type":"image","url":"https://cdn.example.com/a.png"},{"type":"video","url":"https://cdn.example.com/c.mp4"}]}`,
		`{"model":"seedance-2.0-930","prompt":"p","duration":5,"resolution":"720p","ratio":"9:16",
		  "images":["https://cdn.example.com/a.png"],"videos":["https://cdn.example.com/c.mp4"]}`)

	// First/last-frame mode.
	requireUnifiedMatchesNative(t, PlatformCTMOSeedance,
		`{"model":"seedance-2.0-930","prompt":"p","duration":5,
		  "assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/1.png"},{"type":"image","role":"last_frame","url":"https://cdn.example.com/2.png"}]}`,
		`{"model":"seedance-2.0-930","prompt":"p","duration":5,
		  "start_frame_url":"https://cdn.example.com/1.png","end_frame_url":"https://cdn.example.com/2.png"}`)

	for _, tc := range []struct{ name, body, want string }{
		{"audio switch", `{"model":"seedance-2.0-930","prompt":"p","generate_audio":true}`, "generate_audio"},
		{"seed", `{"model":"seedance-2.0-930","prompt":"p","seed":1}`, "seed"},
		{"callback", `{"model":"seedance-2.0-930","prompt":"p","callback_url":"https://example.com/cb"}`, "callback_url"},
		{"option", `{"model":"seedance-2.0-930","prompt":"p","options":{"x":1}}`, "options.x"},
		{"no model", `{"prompt":"p"}`, "model"},
		{"nothing to generate from", `{"model":"seedance-2.0-930"}`, "prompt"},
		{"one frame", `{"model":"seedance-2.0-930","prompt":"p","assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/1.png"}]}`, "together"},
		{"frames with references", `{"model":"seedance-2.0-930","prompt":"p","assets":[
			{"type":"image","role":"first_frame","url":"https://cdn.example.com/1.png"},
			{"type":"image","role":"last_frame","url":"https://cdn.example.com/2.png"},
			{"type":"image","url":"https://cdn.example.com/3.png"}]}`, "cannot be combined"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformCTMOSeedance, []byte(tc.body))
		require.Error(t, err, tc.name)
		require.ErrorContains(t, err, tc.want, tc.name)
	}

	// The aggregate group is never translated as a platform of its own.
	_, err = TranslatePPVideoUnifiedRequest(PlatformSeedanceHub, []byte(`{"model":"x","prompt":"p"}`))
	require.Error(t, err)
}
