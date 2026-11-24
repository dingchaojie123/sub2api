package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	// Upstream ids and the public names clients see.
	xmTestFace       = "seedance_2.5"
	xmTestVIP        = "bd-VIP-SD2.5满参真人脸-720P-1"
	xmTestFull       = "SD2.5-M-满参支持人脸"
	xmTestFacePublic = "Seedance2.5-XM-stable-face"
	xmTestVIPPublic  = "Seedance2.5-XM-720p-vip"
	xmTestFullPublic = "Seedance2.5-XM-M-full"
	// seedance_2.0 (added 2026-11-19).
	xmTest20       = "seedance_2.0"
	xmTest20Public = "Seedance2.0-XM"
	// seedance_2.5-flash (added 2026-11-19).
	xmTestFlash       = "seedance_2.5-flash"
	xmTestFlashPublic = "Seedance2.5-XM-flash"
	// SD2.5-X-满参支持人脸 (added 2026-11-24).
	xmTestX       = "SD2.5-X-满参支持人脸"
	xmTestXPublic = "Seedance2.5-XM-X-full"
)

func TestXMSeedanceModelTable(t *testing.T) {
	t.Parallel()

	// The three models the operator asked for on 2026-11-17, in the order of the request, and the
	// seedance_2.0 and seedance_2.5-flash models added on 2026-11-19.
	require.Equal(t, []string{xmTestFacePublic, xmTestVIPPublic, xmTestFullPublic, xmTest20Public, xmTestFlashPublic, xmTestXPublic}, xmModelNames())
	require.Len(t, xmSeedanceModels, 6)

	seen := make(map[string]string)
	for _, spec := range xmSeedanceModels {
		// A row answers to its upstream id and its public name; no name may belong to two rows.
		for _, name := range []string{spec.Name, spec.publicName()} {
			key := strings.ToLower(name)
			owner, dup := seen[key]
			require.False(t, dup, "name %q is used by %q and %q", name, owner, spec.Name)
			seen[key] = spec.Name
		}
		// Clients pick video models by name: every public name has to say seedance, and none of
		// them carries the upstream's host name.
		require.Contains(t, strings.ToLower(spec.publicName()), "seedance", spec.Name)
		require.NotContains(t, strings.ToLower(spec.publicName()), "987xm", spec.Name)
		require.NotEqual(t, spec.Name, spec.publicName(), spec.Name)
		require.True(t, spec.ForwardResolution, spec.Name)
		require.True(t, spec.FirstLastFrame, spec.Name)
		require.Equal(t, "720p", spec.defaultResolution(), spec.Name)
		require.Contains(t, spec.Resolutions, spec.defaultResolution(), spec.Name)
		// Same names as the jijiu fields: the upstream's own field names are written by the
		// platform's normalizer, not by the table.
		require.Equal(t, "images", spec.imageField(), spec.Name)
		require.Equal(t, "videos", spec.videoField(), spec.Name)
		require.Equal(t, "audios", spec.audioField(), spec.Name)
		wantMin, wantMax := 4, 30
		if spec.Name == xmTest20 {
			wantMin, wantMax = 5, 15
		}
		if spec.Name == xmTestX {
			wantMin, wantMax = 5, 30
		}
		require.Equal(t, wantMin, spec.MinSeconds, spec.Name)
		require.Equal(t, wantMax, spec.MaxSeconds, spec.Name)
	}

	// The operator's figures per model.
	face, ok := xmModelSpecFor("SEEDANCE_2.5")
	require.True(t, ok)
	require.Equal(t, xmTestFace, face.Name)
	require.Equal(t, []string{"720p"}, face.Resolutions)
	require.Equal(t, 30, face.MaxImages)
	require.Equal(t, 0, face.MaxVideos)
	require.Equal(t, 10, face.MaxAudios)

	vip, ok := xmModelSpecFor(xmTestVIPPublic)
	require.True(t, ok)
	require.Equal(t, xmTestVIP, vip.Name)
	require.Equal(t, []string{"720p"}, vip.Resolutions)
	require.Equal(t, 20, vip.MaxImages)
	require.Equal(t, 5, vip.MaxVideos)
	require.Equal(t, 5, vip.MaxAudios)

	full, ok := xmModelSpecFor(strings.ToLower(xmTestFull))
	require.True(t, ok)
	require.Equal(t, xmTestFull, full.Name)
	require.Equal(t, []string{"720p"}, full.Resolutions)
	require.Equal(t, 30, full.MaxImages)
	require.Equal(t, 10, full.MaxVideos)
	require.Equal(t, 10, full.MaxAudios)

	// seedance_2.0 (2026-11-19): 480p / 720p / 1080p, 5-15 seconds, 9 images / 0 videos / 3 audios.
	v20, ok := xmModelSpecFor("SEEDANCE_2.0")
	require.True(t, ok)
	require.Equal(t, xmTest20, v20.Name)
	require.Equal(t, xmTest20Public, v20.publicName())
	require.Equal(t, []string{"480p", "720p", "1080p"}, v20.Resolutions)
	require.Equal(t, 9, v20.MaxImages)
	require.Equal(t, 0, v20.MaxVideos)
	require.Equal(t, 3, v20.MaxAudios)
	require.False(t, v20.NoAudioSwitch)
	require.Equal(t, []string{xmTest20Public, xmTest20}, ppVideoXMModelAliases(xmTest20))

	// seedance_2.5-flash (2026-11-19): 480p / 720p, 4-30 seconds, 10 images / 0 videos / 5 audios.
	flash, ok := xmModelSpecFor("SEEDANCE_2.5-FLASH")
	require.True(t, ok)
	require.Equal(t, xmTestFlash, flash.Name)
	require.Equal(t, xmTestFlashPublic, flash.publicName())
	require.Equal(t, []string{"480p", "720p"}, flash.Resolutions)
	require.Equal(t, 10, flash.MaxImages)
	require.Equal(t, 0, flash.MaxVideos)
	require.Equal(t, 5, flash.MaxAudios)
	require.Equal(t, []string{xmTestFlashPublic, xmTestFlash}, ppVideoXMModelAliases(xmTestFlashPublic))
	// It is a model of its own: seedance_2.5 keeps its limits and does not answer to the flash name.
	require.Equal(t, xmTestFace, face.Name)
	require.Equal(t, []string{"720p"}, face.Resolutions)

	// SD2.5-X-满参支持人脸 (2026-11-24): 720p only, 5-30 seconds, 30 images / 10 videos / 10 audios,
	// no audio switch (like its sibling M-full).
	xSpec, ok := xmModelSpecFor(strings.ToLower(xmTestX))
	require.True(t, ok)
	require.Equal(t, xmTestX, xSpec.Name)
	require.Equal(t, xmTestXPublic, xSpec.publicName())
	require.Equal(t, []string{"720p"}, xSpec.Resolutions)
	require.Equal(t, 30, xSpec.MaxImages)
	require.Equal(t, 10, xSpec.MaxVideos)
	require.Equal(t, 10, xSpec.MaxAudios)
	require.True(t, xSpec.NoAudioSwitch)
	require.Equal(t, []string{xmTestXPublic, xmTestX}, ppVideoXMModelAliases(xmTestXPublic))
	// It is a model of its own: the M-full row keeps its name and does not answer to the X one.
	xm, ok := xmModelSpecFor(xmTestXPublic)
	require.True(t, ok)
	require.NotEqual(t, full.Name, xm.Name)

	require.Equal(t, []string{xmTestFacePublic, xmTestFace}, ppVideoXMModelAliases("seedance_2.5"))
	require.Equal(t, []string{xmTestVIPPublic, xmTestVIP}, ppVideoPricingModelAliases(PlatformXMSeedance, "Seedance2.5-xm-720P-VIP"))

	// Not offered: the display names of the operator's own list and other model families.
	for _, model := range []string{
		"", "稳定渠道-满参-sd2.5 ·真人脸", "sd2.5-01-720", "SD2.5-M-满参", "seedance-2.0", "seedance-2.5",
		"doubao-seedance-2.5-nsfw", "minimax-h3",
	} {
		_, ok = xmModelSpecFor(model)
		require.False(t, ok, model)
	}
}

func TestXMSeedance25FlashRequest(t *testing.T) {
	t.Parallel()

	const (
		a  = "https://example.com/a.png"
		au = "https://example.com/au.mp3"
		v  = "https://example.com/v.mp4"
	)
	for _, tc := range []struct{ body, want string }{
		{
			`{"model":"` + xmTestFlashPublic + `","prompt":"x","duration":4}`,
			`{"model":"` + xmTestFlash + `","prompt":"x","duration":4,"resolution":"720p"}`,
		},
		{
			`{"model":"` + xmTestFlash + `","prompt":"x","duration":30,"resolution":"480P","images":["` + a + `"],"audios":["` + au + `"]}`,
			`{"model":"` + xmTestFlash + `","prompt":"x","duration":30,"resolution":"480p","media":[` +
				`{"type":"reference_image","url":"` + a + `"},{"type":"reference_audio","url":"` + au + `"}]}`,
		},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.JSONEq(t, tc.want, string(prepared))
		require.Equal(t, xmTestFlashPublic, public.Model)
		require.Equal(t, xmTestFlash, public.UpstreamModel)
	}

	prepared, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+xmTestFlashPublic+`","prompt":"x","duration":9,"resolution":"480p"}`))
	require.NoError(t, err)
	meta := PPVideoBillingMetadataFromRequest(PlatformXMSeedance, prepared)
	require.Equal(t, xmTestFlash, meta.Model)
	require.Equal(t, "480p", meta.VideoResolution)
	require.Equal(t, 9, meta.RequestedDurationSeconds)

	// Outside the limits: 3 and 31 seconds, 1080p, a reference video, an 11th image, a 6th audio.
	rep := func(item string, n int) string { return strings.TrimSuffix(strings.Repeat(`"`+item+`",`, n), ",") }
	for _, body := range []string{
		`{"model":"` + xmTestFlashPublic + `","prompt":"x","duration":3}`,
		`{"model":"` + xmTestFlashPublic + `","prompt":"x","duration":31}`,
		`{"model":"` + xmTestFlashPublic + `","prompt":"x","duration":5,"resolution":"1080p"}`,
		`{"model":"` + xmTestFlashPublic + `","prompt":"x","duration":5,"videos":["` + v + `"]}`,
		`{"model":"` + xmTestFlashPublic + `","prompt":"x","duration":5,"images":[` + rep(a, 11) + `]}`,
		`{"model":"` + xmTestFlashPublic + `","prompt":"x","duration":5,"audios":[` + rep(au, 6) + `]}`,
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(body))
		require.Error(t, err, body)
	}
}

// SD2.5-X-满参支持人脸 (2026-11-24): 720p only, 5-30 seconds, 30 images / 10 videos / 10 audios,
// and no audio switch toward the upstream (like M-full).
func TestXMSeedanceXFullRequest(t *testing.T) {
	t.Parallel()

	const (
		a  = "https://example.com/a.png"
		au = "https://example.com/au.mp3"
		v  = "https://example.com/v.mp4"
	)
	for _, tc := range []struct{ body, want string }{
		{
			// Public name, no resolution: 720p is filled in and the id is what goes upstream.
			`{"model":"` + xmTestXPublic + `","prompt":"x","duration":5}`,
			`{"model":"` + xmTestX + `","prompt":"x","duration":5,"resolution":"720p"}`,
		},
		{
			// The upstream id works too; enable_audio: true is the upstream default, so it is left out.
			`{"model":"` + xmTestX + `","prompt":"x","duration":30,"resolution":"720P","enable_audio":true,` +
				`"images":["` + a + `"],"videos":["` + v + `"],"audios":["` + au + `"]}`,
			`{"model":"` + xmTestX + `","prompt":"x","duration":30,"resolution":"720p","media":[` +
				`{"type":"reference_image","url":"` + a + `"},{"type":"reference_video","url":"` + v + `"},` +
				`{"type":"reference_audio","url":"` + au + `"}]}`,
		},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.JSONEq(t, tc.want, string(prepared))
		require.Equal(t, xmTestXPublic, public.Model)
		require.Equal(t, xmTestX, public.UpstreamModel)
	}

	// The limits are reachable: 30 images, 10 videos and 10 audios in one request.
	rep := func(item string, n int) string { return strings.TrimSuffix(strings.Repeat(`"`+item+`",`, n), ",") }
	_, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+xmTestXPublic+`","prompt":"x","duration":10,"images":[`+rep(a, 30)+`],"videos":[`+rep(v, 10)+`],"audios":[`+rep(au, 10)+`]}`))
	require.NoError(t, err)

	// Billing sees the requested seconds and 720p.
	prepared, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+xmTestXPublic+`","prompt":"x","duration":12}`))
	require.NoError(t, err)
	meta := PPVideoBillingMetadataFromRequest(PlatformXMSeedance, prepared)
	require.Equal(t, xmTestX, meta.Model)
	require.Equal(t, "720p", meta.VideoResolution)
	require.Equal(t, 12, meta.RequestedDurationSeconds)

	// Outside the limits: 4 and 31 seconds (this row starts at 5), 480p, 1080p, an 11th video, a 31st
	// image, an 11th audio, and turning the audio off (the upstream refuses the switch for the
	// sibling model, so it is not forwarded here either).
	for _, body := range []string{
		`{"model":"` + xmTestXPublic + `","prompt":"x","duration":4}`,
		`{"model":"` + xmTestXPublic + `","prompt":"x","duration":31}`,
		`{"model":"` + xmTestXPublic + `","prompt":"x","duration":5,"resolution":"480p"}`,
		`{"model":"` + xmTestXPublic + `","prompt":"x","duration":5,"resolution":"1080p"}`,
		`{"model":"` + xmTestXPublic + `","prompt":"x","duration":5,"videos":[` + rep(v, 11) + `]}`,
		`{"model":"` + xmTestXPublic + `","prompt":"x","duration":5,"images":[` + rep(a, 31) + `]}`,
		`{"model":"` + xmTestXPublic + `","prompt":"x","duration":5,"audios":[` + rep(au, 11) + `]}`,
		`{"model":"` + xmTestXPublic + `","prompt":"x","duration":5,"generate_audio":false}`,
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(body))
		require.Error(t, err, body)
	}

	// Capabilities come from the same table: 5-30 s, 720p, 30/10/10, no audio switch.
	caps := PPVideoCapabilitiesForModel(PlatformXMSeedance, xmTestXPublic)
	require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 30}, caps.Duration)
	require.Equal(t, []string{"720p"}, caps.Resolutions)
	require.Equal(t, 30, *caps.Assets.ReferenceImages)
	require.Equal(t, 10, *caps.Assets.ReferenceVideos)
	require.Equal(t, 10, *caps.Assets.ReferenceAudios)
	require.False(t, *caps.GenerateAudio)
}

func TestXMSeedance20Request(t *testing.T) {
	t.Parallel()

	const (
		a  = "https://example.com/a.png"
		au = "https://example.com/au.mp3"
		v  = "https://example.com/v.mp4"
	)
	// Every resolution of the model goes through (720p when none is named), with the audio switch.
	for _, tc := range []struct{ body, want string }{
		{
			`{"model":"` + xmTest20Public + `","prompt":"x","duration":5}`,
			`{"model":"` + xmTest20 + `","prompt":"x","duration":5,"resolution":"720p"}`,
		},
		{
			`{"model":"` + xmTest20 + `","prompt":"x","duration":15,"resolution":"480p","generate_audio":false,"images":["` + a + `"],"audios":["` + au + `"]}`,
			`{"model":"` + xmTest20 + `","prompt":"x","duration":15,"resolution":"480p","enable_audio":false,"media":[` +
				`{"type":"reference_image","url":"` + a + `"},{"type":"reference_audio","url":"` + au + `"}]}`,
		},
		{
			`{"model":"` + xmTest20Public + `","prompt":"x","duration":10,"resolution":"1080P"}`,
			`{"model":"` + xmTest20 + `","prompt":"x","duration":10,"resolution":"1080p"}`,
		},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.JSONEq(t, tc.want, string(prepared))
		require.Equal(t, xmTest20Public, public.Model)
		require.Equal(t, xmTest20, public.UpstreamModel)
	}

	// Billing sees the resolution, so a per-second channel price can have one tier per resolution.
	prepared, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+xmTest20Public+`","prompt":"x","duration":8,"resolution":"1080p"}`))
	require.NoError(t, err)
	meta := PPVideoBillingMetadataFromRequest(PlatformXMSeedance, prepared)
	require.Equal(t, xmTest20, meta.Model)
	require.Equal(t, 8, meta.RequestedDurationSeconds)
	require.Equal(t, "1080p", meta.VideoResolution)

	// Outside the model's limits: 4 and 16 seconds, 4k, a reference video, a tenth image, a fourth audio.
	for _, body := range []string{
		`{"model":"` + xmTest20Public + `","prompt":"x","duration":4}`,
		`{"model":"` + xmTest20Public + `","prompt":"x","duration":16}`,
		`{"model":"` + xmTest20Public + `","prompt":"x","duration":5,"resolution":"4k"}`,
		`{"model":"` + xmTest20Public + `","prompt":"x","duration":5,"videos":["` + v + `"]}`,
		`{"model":"` + xmTest20Public + `","prompt":"x","duration":5,"images":["` + a + `","` + a + `","` + a + `","` + a + `","` + a + `","` + a + `","` + a + `","` + a + `","` + a + `","` + a + `"]}`,
		`{"model":"` + xmTest20Public + `","prompt":"x","duration":5,"audios":["` + au + `","` + au + `","` + au + `","` + au + `"]}`,
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(body))
		require.Error(t, err, body)
	}
}

func TestXMSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformXMSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformXMSeedance))
	require.True(t, IsAllowedQuotaPlatform(PlatformXMSeedance))
	require.True(t, IsSeedanceHubMemberPlatform(PlatformXMSeedance))
	require.True(t, PPVideoGroupServesPlatform(PlatformSeedanceHub, PlatformXMSeedance))

	for _, model := range []string{xmTestFace, xmTestVIP, xmTestFull, xmTestFacePublic, xmTestVIPPublic, xmTestFullPublic, xmTestX, xmTestXPublic} {
		require.True(t, ppVideoModelBelongsToPlatform(PlatformXMSeedance, model), model)
		// A model of this platform is never foreign to it, even though the upstream id seedance_2.5
		// starts with "seedance" and so also looks like a model of the generic Seedance platform.
		require.False(t, ppVideoModelKnownForeignToPlatform(PlatformXMSeedance, model), model)
	}
	require.False(t, ppVideoModelBelongsToPlatform(PlatformXMSeedance, "other"))
	// This platform's models are foreign to the others.
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformZexiSeedance, xmTestVIPPublic))
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformHuahuoSeedance, xmTestFullPublic))
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformHuahuoSeedance, xmTestXPublic))

	// A whitelist saved under either name is listed once, under the public name.
	require.Equal(t, []string{xmTestFacePublic, xmTestFullPublic, xmTestXPublic},
		filterPPVideoPublicModelsForPlatform(PlatformXMSeedance, []string{xmTestFace, xmTestFacePublic, "other", xmTestFull, xmTestX, xmTestXPublic}))
	require.Equal(t, xmTestVIPPublic, PPVideoPublicModelName(PlatformXMSeedance, xmTestVIP))
	require.Equal(t, xmTestXPublic, PPVideoPublicModelName(PlatformXMSeedance, xmTestX))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformXMSeedance, "other"))
}

// Account sync answers from the local table without any HTTP request.
func TestXMSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformXMSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Equal(t, xmModelNames(), models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformXMSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestXMSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformXMSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformXMSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformXMSeedance, "xm-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer xm-key", headers.Get("Authorization"))
	require.Empty(t, headers.Get("Idempotency-Key"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformXMSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformXMSeedance))

	// queued / in_progress -> completed / failed; unknown is neither success nor failure.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","object":"video","status":"queued","progress":0}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"unknown"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed","progress":100}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"code":"x","message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformXMSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	failed, err := ParsePPVideoResponse(PlatformXMSeedance, []byte(`{"id":"task_1","status":"failed","error":{"code":"x","message":"content rejected"}}`))
	require.NoError(t, err)
	require.Equal(t, "content rejected", failed.ErrorMessage)
	rejected, err := ParsePPVideoResponse(PlatformXMSeedance, []byte(`{"error":"insufficient balance"}`))
	require.NoError(t, err)
	require.Equal(t, "insufficient balance", rejected.ErrorMessage)

	// Billing uses the requested seconds, never a length or usage the upstream reports.
	echoed, err := ParsePPVideoResponse(PlatformXMSeedance, []byte(`{"id":"task_1","status":"completed","seconds":"9","usage":{"total_tokens":123}}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)
	require.Zero(t, echoed.InputVideoDurationMilliseconds)
}

func TestXMSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: xmTestVIP}

	raw := `{"id":"task_1","object":"video","model":"` + xmTestVIP + `","status":"completed","progress":100,` +
		`"video_url":"https://ai.987xm.com/v1/videos/task_1/content","cost":2.9,"final_cost":2.9,"price":3,"credit_amount":12}`
	parsed, err := ParsePPVideoResponse(PlatformXMSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformXMSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	// The client sees the public name, this gateway's own address, and nothing of the upstream's.
	require.Equal(t, xmTestVIPPublic, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	for _, field := range []string{"cost", "final_cost", "price", "credit_amount"} {
		require.False(t, gjson.GetBytes(normalized, field).Exists(), field)
	}
	require.NotContains(t, string(normalized), "987xm")
	require.NotContains(t, string(normalized), "bd-VIP")

	// A task that is still running never shows a result address, even if the upstream sends one.
	running := `{"id":"task_1","status":"in_progress","progress":30,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformXMSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformXMSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")
}

func TestXMSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	const (
		a  = "https://example.com/a.png"
		b  = "https://example.com/b.png"
		v  = "https://example.com/v.mp4"
		au = "https://example.com/au.mp3"
	)
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "minimal request: the default resolution is sent, nothing else",
			body: `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":5}`,
			want: `{"model":"` + xmTestFull + `","prompt":"x","duration":5,"resolution":"720p"}`,
		},
		{
			name: "every field, media as typed items; the upstream id is what is sent",
			body: `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":"8","ratio":"9:16","resolution":"720P",` +
				`"images":["` + a + `","` + b + `"],"reference_videos":["` + v + `"],"audios":["` + au + `"]}`,
			want: `{"model":"` + xmTestFull + `","prompt":"x","duration":8,"resolution":"720p","ratio":"9:16","media":[` +
				`{"type":"reference_image","url":"` + a + `"},` +
				`{"type":"reference_image","url":"` + b + `"},` +
				`{"type":"reference_video","url":"` + v + `"},` +
				`{"type":"reference_audio","url":"` + au + `"}]}`,
		},
		{
			name: "the upstream id is accepted as the model name, the longest length, aspect_ratio as an alias",
			body: `{"model":"SEEDANCE_2.5","prompt":"x","duration":30,"resolution":"720p","aspect_ratio":"21:9"}`,
			want: `{"model":"seedance_2.5","prompt":"x","duration":30,"resolution":"720p","ratio":"21:9"}`,
		},
		{
			name: "enable_audio is accepted as an alias of generate_audio, and false is sent as it is",
			body: `{"model":"` + xmTestFacePublic + `","prompt":"x","duration":4,"enable_audio":false}`,
			want: `{"model":"` + xmTestFace + `","prompt":"x","duration":4,"resolution":"720p","enable_audio":false}`,
		},
		{
			name: "models that refuse the audio switch never get it: true is the upstream default and is left out",
			body: `{"model":"` + xmTestVIPPublic + `","prompt":"x","duration":4,"enable_audio":true}`,
			want: `{"model":"` + xmTestVIP + `","prompt":"x","duration":4,"resolution":"720p"}`,
		},
		{
			name: "same for generate_audio on the other per-request model",
			body: `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":4,"generate_audio":true}`,
			want: `{"model":"` + xmTestFull + `","prompt":"x","duration":4,"resolution":"720p"}`,
		},
		{
			name: "first and last frame",
			body: `{"model":"` + xmTestFacePublic + `","prompt":"x","duration":5,"first_frame":"` + a + `","last_frame":"` + b + `"}`,
			want: `{"model":"` + xmTestFace + `","prompt":"x","duration":5,"resolution":"720p","media":[` +
				`{"type":"first_frame","url":"` + a + `"},{"type":"last_frame","url":"` + b + `"}]}`,
		},
		{
			name: "a first frame alone, written as an object, and end_frame as an alias of last_frame",
			body: `{"model":"` + xmTestFacePublic + `","prompt":"x","duration":5,"first_frame":{"url":"` + a + `"},"end_frame":"` + b + `"}`,
			want: `{"model":"` + xmTestFace + `","prompt":"x","duration":5,"resolution":"720p","media":[` +
				`{"type":"first_frame","url":"` + a + `"},{"type":"last_frame","url":"` + b + `"}]}`,
		},
		{
			name: "audio references need no image here: the upstream decides",
			body: `{"model":"` + xmTestFacePublic + `","prompt":"x","duration":5,"audio_urls":["` + au + `"]}`,
			want: `{"model":"` + xmTestFace + `","prompt":"x","duration":5,"resolution":"720p","media":[` +
				`{"type":"reference_audio","url":"` + au + `"}]}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestXMSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ model, upstream, resolution string }{
		{xmTestFacePublic, xmTestFace, "720p"},
		{xmTestVIPPublic, xmTestVIP, "720p"},
		{xmTestFullPublic, xmTestFull, "720p"},
		{xmTestFace, xmTestFace, "720p"},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"`+tc.model+`","prompt":"x","duration":12,"resolution":"`+tc.resolution+`","images":["https://example.com/1.png"]}`))
		require.NoError(t, err, tc.model)
		// The task is shown under the public name; only the upstream id is ever sent.
		require.Equal(t, xmPublicNameOf(t, tc.upstream), public.Model)
		require.Equal(t, tc.upstream, public.UpstreamModel)
		require.Equal(t, tc.upstream, gjson.GetBytes(prepared, "model").String())
		require.Equal(t, int64(12000), public.DurationMilliseconds)
		require.Equal(t, 1, public.VideoCount)
		require.Equal(t, tc.resolution, public.Resolution)
		require.True(t, public.HasImage)

		meta := PPVideoBillingMetadataFromRequest(PlatformXMSeedance, prepared)
		require.Equal(t, PlatformXMSeedance, meta.Platform)
		require.Equal(t, tc.upstream, meta.Model)
		require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
		require.Equal(t, 12, meta.RequestedDurationSeconds)
		require.Equal(t, tc.resolution, meta.VideoResolution)
	}

	// The default resolution is sent, so the task is billed at it.
	prepared, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+xmTestFacePublic+`","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, "720p", gjson.GetBytes(prepared, "resolution").String())
	require.Equal(t, "720p", PPVideoBillingMetadataFromRequest(PlatformXMSeedance, prepared).VideoResolution)

	// Frames count as an image input.
	_, public, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+xmTestFacePublic+`","prompt":"x","duration":5,"first_frame":"https://example.com/a.png"}`))
	require.NoError(t, err)
	require.True(t, public.HasImage)
}

func xmPublicNameOf(t *testing.T, model string) string {
	t.Helper()
	name, ok := ppVideoXMPublicModel(model)
	require.True(t, ok, model)
	return name
}

func TestXMSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	full := `{"model":"` + xmTestFullPublic + `","prompt":"x",`
	vip := `{"model":"` + xmTestVIPPublic + `","prompt":"x",`
	face := `{"model":"` + xmTestFacePublic + `","prompt":"x",`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","duration":5}`, "model is required"},
		{"the operator's display name is not a model", `{"model":"sd2.5-01-720","prompt":"x","duration":5}`, "is not available"},
		{"missing prompt", `{"model":"` + xmTestFullPublic + `","duration":5}`, "prompt is required"},
		{"missing duration", full + `"ratio":"16:9"}`, "duration is required"},
		{"below the minimum", full + `"duration":3}`, "from 4 to 30 seconds"},
		{"above the maximum", full + `"duration":31}`, "from 4 to 30 seconds"},
		{"automatic duration is not offered", full + `"duration":-1}`, "from 4 to 30 seconds"},
		{"a fractional duration", full + `"duration":5.5}`, "integer"},
		{"1080p on a 720p model", full + `"duration":5,"resolution":"1080p"}`, "resolution must be 720p for this model"},
		{"1080p on the stable-face model", face + `"duration":5,"resolution":"1080p"}`, "resolution must be 720p for this model"},
		{"480p on the stable-face model", face + `"duration":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		{"4k", face + `"duration":5,"resolution":"4k"}`, "resolution must be 720p for this model"},
		{"unknown ratio", full + `"duration":5,"ratio":"2:1"}`, "ratio must be one of"},
		{"adaptive is not a documented ratio", full + `"duration":5,"ratio":"adaptive"}`, "ratio must be one of"},
		{"the upstream's own media array", full + `"duration":5,"media":[{"type":"reference_image","url":` + ref + `}]}`, "does not support media"},
		{"content", full + `"duration":5,"content":[]}`, "does not support content"},
		{"seed", full + `"duration":5,"seed":1}`, "does not support seed"},
		{"watermark", full + `"duration":5,"watermark":false}`, "does not support watermark"},
		{"callback", full + `"duration":5,"callback_url":"https://example.com/cb"}`, "does not support callback_url"},
		{"generate_audio as a string", full + `"duration":5,"generate_audio":"true"}`, "generate_audio must be a boolean"},
		{"enable_audio as a string", full + `"duration":5,"enable_audio":"true"}`, "enable_audio must be a boolean"},
		{"audio off on a model that cannot", full + `"duration":5,"generate_audio":false}`, "does not support turning the audio off"},
		{"audio off on the other such model", vip + `"duration":5,"enable_audio":false}`, "does not support turning the audio off"},
		{"both audio switches", full + `"duration":5,"generate_audio":true,"enable_audio":true}`, "either generate_audio or enable_audio"},
		{"assets on v1", full + `"duration":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"two videos in one request", full + `"duration":5,"n":2}`, "one video per request"},
		{"too many images", full + `"duration":5,"images":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many images on the 20-image model", vip + `"duration":5,"images":[` + repeatedJijiuItems(ref, 21) + `]}`, "at most 20 reference images"},
		{"too many videos on the 5-video model", vip + `"duration":5,"videos":[` + repeatedJijiuItems(ref, 6) + `]}`, "at most 5 reference videos"},
		{"too many audios on the 5-audio model", vip + `"duration":5,"audios":[` + repeatedJijiuItems(ref, 6) + `]}`, "at most 5 reference audios"},
		{"too many audios", full + `"duration":5,"audios":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference audios"},
		{"reference videos on the model that has none", face + `"duration":5,"videos":[` + ref + `]}`, "does not support reference videos for this model"},
		{"two names for the same images", full + `"duration":5,"images":[` + ref + `],"image_urls":[` + ref + `]}`, "accepts only one of"},
		{"plain http media", full + `"duration":5,"images":["http://example.com/1.png"]}`, "must be a public HTTPS URL"},
		{"media on a private address", full + `"duration":5,"images":["https://127.0.0.1/1.png"]}`, "must be a public HTTPS URL"},
		{"a last frame alone", full + `"duration":5,"last_frame":` + ref + `}`, "last_frame requires first_frame"},
		{"frames with reference images", full + `"duration":5,"first_frame":` + ref + `,"images":[` + ref + `]}`, "cannot be combined with reference"},
		{"frames with a reference audio", full + `"duration":5,"first_frame":` + ref + `,"audios":[` + ref + `]}`, "cannot be combined with reference"},
		{"plain http frame", full + `"duration":5,"first_frame":"http://example.com/a.png"}`, "must be a public HTTPS URL"},
		{"two names for the last frame", full + `"duration":5,"first_frame":` + ref + `,"end_frame":` + ref + `,"last_frame":` + ref + `}`, "only one of end_frame and last_frame"},
		{"another frame alias", full + `"duration":5,"first_frame_url":` + ref + `}`, "does not support first or last frames"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// The upper limits of each model are accepted.
	for _, tc := range []struct {
		head                   string
		images, videos, audios int
	}{
		{full, 30, 10, 10},
		{vip, 20, 5, 5},
		{face, 30, 0, 10},
	} {
		body := tc.head + `"duration":30,"images":[` + repeatedJijiuItems(ref, tc.images) + `],"audios":[` + repeatedJijiuItems(ref, tc.audios) + `]`
		if tc.videos > 0 {
			body += `,"videos":[` + repeatedJijiuItems(ref, tc.videos) + `]`
		}
		_, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, []byte(body+`}`))
		require.NoError(t, err, tc.head)
	}

	// Other platforms still refuse what only this one offers (the same request is fine without it).
	_, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-特价满血","prompt":"x","duration":5}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-特价满血","prompt":"x","duration":5,"generate_audio":true}`))
	require.ErrorContains(t, err, "does not support generate_audio")
}

func TestTranslatePPVideoUnifiedRequestToXM(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformXMSeedance, []byte(`{
		"model": "`+xmTestFullPublic+`",
		"prompt": "@image1 在街道上与@image2 对话",
		"duration": 5,
		"resolution": "720p",
		"ratio": "16:9",
		"generate_audio": true,
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "image", "url": "https://cdn.example.com/other.jpg"},
			{"type": "video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "`+xmTestFull+`",
		"prompt": "@image1 在街道上与@image2 对话",
		"duration": 5,
		"resolution": "720p",
		"ratio": "16:9",
		"media": [
			{"type": "reference_image", "url": "https://cdn.example.com/person.jpg"},
			{"type": "reference_image", "url": "https://cdn.example.com/other.jpg"},
			{"type": "reference_video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "reference_audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`, string(prepared))

	// Frames, with the audio switch turned off (false is different from leaving it out).
	translated, err = TranslatePPVideoUnifiedRequest(PlatformXMSeedance, []byte(`{
		"model": "`+xmTestFacePublic+`",
		"prompt": "花朵从含苞到盛开",
		"duration": 5,
		"generate_audio": false,
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/bud.png"},
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/bloom.png"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err = PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "`+xmTestFace+`",
		"prompt": "花朵从含苞到盛开",
		"duration": 5,
		"resolution": "720p",
		"enable_audio": false,
		"media": [
			{"type": "first_frame", "url": "https://cdn.example.com/bud.png"},
			{"type": "last_frame", "url": "https://cdn.example.com/bloom.png"}
		]
	}`, string(prepared))

	for _, tc := range []struct{ name, body, want string }{
		{"last frame alone", `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":5,"assets":[{"type":"image","role":"last_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
		{"frames with references", `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"},{"type":"image","url":"https://cdn.example.com/b.png"}]}`, "cannot be combined"},
		{"no seed", `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":5,"seed":7}`, "seed"},
		{"no watermark", `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":5,"watermark":true}`, "watermark"},
		{"no callback", `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":5,"callback_url":"https://example.com/cb"}`, "callback_url"},
		{"no metadata", `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":5,"metadata":{"a":"b"}}`, "metadata"},
		{"no options", `{"model":"` + xmTestFullPublic + `","prompt":"x","duration":5,"options":{"a":"b"}}`, "options"},
		{"prompt is required", `{"model":"` + xmTestFullPublic + `","duration":5}`, "prompt"},
		{"duration is required", `{"model":"` + xmTestFullPublic + `","prompt":"x"}`, "duration"},
		{"model is required", `{"prompt":"x","duration":5}`, "model"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformXMSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}

	// Rules that live in the native normalizer apply to v2 requests as well.
	for _, tc := range []struct{ name, body, want string }{
		{"reference videos on the model that has none", `{"model":"` + xmTestFacePublic + `","prompt":"x","duration":5,"assets":[{"type":"video","url":"https://cdn.example.com/a.mp4"}]}`, "does not support reference videos"},
		{"unknown model", `{"model":"sd2.5-01-720","prompt":"x","duration":5}`, "is not available"},
		{"duration out of range", `{"model":"` + xmTestVIPPublic + `","prompt":"x","duration":31}`, "from 4 to 30 seconds"},
		{"resolution out of range", `{"model":"` + xmTestVIPPublic + `","prompt":"x","duration":5,"resolution":"1080p"}`, "resolution must be 720p"},
	} {
		translated, err := TranslatePPVideoUnifiedRequest(PlatformXMSeedance, []byte(tc.body))
		require.NoError(t, err, tc.name)
		_, _, err = PreparePPVideoRequestBody(PlatformXMSeedance, PPVideoOperationGeneric, translated)
		require.ErrorContains(t, err, tc.want, tc.name)
	}
}

// A unified request must behave exactly like the equivalent native request.
func TestUnifiedXMRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformXMSeedance,
		`{
			"model": "`+xmTestVIPPublic+`",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"resolution": "720p",
			"ratio": "9:16",
			"generate_audio": true,
			"assets": [
				{"type": "image", "name": "cat.png", "url": "https://cdn.example.com/cat.png"},
				{"type": "image", "url": "https://cdn.example.com/cat2.png"},
				{"type": "video", "url": "https://cdn.example.com/run.mp4"},
				{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
			]
		}`,
		`{
			"model": "`+xmTestVIPPublic+`",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"resolution": "720p",
			"ratio": "9:16",
			"generate_audio": true,
			"images": ["https://cdn.example.com/cat.png", "https://cdn.example.com/cat2.png"],
			"videos": ["https://cdn.example.com/run.mp4"],
			"audios": ["https://cdn.example.com/voice.mp3"]
		}`,
	)
	requireUnifiedMatchesNative(t, PlatformXMSeedance,
		`{
			"model": "`+xmTestFacePublic+`",
			"prompt": "花朵从含苞到盛开",
			"duration": 5,
			"assets": [
				{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/bud.png"},
				{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/bloom.png"}
			]
		}`,
		`{
			"model": "`+xmTestFacePublic+`",
			"prompt": "花朵从含苞到盛开",
			"duration": 5,
			"first_frame": "https://cdn.example.com/bud.png",
			"last_frame": "https://cdn.example.com/bloom.png"
		}`,
	)
}

func TestXMSeedanceCapabilitiesQuoteAndHubLines(t *testing.T) {
	t.Parallel()

	face := PPVideoCapabilitiesForModel(PlatformXMSeedance, xmTestFacePublic)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, face.Duration)
	require.Equal(t, []string{"720p"}, face.Resolutions)
	require.Equal(t, 30, *face.Assets.ReferenceImages)
	require.Equal(t, 0, *face.Assets.ReferenceVideos)
	require.Equal(t, 10, *face.Assets.ReferenceAudios)
	require.True(t, *face.Assets.FirstLastFrame)
	require.True(t, *face.GenerateAudio)
	require.False(t, *face.Callback)
	require.Equal(t, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, face.Ratios)

	vip := PPVideoCapabilitiesForModel(PlatformXMSeedance, xmTestVIP)
	require.Equal(t, []string{"720p"}, vip.Resolutions)
	require.False(t, *vip.GenerateAudio, "the upstream refuses the audio switch for this model")
	require.Equal(t, 20, *vip.Assets.ReferenceImages)
	require.Equal(t, 5, *vip.Assets.ReferenceVideos)
	require.Equal(t, 5, *vip.Assets.ReferenceAudios)
	require.Nil(t, PPVideoCapabilitiesForModel(PlatformXMSeedance, "sd2.5-01-720").Duration)

	// The price estimate enforces the same limits as the submission.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformXMSeedance, xmTestFullPublic, "", 10)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformXMSeedance, xmTestFace, "720p", 30)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{xmTestFullPublic, "", 3},
		{xmTestFullPublic, "", 31},
		{xmTestFullPublic, "1080p", 10},
		{xmTestFacePublic, "4k", 10},
		{xmTestFacePublic, "1080p", 10},
		{xmTestFacePublic, "480p", 10},
		{"sd2.5-01-720", "", 10},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformXMSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.5--720p--稳定满参真人脸--按次--XM", "720p", 10)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)

	// Every model of the table is one line of the aggregate group, tag XM, and the line reports
	// the platform's own limits. All three are billed per request.
	wantNames := map[string]string{
		xmTestFace:  "Seedance2.5--720p--稳定满参真人脸--按次--XM",
		xmTestVIP:   "Seedance2.5--720p--01--按次--XM",
		xmTestFull:  "Seedance2.5--720p--M满参--按次--XM",
		xmTest20:    "Seedance2.0--480p/720p/1080p--sd2.0--按秒--XM",
		xmTestFlash: "Seedance2.5--480p/720p--flash--按秒--XM",
		xmTestX:     "Seedance2.5--720p--X满参--按次--XM",
	}
	wantBilling := map[string]BillingMode{
		xmTestFace:  BillingModePerRequest,
		xmTestVIP:   BillingModePerRequest,
		xmTestFull:  BillingModePerRequest,
		xmTest20:    BillingModeVideo,
		xmTestFlash: BillingModeVideo,
		xmTestX:     BillingModePerRequest,
	}
	count := 0
	for _, line := range SeedanceHubLines() {
		if line.Platform != PlatformXMSeedance {
			continue
		}
		count++
		spec, found := xmModelSpecFor(line.Model)
		require.True(t, found, line.Name)
		require.Equal(t, spec.publicName(), line.Model, line.Name)
		require.Equal(t, wantNames[spec.Name], line.Name)
		require.Equal(t, wantBilling[spec.Name], line.Billing, line.Name)
		// Every name a line answers to reaches the line: the hub name, the public name and the upstream id.
		for _, name := range []string{line.Name, line.Model, spec.Name} {
			byName, found := SeedanceHubLineByName(name)
			require.True(t, found, name)
			require.Equal(t, line.Name, byName.Name)
		}
		// Only this platform serves it.
		_, found = SeedanceHubLineFor(PlatformZexiSeedance, line.Model)
		require.False(t, found, line.Name)
		require.Equal(t, PPVideoCapabilitiesForModel(PlatformXMSeedance, spec.Name).Duration,
			PPVideoCapabilitiesForModel(PlatformSeedanceHub, line.Name).Duration)

		// A hub request is rewritten to the platform's public name, and then goes through the platform.
		quoted := fmt.Sprintf("%q", line.Name)
		resolved, body, err := ResolveSeedanceHubRequest([]byte(`{"model":` + quoted + `,"prompt":"x","duration":5}`))
		require.NoError(t, err)
		require.Equal(t, line.Name, resolved.Name)
		require.Equal(t, line.Model, gjson.GetBytes(body, "model").String())
		prepared, _, err := PreparePPVideoRequestBody(resolved.Platform, PPVideoOperationGeneric, body)
		require.NoError(t, err)
		require.Equal(t, spec.Name, gjson.GetBytes(prepared, "model").String())
	}
	require.Equal(t, len(xmSeedanceModels), count)

	// The operator's final table of 2026-11-19 lists seedance_2.0, seedance_2.5 and seedance_2.5-flash
	// of XM: those three lines are visible, the other two (vip, M-full) stay hidden. SD2.5-X-满参
	// (2026-11-24, the operator's request, with a price) is visible too.
	for name, wantHidden := range map[string]bool{
		"Seedance2.0--480p/720p/1080p--sd2.0--按秒--XM": false,
		"Seedance2.5--720p--稳定满参真人脸--按次--XM":             false,
		"Seedance2.5--480p/720p--flash--按秒--XM":      false,
		"Seedance2.5--720p--X满参--按次--XM":            false,
		"Seedance2.5--720p--01--按次--XM":              true,
		"Seedance2.5--720p--M满参--按次--XM":            true,
	} {
		line, found := SeedanceHubLineByName(name)
		require.True(t, found, name)
		require.Equal(t, wantHidden, line.Hidden, name)
	}
}

func xmTestAccount() *Account {
	return &Account{ID: 7, Platform: PlatformXMSeedance, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "xm-key"}}
}

func xmTestResponse(status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestXMContentDownloadURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://cdn.example.com/v.mp4?sig=1", xmContentDownloadURL([]byte(`{"url":" https://cdn.example.com/v.mp4?sig=1 "}`)))
	for _, raw := range []string{
		``, `not json`, `{}`, `{"url":""}`, `{"url":5}`,
		`{"url":"http://cdn.example.com/v.mp4"}`,
		`{"url":"/v1/videos/task_1/content"}`,
		`{"url":"https://user:pass@cdn.example.com/v.mp4"}`,
		`{"url":"ftp://cdn.example.com/v.mp4"}`,
	} {
		require.Empty(t, xmContentDownloadURL([]byte(raw)), raw)
	}
}

func TestXMContentFollowsTheDownloadAddressWithoutCredentials(t *testing.T) {
	t.Parallel()

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{
		xmTestResponse(http.StatusOK, "application/json; charset=utf-8", `{"url":"https://cdn.example.com/v.mp4?sig=1"}`),
		xmTestResponse(http.StatusPartialContent, "video/mp4", "video"),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	resp, err := svc.ForwardPPVideoContent(context.Background(), xmTestAccount(), "task-1", "bytes=0-4", "\"v1\"")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "video", string(body))
	require.Equal(t, http.StatusPartialContent, resp.StatusCode)

	require.Len(t, upstream.requests, 2)
	// The first call is the authenticated /content endpoint of the upstream ...
	require.Equal(t, "https://ai.987xm.com/v1/videos/task-1/content", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer xm-key", upstream.requests[0].Header.Get("Authorization"))
	require.Contains(t, upstream.requests[0].Header.Get("Accept"), "video/*")
	// ... the second one is the address it answered, with no credential and with the range.
	require.Equal(t, "https://cdn.example.com/v.mp4?sig=1", upstream.requests[1].URL.String())
	require.Empty(t, upstream.requests[1].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-4", upstream.requests[1].Header.Get("Range"))
	require.Equal(t, "\"v1\"", upstream.requests[1].Header.Get("If-Range"))
}

func TestXMContentReturnsAFileAnswerUnchanged(t *testing.T) {
	t.Parallel()

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{xmTestResponse(http.StatusOK, "video/mp4", "video")}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	resp, err := svc.ForwardPPVideoContent(context.Background(), xmTestAccount(), "task-1", "", "")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "video", string(body))
	require.Len(t, upstream.requests, 1)
}

func TestXMContentReportsAnExpiredLinkAndBadAnswers(t *testing.T) {
	t.Parallel()

	// 410 (the upstream's "link expired") reaches the caller as it is.
	upstream := &oponeUpstreamRecorder{responses: []*http.Response{xmTestResponse(http.StatusGone, "application/json", `{"error":{"message":"expired"}}`)}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	resp, err := svc.ForwardPPVideoContent(context.Background(), xmTestAccount(), "task-1", "", "")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusGone, resp.StatusCode)
	require.Len(t, upstream.requests, 1)

	// The address it answers expired (or is refused): that status is returned, not a made-up success.
	upstream = &oponeUpstreamRecorder{responses: []*http.Response{
		xmTestResponse(http.StatusOK, "application/json", `{"url":"https://cdn.example.com/v.mp4"}`),
		xmTestResponse(http.StatusForbidden, "text/plain", "expired"),
	}}
	svc = &OpenAIGatewayService{httpUpstream: upstream}
	resp2, err := svc.ForwardPPVideoContent(context.Background(), xmTestAccount(), "task-1", "", "")
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusForbidden, resp2.StatusCode)

	// JSON without a usable address is an error and nothing is fetched.
	for _, raw := range []string{`{}`, `{"url":"http://cdn.example.com/v.mp4"}`, `{"url":"https://u:p@cdn.example.com/v.mp4"}`} {
		upstream = &oponeUpstreamRecorder{responses: []*http.Response{xmTestResponse(http.StatusOK, "application/json", raw)}}
		svc = &OpenAIGatewayService{httpUpstream: upstream}
		_, err = svc.ForwardPPVideoContent(context.Background(), xmTestAccount(), "task-1", "", "")
		require.Error(t, err, raw)
		require.Len(t, upstream.requests, 1, raw)
	}
}

// A HEAD request is answered by reading the upstream's JSON with a GET, because only its body
// tells where the file is.
func TestXMContentHeadRequestIsSentAsGet(t *testing.T) {
	t.Parallel()

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{
		xmTestResponse(http.StatusOK, "application/json", `{"url":"https://cdn.example.com/v.mp4"}`),
		xmTestResponse(http.StatusOK, "video/mp4", "video"),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	resp, err := svc.forwardPPVideoContentWithMethod(context.Background(), http.MethodHead, xmTestAccount(), "task-1", "", "")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, http.MethodGet, upstream.requests[0].Method)
	require.Equal(t, http.MethodGet, upstream.requests[1].Method)
}
