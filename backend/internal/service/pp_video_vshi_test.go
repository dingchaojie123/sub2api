package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestVshiSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Len(t, vshiSeedanceModels, 4)
	seen := make(map[string]string)
	for _, spec := range vshiSeedanceModels {
		// A row answers to its upstream name and its public name (the same string when no
		// public name is set); no name may belong to two rows.
		names := []string{spec.Name}
		if spec.publicName() != spec.Name {
			names = append(names, spec.publicName())
		}
		for _, name := range names {
			key := strings.ToLower(name)
			owner, dup := seen[key]
			require.False(t, dup, "name %q is used by %q and %q", name, owner, spec.Name)
			seen[key] = spec.Name
		}
		// Clients pick video models by name: every public name has to say seedance.
		require.Contains(t, strings.ToLower(spec.publicName()), "seedance", spec.Name)
		// ... and none of them names the upstream vendor.
		require.NotContains(t, strings.ToLower(spec.publicName()), "vshi", spec.Name)
		require.NotEmpty(t, spec.Resolutions, spec.Name)
		require.Contains(t, spec.Resolutions, spec.defaultResolution(), spec.Name)
		require.False(t, spec.FirstLastFrame, spec.Name)

		// One set of limits per version (the operator's figures).
		if strings.Contains(spec.publicName(), "2.0") {
			require.Equal(t, 4, spec.MinSeconds, spec.Name)
			require.Equal(t, 15, spec.MaxSeconds, spec.Name)
			require.Equal(t, 9, spec.MaxImages, spec.Name)
			require.Equal(t, 3, spec.MaxVideos, spec.Name)
			require.Equal(t, 3, spec.MaxAudios, spec.Name)
		} else {
			require.Equal(t, 4, spec.MinSeconds, spec.Name)
			require.Equal(t, 30, spec.MaxSeconds, spec.Name)
			require.Equal(t, 30, spec.MaxImages, spec.Name)
			require.Equal(t, 10, spec.MaxVideos, spec.Name)
			require.Equal(t, 10, spec.MaxAudios, spec.Name)
		}
	}

	pro, ok := vshiModelSpecFor("nsfw-PRO")
	require.True(t, ok)
	require.Equal(t, "NSFW-pro", pro.Name)
	require.Equal(t, "Seedance2.0-NSFW-pro", pro.publicName())
	require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, pro.Resolutions)
	require.Equal(t, "720p", pro.defaultResolution())

	// Names with a hyphen and a dot are matched as written, ignoring case.
	v25, ok := vshiModelSpecFor("nsfw-2.5")
	require.True(t, ok)
	require.Equal(t, "NSFW-2.5", v25.Name)
	require.Equal(t, "Seedance2.5-NSFW", v25.publicName())
	require.Equal(t, []string{"480p", "720p", "1080p"}, v25.Resolutions)

	// Public name first, so a channel price configured under it wins; the upstream name still works.
	require.Equal(t, []string{"Seedance2.0-NSFW-pro", "NSFW-pro"}, ppVideoVshiModelAliases("nsfw-pro"))
	require.Equal(t, []string{"Seedance2.0-NSFW-pro", "NSFW-pro"}, ppVideoPricingModelAliases(PlatformVshiSeedance, "Seedance2.0-nsfw-PRO"))

	// Not offered: every other upstream model, including the ones the page lists.
	for _, model := range []string{"", "NSFW-pro-x", "seedance-2.0-mini", "vs-sd2.5-b", "vs-sd 2.5-a", "zhongshi-seedance-2.0-mini-480p", "doubao-seedance-2-0-260128", "huoshan-seedance-2-5-260628", "sd2.5-s", "sd2.0-110", "wan3.0-video", "jianyi-h3", "happyhorse-1.1"} {
		_, ok = vshiModelSpecFor(model)
		require.False(t, ok, model)
	}
}

func TestVshiSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformVshiSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformVshiSeedance))

	require.True(t, ppVideoModelBelongsToPlatform(PlatformVshiSeedance, "NSFW-pro"))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformVshiSeedance, "Seedance2.5-NSFW"))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformVshiSeedance, "other"))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformVshiSeedance, "vs-sd2.5-b"))
	// A whitelist saved under either name is listed once, under the public name; other names are dropped.
	require.Equal(t, []string{"Seedance2.0-NSFW-pro", "Seedance2.5-NSFW"},
		filterPPVideoPublicModelsForPlatform(PlatformVshiSeedance, []string{"NSFW-pro", "Seedance2.0-NSFW-pro", "NSFW-2.5", "vs-sd2.5-b", "other"}))
	require.Equal(t, "Seedance2.0-NSFW-pro", PPVideoPublicModelName(PlatformVshiSeedance, "NSFW-pro"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformVshiSeedance, "other"))
}

// Account sync answers from the local table without any HTTP request.
func TestVshiSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformVshiSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Equal(t, vshiModelNames(), models)
	require.Equal(t, []string{"Seedance2.0-NSFW-pro", "Seedance2.0-NSFW-fast", "Seedance2.0-NSFW-mini", "Seedance2.5-NSFW"}, models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformVshiSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestVshiSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	// The unified video collection, not /v1/videos.
	createPath, err := PPVideoUpstreamPath(PlatformVshiSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/unified/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformVshiSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/unified/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformVshiSeedance, "vshi-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer vshi-key", headers.Get("Authorization"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformVshiSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformVshiSeedance))

	// queued / in_progress -> completed / failed; a failed task may come back with HTTP 200.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","object":"video","status":"queued","progress":0,"submission_outcome":"accepted","api_version":"unified-v1"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed","progress":100,"result":{"download_url":"https://api.vshiai.com/v1/unified/videos/task_1/content"}}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"code":"x","message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformVshiSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	failed, err := ParsePPVideoResponse(PlatformVshiSeedance, []byte(`{"id":"task_1","status":"failed","error":{"code":"x","message":"素材不合规"}}`))
	require.NoError(t, err)
	require.Equal(t, "素材不合规", failed.ErrorMessage)

	// Billing uses the requested seconds, never the length or cost the upstream echoes.
	echoed, err := ParsePPVideoResponse(PlatformVshiSeedance, []byte(`{"id":"task_1","status":"completed","seconds":"9","final_cost":1.5,"usage":{"duration":9}}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)
}

func TestVshiSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: "NSFW-pro"}

	raw := `{"id":"task_1","task_id":"task_1","object":"video","model":"NSFW-pro","status":"completed","progress":100,` +
		`"result":{"preview_url":"https://api.vshiai.com/v1/unified/videos/task_1/content","download_url":"https://api.vshiai.com/v1/unified/videos/task_1/content?download=1"},` +
		`"final_cost":1.5,"usage":{"duration":9},"api_version":"unified-v1"}`
	parsed, err := ParsePPVideoResponse(PlatformVshiSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformVshiSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	// The client sees the public name, this gateway's own address, and nothing of the upstream's.
	require.Equal(t, "Seedance2.0-NSFW-pro", gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	require.NotContains(t, string(normalized), "vshiai.com")
	require.False(t, gjson.GetBytes(normalized, "final_cost").Exists())
	require.False(t, gjson.GetBytes(normalized, "result.preview_url").Exists())
	require.False(t, gjson.GetBytes(normalized, "result.download_url").Exists())

	// A running task never shows an address, even when the upstream already carries one.
	running := `{"id":"task_1","status":"in_progress","progress":30,"result":{"preview_url":"https://api.vshiai.com/x.mp4"}}`
	parsed, err = ParsePPVideoResponse(PlatformVshiSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformVshiSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "vshiai.com")
}

func TestVshiSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	prepare := func(body string) (map[string]any, error) {
		prepared, _, err := PreparePPVideoRequestBody(PlatformVshiSeedance, PPVideoOperationGeneric, []byte(body))
		if err != nil {
			return nil, err
		}
		return gjson.ParseBytes(prepared).Value().(map[string]any), nil
	}

	// The upstream receives its own name, whichever name was used, and its own field names.
	payload, err := prepare(`{"model":"Seedance2.0-NSFW-pro","prompt":"x","duration":15,"resolution":"4k","ratio":"9:16","images":["https://example.com/1.png"]}`)
	require.NoError(t, err)
	require.Equal(t, "NSFW-pro", payload["model"])
	require.EqualValues(t, 15, payload["seconds"])
	require.Equal(t, "4k", payload["resolution"])
	require.Equal(t, "9:16", payload["aspect_ratio"])
	require.Equal(t, []any{"https://example.com/1.png"}, payload["image_urls"])

	// The resolution is filled in (720p) when omitted.
	payload, err = prepare(`{"model":"NSFW-2.5","prompt":"x","seconds":30}`)
	require.NoError(t, err)
	require.Equal(t, "720p", payload["resolution"])

	// 2.5 takes 30 seconds, 1080p and 10 reference videos; 2.0 does not.
	videos := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		videos = append(videos, fmt.Sprintf(`"https://example.com/%d.mp4"`, i))
	}
	_, err = prepare(`{"model":"NSFW-2.5","prompt":"x","seconds":30,"resolution":"1080p","videos":[` + strings.Join(videos, ",") + `]}`)
	require.NoError(t, err)

	for name, body := range map[string]string{
		"2.0 longer than 15 seconds":      `{"model":"NSFW-pro","prompt":"x","seconds":16}`,
		"shorter than 4 seconds":          `{"model":"NSFW-2.5","prompt":"x","seconds":3}`,
		"2.5 longer than 30 seconds":      `{"model":"NSFW-2.5","prompt":"x","seconds":31}`,
		"fast has no 1080p":               `{"model":"NSFW-fast","prompt":"x","seconds":5,"resolution":"1080p"}`,
		"2.5 has no 4k":                   `{"model":"NSFW-2.5","prompt":"x","seconds":5,"resolution":"4k"}`,
		"mini has no 1080p":               `{"model":"NSFW-mini","prompt":"x","seconds":5,"resolution":"1080p"}`,
		"2.0 takes 9 images at most":      `{"model":"NSFW-pro","prompt":"x","seconds":5,"images":["https://example.com/1.png","https://example.com/2.png","https://example.com/3.png","https://example.com/4.png","https://example.com/5.png","https://example.com/6.png","https://example.com/7.png","https://example.com/8.png","https://example.com/9.png","https://example.com/10.png"]}`,
		"2.0 takes 3 videos at most":      `{"model":"NSFW-pro","prompt":"x","seconds":5,"videos":["https://example.com/1.mp4","https://example.com/2.mp4","https://example.com/3.mp4","https://example.com/4.mp4"]}`,
		"2.5 takes 10 videos at most":     `{"model":"NSFW-2.5","prompt":"x","seconds":5,"videos":["https://example.com/1.mp4","https://example.com/2.mp4","https://example.com/3.mp4","https://example.com/4.mp4","https://example.com/5.mp4","https://example.com/6.mp4","https://example.com/7.mp4","https://example.com/8.mp4","https://example.com/9.mp4","https://example.com/10.mp4","https://example.com/11.mp4"]}`,
		"reference over http":             `{"model":"NSFW-pro","prompt":"x","seconds":5,"images":["http://example.com/1.png"]}`,
		"ratio outside the accepted set":  `{"model":"NSFW-pro","prompt":"x","seconds":5,"ratio":"5:4"}`,
		"the upstream's content array":    `{"model":"NSFW-pro","prompt":"x","seconds":5,"content":[{"type":"text","text":"x"}]}`,
		"the upstream's media array":      `{"model":"NSFW-pro","prompt":"x","seconds":5,"media":[]}`,
		"provider options are not passed": `{"model":"NSFW-pro","prompt":"x","seconds":5,"provider_options":{"skipFacePrivacy":true}}`,
		"generation mode is not passed":   `{"model":"NSFW-pro","prompt":"x","seconds":5,"generation_mode":"first_last"}`,
		"official line is not offered":    `{"model":"doubao-seedance-2-0-260128","prompt":"x","seconds":5}`,
		"unknown model":                   `{"model":"other","prompt":"x","seconds":5}`,
	} {
		_, err := prepare(body)
		require.Error(t, err, name)
	}
}
