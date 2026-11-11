package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// requireUnifiedMatchesNative checks that a unified (v2) request behaves exactly
// like the equivalent native request: the existing pipeline produces the same
// upstream body, the same public request and the same billing metadata.
func requireUnifiedMatchesNative(t *testing.T, platform string, unifiedBody string, nativeBody string) {
	t.Helper()

	translated, err := TranslatePPVideoUnifiedRequest(platform, []byte(unifiedBody))
	require.NoError(t, err)

	translatedPrepared, translatedPublic, err := PreparePPVideoRequestBody(platform, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	nativePrepared, nativePublic, err := PreparePPVideoRequestBody(platform, PPVideoOperationGeneric, []byte(nativeBody))
	require.NoError(t, err)

	require.JSONEq(t, string(nativePrepared), string(translatedPrepared))
	require.Equal(t, nativePublic, translatedPublic)
	require.Equal(t,
		PPVideoBillingMetadataFromRequest(platform, []byte(nativeBody)),
		PPVideoBillingMetadataFromRequest(platform, translated),
	)
}

func ppVideoUnifiedTestImageAssets(count int) string {
	items := make([]string, 0, count)
	for index := 0; index < count; index++ {
		items = append(items, fmt.Sprintf(`{"type":"image","url":"https://cdn.example.com/%d.png"}`, index))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func TestTranslatePPVideoUnifiedRequestToWan(t *testing.T) {
	t.Parallel()

	frames, err := TranslatePPVideoUnifiedRequest(PlatformWan3, []byte(`{
		"model": "wan3.0-video",
		"prompt": "猫咪从草地左边跑到右边",
		"duration": 8,
		"resolution": "720p",
		"ratio": "16:9",
		"generate_audio": true,
		"seed": 5,
		"watermark": false,
		"options": {"prompt_extend": false},
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/start.jpg"},
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/end.jpg"}
		]
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "wan3.0-video",
		"input": {
			"prompt": "猫咪从草地左边跑到右边",
			"media": [
				{"type": "first_frame", "url": "https://cdn.example.com/start.jpg"},
				{"type": "last_frame", "url": "https://cdn.example.com/end.jpg"}
			]
		},
		"parameters": {
			"duration": 8,
			"resolution": "720p",
			"ratio": "16:9",
			"audio": true,
			"watermark": false,
			"seed": 5,
			"prompt_extend": false
		}
	}`, string(frames))

	references, err := TranslatePPVideoUnifiedRequest(PlatformWan3, []byte(`{
		"prompt": "参考素材生成",
		"assets": [
			{"type": "video", "url": "https://cdn.example.com/ref.mp4"},
			{"type": "image", "url": "https://cdn.example.com/ref.jpg"},
			{"type": "audio", "url": "https://cdn.example.com/ref.mp3"}
		]
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"input": {
			"prompt": "参考素材生成",
			"media": [
				{"type": "reference_video", "url": "https://cdn.example.com/ref.mp4"},
				{"type": "reference_image", "url": "https://cdn.example.com/ref.jpg"},
				{"type": "reference_audio", "url": "https://cdn.example.com/ref.mp3"}
			]
		},
		"parameters": {}
	}`, string(references))
}

func TestTranslatePPVideoUnifiedRequestToPixverse(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "text to video keeps the ratio",
			body: `{"prompt":"纸鹤起飞","duration":5,"resolution":"720p","ratio":"16:9","generate_audio":false,"seed":3}`,
			want: `{"input":{"prompt":"纸鹤起飞"},"parameters":{"duration":5,"resolution":"720p","aspect_ratio":"16:9","generate_audio":false,"seed":3}}`,
		},
		{
			name: "reference image leaves the ratio out because the model ignores it",
			body: `{"model":"pixverse-v6","prompt":"纸鹤起飞","duration":5,"ratio":"9:16","assets":[{"type":"image","url":"https://cdn.example.com/ref.jpg"}]}`,
			want: `{"model":"pixverse-v6","input":{"prompt":"纸鹤起飞","img_url":"https://cdn.example.com/ref.jpg"},"parameters":{"duration":5}}`,
		},
		{
			name: "first and last frame",
			body: `{"prompt":"p","assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.jpg"},{"type":"image","role":"last_frame","url":"https://cdn.example.com/b.jpg"}]}`,
			want: `{"input":{"prompt":"p","first_frame_url":"https://cdn.example.com/a.jpg","last_frame_url":"https://cdn.example.com/b.jpg"},"parameters":{}}`,
		},
		{
			name: "reference video is the video extension input",
			body: `{"prompt":"p","assets":[{"type":"video","url":"https://cdn.example.com/v.mp4"}]}`,
			want: `{"input":{"prompt":"p","video_url":"https://cdn.example.com/v.mp4"},"parameters":{}}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			translated, err := TranslatePPVideoUnifiedRequest(PlatformPixverseV6, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(translated))
		})
	}
}

func TestTranslatePPVideoUnifiedRequestToCompShare(t *testing.T) {
	t.Parallel()

	full, err := TranslatePPVideoUnifiedRequest(PlatformMiniMaxH3CompShare, []byte(`{
		"model": "MiniMax-H3",
		"prompt": "晨光照亮湖面",
		"duration": 10,
		"resolution": "1080p",
		"ratio": "9:16",
		"generate_audio": false,
		"watermark": true,
		"callback_url": "https://example.com/callback",
		"callback_token": "token-1",
		"options": {"use_context_ir": true, "skill_id": "skill-1"},
		"assets": [
			{"type": "image", "url": "https://cdn.example.com/a.png"},
			{"type": "video", "url": "https://cdn.example.com/b.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/c.mp3"}
		]
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "MiniMax-H3",
		"content": [
			{"type": "text", "text": "晨光照亮湖面"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/a.png"}, "role": "reference_image"},
			{"type": "video_url", "video_url": {"url": "https://cdn.example.com/b.mp4"}, "role": "reference_video"},
			{"type": "audio_url", "audio_url": {"url": "https://cdn.example.com/c.mp3"}, "role": "reference_audio"}
		],
		"duration": 10,
		"resolution": "1080P",
		"ratio": "9:16",
		"mute_audio": true,
		"aigc_watermark": true,
		"use_context_ir": true,
		"skill_id": "skill-1",
		"callback_url": "https://example.com/callback",
		"callback_token": "token-1"
	}`, string(full))

	frames, err := TranslatePPVideoUnifiedRequest(PlatformMiniMaxH3CompShare, []byte(`{
		"prompt": "p",
		"resolution": "2K",
		"generate_audio": true,
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/a.png"},
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/b.png"}
		]
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"content": [
			{"type": "text", "text": "p"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/a.png"}, "role": "first_frame"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/b.png"}, "role": "last_frame"}
		],
		"resolution": "2K",
		"mute_audio": false
	}`, string(frames))
}

func TestTranslatePPVideoUnifiedRequestToGeneralModels(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(Platform88APIVideo, []byte(`{
		"model": "veo-3.1",
		"prompt": "p",
		"duration": 8,
		"resolution": "1080p",
		"ratio": "16:9",
		"generate_audio": true,
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/f.png"},
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/l.png"},
			{"type": "image", "url": "https://cdn.example.com/r.png"},
			{"type": "video", "url": "https://cdn.example.com/v.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/a.mp3"}
		]
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "veo-3.1",
		"prompt": "p",
		"duration": 8,
		"resolution": "1080p",
		"ratio": "16:9",
		"generate_audio": true,
		"images": ["https://cdn.example.com/r.png"],
		"videos": ["https://cdn.example.com/v.mp4"],
		"metadata": {
			"referenceAudios": ["https://cdn.example.com/a.mp3"],
			"firstFrame": "https://cdn.example.com/f.png",
			"lastFrame": "https://cdn.example.com/l.png"
		}
	}`, string(translated))
}

func TestUnifiedWanRequestMatchesNativeRequestThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformWan3, `{
		"model": "wan3.0-video",
		"prompt": "猫咪跑过草地",
		"duration": 8,
		"resolution": "720p",
		"ratio": "16:9",
		"generate_audio": false,
		"watermark": false,
		"seed": 5,
		"options": {"prompt_extend": true},
		"assets": [{"type": "image", "url": "https://cdn.example.com/ref.jpg"}]
	}`, `{
		"model": "wan3.0-video",
		"input": {
			"prompt": "猫咪跑过草地",
			"media": [{"type": "reference_image", "url": "https://cdn.example.com/ref.jpg"}]
		},
		"parameters": {
			"duration": 8,
			"resolution": "720p",
			"ratio": "16:9",
			"audio": false,
			"watermark": false,
			"seed": 5,
			"prompt_extend": true
		}
	}`)
}

func TestUnifiedPixverseRequestMatchesNativeRequestThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	// The ratio is dropped for image-to-video, exactly like the native request.
	requireUnifiedMatchesNative(t, PlatformPixverseV6, `{
		"model": "pixverse-v6",
		"prompt": "纸鹤起飞",
		"duration": 5,
		"resolution": "720p",
		"ratio": "9:16",
		"generate_audio": false,
		"seed": 3,
		"assets": [{"type": "image", "url": "https://cdn.example.com/ref.jpg"}]
	}`, `{
		"model": "pixverse-v6",
		"input": {"prompt": "纸鹤起飞", "img_url": "https://cdn.example.com/ref.jpg"},
		"parameters": {"duration": 5, "resolution": "720p", "generate_audio": false, "seed": 3}
	}`)
}

func TestUnifiedCompShareRequestMatchesNativeRequestThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformMiniMaxH3CompShare, `{
		"model": "MiniMax-H3",
		"prompt": "晨光照亮湖面",
		"duration": 10,
		"resolution": "1080p",
		"ratio": "9:16",
		"generate_audio": false,
		"watermark": false,
		"assets": [{"type": "image", "url": "https://cdn.example.com/a.png"}]
	}`, `{
		"model": "MiniMax-H3",
		"content": [
			{"type": "text", "text": "晨光照亮湖面"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/a.png"}, "role": "reference_image"}
		],
		"duration": 10,
		"resolution": "1080P",
		"ratio": "9:16",
		"mute_audio": true,
		"aigc_watermark": false
	}`)
}

func TestUnifiedGeneralModelRequestMatchesNativeRequestThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, Platform88APIVideo, `{
		"model": "grok-imagine-video",
		"prompt": "ocean",
		"duration": 5,
		"resolution": "720p",
		"ratio": "9:16",
		"assets": [{"type": "image", "url": "https://example.com/reference.png"}]
	}`, `{
		"model": "grok-imagine-video",
		"prompt": "ocean",
		"images": ["https://example.com/reference.png"],
		"duration": 5,
		"ratio": "9:16",
		"resolution": "720p"
	}`)
}

func TestTranslatePPVideoUnifiedRequestRejectsInvalidRequestsOnPhaseTwoPlatforms(t *testing.T) {
	t.Parallel()

	const (
		firstFrame = `{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}`
		reference  = `{"type":"image","url":"https://cdn.example.com/b.png"}`
		audio      = `{"type":"audio","url":"https://cdn.example.com/c.mp3"}`
	)

	for _, tc := range []struct {
		name     string
		platform string
		body     string
		want     string
	}{
		{"h3 without compshare is not supported yet", PlatformMiniMaxH3, `{"prompt":"x"}`, "not supported for this platform yet"},

		{"wan missing prompt", PlatformWan3, `{"assets":[` + reference + `]}`, "prompt: is required for this model"},
		{"wan frames with references", PlatformWan3, `{"prompt":"x","assets":[` + firstFrame + `,` + reference + `]}`, "assets: first_frame and last_frame images cannot be combined with reference assets for this model"},
		{"wan too many reference images", PlatformWan3, `{"prompt":"x","assets":` + ppVideoUnifiedTestImageAssets(11) + `}`, "assets[10]: too many reference images; this model allows at most 10"},
		{"wan callback_url", PlatformWan3, `{"prompt":"x","callback_url":"https://example.com/cb"}`, "callback_url: is not supported by this model"},
		{"wan metadata", PlatformWan3, `{"prompt":"x","metadata":{"a":1}}`, "metadata: is not supported by this model"},
		{"wan unknown option", PlatformWan3, `{"prompt":"x","options":{"foo":1}}`, "options.foo: is not a supported option for this model"},
		{"wan prompt_extend type", PlatformWan3, `{"prompt":"x","options":{"prompt_extend":"yes"}}`, "options.prompt_extend: must be a boolean"},

		{"pixverse missing prompt", PlatformPixverseV6, `{"assets":[` + reference + `]}`, "prompt: is required for this model"},
		{"pixverse audio asset", PlatformPixverseV6, `{"prompt":"x","assets":[` + reference + `,` + audio + `]}`, "assets[1].type: audio assets are not supported by this model"},
		{"pixverse first frame alone", PlatformPixverseV6, `{"prompt":"x","assets":[` + firstFrame + `]}`, "assets: first_frame and last_frame must be provided together for this model"},
		{"pixverse two reference images", PlatformPixverseV6, `{"prompt":"x","assets":` + ppVideoUnifiedTestImageAssets(2) + `}`, "assets[1]: too many reference images; this model allows at most 1"},
		{"pixverse watermark", PlatformPixverseV6, `{"prompt":"x","watermark":true}`, "watermark: is not supported by this model"},
		{"pixverse callback_url", PlatformPixverseV6, `{"prompt":"x","callback_url":"https://example.com/cb"}`, "callback_url: is not supported by this model"},
		{"pixverse option", PlatformPixverseV6, `{"prompt":"x","options":{"foo":1}}`, "options.foo: is not a supported option for this model"},

		{"compshare resolution outside the native set", PlatformMiniMaxH3CompShare, `{"prompt":"x","resolution":"720p"}`, "resolution: must be one of 480p, 768p, 1080p, 2k, 4k for this model"},
		{"compshare ratio", PlatformMiniMaxH3CompShare, `{"prompt":"x","ratio":"5:4"}`, "ratio: must be one of adaptive, 21:9, 16:9, 4:3, 1:1, 3:4, 9:16 for this model"},
		{"compshare duration too short", PlatformMiniMaxH3CompShare, `{"prompt":"x","duration":3}`, "duration: must be between 4 and 30 for this model"},
		{"compshare duration too long", PlatformMiniMaxH3CompShare, `{"prompt":"x","duration":31}`, "duration: must be between 4 and 30 for this model"},
		{"compshare skill without context ir", PlatformMiniMaxH3CompShare, `{"prompt":"x","options":{"skill_id":"s"}}`, "options.skill_id: requires options.use_context_ir to be true"},
		{"compshare skill type", PlatformMiniMaxH3CompShare, `{"prompt":"x","options":{"use_context_ir":true,"skill_id":1}}`, "options.skill_id: must be a string"},
		{"compshare callback_token without callback_url", PlatformMiniMaxH3CompShare, `{"prompt":"x","callback_token":"t"}`, "callback_token: requires callback_url"},
		{"compshare frames with references", PlatformMiniMaxH3CompShare, `{"prompt":"x","assets":[` + firstFrame + `,` + reference + `]}`, "assets: first_frame and last_frame images cannot be combined with reference assets for this model"},
		{"compshare reference audio alone", PlatformMiniMaxH3CompShare, `{"prompt":"x","assets":[` + audio + `]}`, "assets[0]: reference audio requires a reference image or reference video for this model"},
		{"compshare too many reference images", PlatformMiniMaxH3CompShare, `{"prompt":"x","assets":` + ppVideoUnifiedTestImageAssets(10) + `}`, "assets[9]: too many reference images; this model allows at most 9"},
		{"compshare seed", PlatformMiniMaxH3CompShare, `{"prompt":"x","seed":1}`, "seed: is not supported by this model"},
		{"compshare metadata", PlatformMiniMaxH3CompShare, `{"prompt":"x","metadata":{"a":1}}`, "metadata: is not supported by this model"},
		{"compshare unknown option", PlatformMiniMaxH3CompShare, `{"prompt":"x","options":{"foo":1}}`, "options.foo: is not a supported option for this model"},
		{"compshare nothing to render", PlatformMiniMaxH3CompShare, `{"model":"MiniMax-H3"}`, "prompt: prompt or at least one asset is required"},

		{"general missing model", Platform88APIVideo, `{"prompt":"x"}`, "model: is required for this model group"},
		{"general seed", Platform88APIVideo, `{"model":"grok-imagine-video","prompt":"x","seed":1}`, "seed: is not supported by this model"},
		{"general watermark", Platform88APIVideo, `{"model":"grok-imagine-video","prompt":"x","watermark":true}`, "watermark: is not supported by this model"},
		{"general callback_url", Platform88APIVideo, `{"model":"grok-imagine-video","prompt":"x","callback_url":"https://example.com/cb"}`, "callback_url: is not supported by this model"},
		{"general metadata", Platform88APIVideo, `{"model":"grok-imagine-video","prompt":"x","metadata":{"a":1}}`, "metadata: is not supported by this model"},
		{"general option", Platform88APIVideo, `{"model":"grok-imagine-video","prompt":"x","options":{"foo":1}}`, "options.foo: is not a supported option for this model"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := TranslatePPVideoUnifiedRequest(tc.platform, []byte(tc.body))
			require.Error(t, err)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
