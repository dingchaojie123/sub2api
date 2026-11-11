package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestIsPPVideoUnifiedPlatform(t *testing.T) {
	t.Parallel()
	for _, platform := range []string{
		PlatformGuangying,
		PlatformByteDance,
		PlatformWan3,
		PlatformPixverseV6,
		PlatformMiniMaxH3CompShare,
		Platform88APIVideo,
		PlatformMaiTokenSeedance,
		PlatformXingAPISeedance,
		PlatformJijiuSeedance,
		PlatformCTMOSeedance,
		PlatformSeedanceHub,
		" " + PlatformGuangying + " ",
	} {
		require.True(t, IsPPVideoUnifiedPlatform(platform), "platform=%q", platform)
	}
	for _, platform := range []string{PlatformSeedance, PlatformOPONE, PlatformMiniMaxH3, PlatformOpenAI, ""} {
		require.False(t, IsPPVideoUnifiedPlatform(platform), "platform=%q", platform)
	}
}

func TestTranslatePPVideoUnifiedRequestToGuangying(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformGuangying, []byte(`{
		"model": "Seedance 2.5-全参720P",
		"prompt": "@角色.png 在 @场景.png 中自然向前走，配音参考 @配音.wav",
		"duration": 5,
		"resolution": "720p",
		"ratio": "9:16",
		"assets": [
			{"type": "image", "name": "角色.png", "url": "https://cdn.example.com/a.png"},
			{"type": "image", "role": "reference", "name": "场景.png", "url": "https://cdn.example.com/b.png"},
			{"type": "audio", "name": "配音.wav", "url": "https://cdn.example.com/c.wav"}
		],
		"options": {"native_face": true},
		"metadata": {"trace": "customer-1"}
	}`))

	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "Seedance 2.5-全参720P",
		"prompt": "@角色.png 在 @场景.png 中自然向前走，配音参考 @配音.wav",
		"duration": 5,
		"resolution": "720p",
		"ratio": "9:16",
		"assets": [
			{"type": "image", "name": "角色.png", "url": "https://cdn.example.com/a.png"},
			{"type": "image", "name": "场景.png", "url": "https://cdn.example.com/b.png"},
			{"type": "audio", "name": "配音.wav", "url": "https://cdn.example.com/c.wav"}
		],
		"native_face": true,
		"metadata": {"trace": "customer-1"}
	}`, string(translated))
}

func TestTranslatePPVideoUnifiedRequestToByteDance(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformByteDance, []byte(`{
		"model": "doubao-seedance-2-0-260128",
		"prompt": "人物微笑挥手",
		"duration": 5,
		"resolution": "720p",
		"ratio": "9:16",
		"generate_audio": false,
		"seed": 7,
		"watermark": false,
		"callback_url": "https://example.com/callback",
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/first.png"},
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/last.png"},
			{"type": "image", "url": "https://cdn.example.com/ref.png", "name": "ignored-name"},
			{"type": "video", "url": "https://cdn.example.com/ref.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/ref.mp3"}
		],
		"options": {"camera_fixed": true, "omni_reference_task_type": "r2v"}
	}`))

	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "doubao-seedance-2-0-260128",
		"input": {
			"content": [
				{"type": "text", "text": "人物微笑挥手"},
				{"type": "image_url", "image_url": {"url": "https://cdn.example.com/first.png"}, "role": "first_frame"},
				{"type": "image_url", "image_url": {"url": "https://cdn.example.com/last.png"}, "role": "last_frame"},
				{"type": "image_url", "image_url": {"url": "https://cdn.example.com/ref.png"}, "role": "reference_image"},
				{"type": "video_url", "video_url": {"url": "https://cdn.example.com/ref.mp4"}, "role": "reference_video"},
				{"type": "audio_url", "audio_url": {"url": "https://cdn.example.com/ref.mp3"}, "role": "reference_audio"}
			]
		},
		"parameters": {
			"duration": 5,
			"resolution": "720p",
			"ratio": "9:16",
			"generate_audio": false,
			"seed": 7,
			"watermark": false,
			"callback_url": "https://example.com/callback",
			"camera_fixed": true,
			"omni_reference_task_type": "r2v"
		}
	}`, string(translated))
}

// The translated body must behave exactly like the equivalent native request:
// the existing pipeline produces the same upstream body and the same public
// request, so billing and routing see no difference between v1 and v2.
func TestUnifiedByteDanceRequestMatchesNativeRequestThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformByteDance, []byte(`{
		"model": "doubao-seedance-2-0-260128",
		"prompt": "让素材中的人物微笑挥手",
		"duration": 5,
		"resolution": "720p",
		"ratio": "9:16",
		"generate_audio": false,
		"assets": [
			{"type": "image", "url": "https://cdn.example.com/ref.png"}
		]
	}`))
	require.NoError(t, err)

	native := []byte(`{
		"model": "doubao-seedance-2-0-260128",
		"input": {"content": [
			{"type": "text", "text": "让素材中的人物微笑挥手"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example.com/ref.png"}, "role": "reference_image"}
		]},
		"parameters": {"duration": 5, "resolution": "720p", "ratio": "9:16", "generate_audio": false}
	}`)

	translatedPrepared, translatedPublic, err := PreparePPVideoRequestBody(PlatformByteDance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	nativePrepared, nativePublic, err := PreparePPVideoRequestBody(PlatformByteDance, PPVideoOperationGeneric, native)
	require.NoError(t, err)

	require.JSONEq(t, string(nativePrepared), string(translatedPrepared))
	require.Equal(t, nativePublic, translatedPublic)
	require.Equal(t,
		PPVideoBillingMetadataFromRequest(PlatformByteDance, native),
		PPVideoBillingMetadataFromRequest(PlatformByteDance, translated),
	)
}

// Billing may be computed from either the client's v2 body or the translated
// body. Duration, resolution and the audio flag must agree in both cases.
func TestUnifiedRequestBillingMetadataIsIdenticalForRawAndTranslatedBody(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		platform       string
		body           string
		wantAudio      bool
		wantSeconds    int
		wantResolution string
	}{
		{
			name:           "bytedance audio off",
			platform:       PlatformByteDance,
			body:           `{"model":"doubao-seedance-2-0-260128","prompt":"x","duration":8,"resolution":"1080p","generate_audio":false}`,
			wantAudio:      false,
			wantSeconds:    8,
			wantResolution: VideoBillingResolution1080P,
		},
		{
			name:           "bytedance audio on",
			platform:       PlatformByteDance,
			body:           `{"model":"doubao-seedance-2-0-260128","prompt":"x","duration":5,"resolution":"720p","generate_audio":true}`,
			wantAudio:      true,
			wantSeconds:    5,
			wantResolution: VideoBillingResolution720P,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			translated, err := TranslatePPVideoUnifiedRequest(tc.platform, []byte(tc.body))
			require.NoError(t, err)

			raw := PPVideoBillingMetadataFromRequest(tc.platform, []byte(tc.body))
			after := PPVideoBillingMetadataFromRequest(tc.platform, translated)
			for name, metadata := range map[string]PPVideoBillingMetadata{"raw": raw, "translated": after} {
				require.NoError(t, metadata.ValidationError, name)
				require.Equal(t, tc.wantSeconds, metadata.RequestedDurationSeconds, name)
				require.Equal(t, tc.wantResolution, metadata.VideoResolution, name)
				require.Equal(t, tc.wantAudio, metadata.HasAudio, name)
			}
		})
	}
}

func TestUnifiedGuangyingRequestKeepsAssetNamesThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	publicModel := "Seedance 2.5-全参720P"
	account := &Account{
		Platform: PlatformGuangying,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{publicModel: publicModel},
		},
	}
	translated, err := TranslatePPVideoUnifiedRequest(PlatformGuangying, []byte(`{
		"model": "Seedance 2.5-全参720P",
		"prompt": "@android.png @角色.png 生成一段视频，配音参考 @13438346279716350.mp4 音频.wav 截取.wav",
		"duration": 15,
		"resolution": "720p",
		"ratio": "16:9",
		"assets": [
			{"type": "image", "name": "android.png", "url": "https://cdn.example.com/hashed-reference.png"},
			{"type": "image", "name": "角色.png", "url": "https://cdn.example.com/character.png"},
			{"type": "audio", "name": "13438346279716350.mp4 音频.wav 截取.wav", "url": "https://cdn.example.com/trimmed-audio.wav"}
		]
	}`))
	require.NoError(t, err)

	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, translated)

	require.NoError(t, err)
	require.Equal(t, publicModel, public.Model)
	require.Equal(t, int64(15000), public.DurationMilliseconds)
	require.Equal(t, "android.png", gjson.GetBytes(prepared, "assets.0.name").String())
	require.Equal(t, "角色.png", gjson.GetBytes(prepared, "assets.1.name").String())
	require.Equal(t, "13438346279716350.mp4 音频.wav 截取.wav", gjson.GetBytes(prepared, "assets.2.name").String())
	require.Equal(t, "audio", gjson.GetBytes(prepared, "assets.2.type").String())
	require.Equal(t, "16:9", gjson.GetBytes(prepared, "ratio").String())
}

func TestUnifiedRequestAutoNamesUnnamedAssetsPerTypeAndSkipsTakenNames(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformGuangying, []byte(`{
		"prompt": "x",
		"assets": [
			{"type": "image", "url": "https://cdn.example.com/a.png"},
			{"type": "image", "name": "image1", "url": "https://cdn.example.com/b.png"},
			{"type": "image", "url": "https://cdn.example.com/c.png"},
			{"type": "audio", "url": "https://cdn.example.com/d.wav"},
			{"type": "video", "url": "https://cdn.example.com/e.mp4"}
		]
	}`))

	require.NoError(t, err)
	names := make([]string, 0, 5)
	for _, asset := range gjson.GetBytes(translated, "assets").Array() {
		names = append(names, asset.Get("name").String())
	}
	require.Equal(t, []string{"image2", "image1", "image3", "audio1", "video1"}, names)
}

func TestTranslatePPVideoUnifiedRequestRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		platform string
		body     string
		want     string
	}{
		{"invalid json", PlatformGuangying, `{"prompt":`, "request body must be valid JSON"},
		{"not an object", PlatformGuangying, `["x"]`, "request body must be a JSON object"},
		{"unsupported platform", PlatformSeedance, `{"prompt":"x"}`, "not supported for this platform yet"},
		{"unknown top-level field", PlatformGuangying, `{"prompt":"x","aspect_ratio":"16:9"}`, "aspect_ratio: unknown field"},
		{"images array is not part of v2", PlatformByteDance, `{"prompt":"x","images":["https://cdn.example.com/a.png"]}`, "images: unknown field"},
		{"string duration", PlatformGuangying, `{"prompt":"x","duration":"5"}`, "duration: must be an integer"},
		{"fractional duration", PlatformGuangying, `{"prompt":"x","duration":5.5}`, "duration: must be an integer"},
		{"zero duration", PlatformGuangying, `{"prompt":"x","duration":0}`, "duration: must be at least 1"},
		{"non-string prompt", PlatformGuangying, `{"prompt":1}`, "prompt: must be a string"},
		{"non-boolean generate_audio", PlatformByteDance, `{"prompt":"x","generate_audio":"yes"}`, "generate_audio: must be a boolean"},
		{"assets not an array", PlatformGuangying, `{"prompt":"x","assets":{}}`, "assets: must be an array"},
		{"asset not an object", PlatformGuangying, `{"prompt":"x","assets":["a"]}`, "assets[0]: must be an object"},
		{"asset unknown field", PlatformGuangying, `{"prompt":"x","assets":[{"type":"image","url":"u","file_id":"f"}]}`, "assets[0].file_id: unknown field"},
		{"asset bad type", PlatformGuangying, `{"prompt":"x","assets":[{"type":"pdf","url":"u"}]}`, "assets[0].type: must be image, video, or audio"},
		{"asset missing url", PlatformGuangying, `{"prompt":"x","assets":[{"type":"image","url":"u"},{"type":"image"}]}`, "assets[1].url: is required"},
		{"asset bad role", PlatformByteDance, `{"prompt":"x","assets":[{"type":"image","role":"cover","url":"u"}]}`, "assets[0].role: must be first_frame, last_frame, or reference"},
		{"frame role on audio", PlatformByteDance, `{"prompt":"x","assets":[{"type":"audio","role":"first_frame","url":"u"}]}`, `assets[0].role: "first_frame" is only valid for image assets`},
		{"two first frames", PlatformByteDance, `{"prompt":"x","assets":[{"type":"image","role":"first_frame","url":"a"},{"type":"image","role":"first_frame","url":"b"}]}`, "assets[1].role: at most one first_frame image is allowed"},
		{"two last frames", PlatformByteDance, `{"prompt":"x","assets":[{"type":"image","role":"last_frame","url":"a"},{"type":"image","role":"last_frame","url":"b"}]}`, "assets[1].role: at most one last_frame image is allowed"},
		{"duplicate names", PlatformGuangying, `{"prompt":"x","assets":[{"type":"image","name":"a.png","url":"u"},{"type":"audio","name":"a.png","url":"v"}]}`, `assets[1].name: "a.png" is duplicated`},
		{"guangying generate_audio", PlatformGuangying, `{"prompt":"x","generate_audio":true}`, "generate_audio: is not supported by this model"},
		{"guangying seed", PlatformGuangying, `{"prompt":"x","seed":1}`, "seed: is not supported by this model"},
		{"guangying watermark", PlatformGuangying, `{"prompt":"x","watermark":true}`, "watermark: is not supported by this model"},
		{"guangying callback_url", PlatformGuangying, `{"prompt":"x","callback_url":"https://example.com/cb"}`, "callback_url: is not supported by this model"},
		{"guangying missing prompt", PlatformGuangying, `{"assets":[{"type":"image","url":"u"}]}`, "prompt: is required for this model"},
		{"guangying unknown option", PlatformGuangying, `{"prompt":"x","options":{"foo":1}}`, "options.foo: is not a supported option for this model"},
		{"guangying native_face type", PlatformGuangying, `{"prompt":"x","options":{"native_face":"yes"}}`, "options.native_face: must be a boolean"},
		{"bytedance nothing to render", PlatformByteDance, `{"model":"doubao-seedance-2-0-260128"}`, "prompt: prompt or at least one asset is required"},
		{"bytedance callback_token", PlatformByteDance, `{"prompt":"x","callback_token":"t"}`, "callback_token: is not supported by this model"},
		{"bytedance metadata", PlatformByteDance, `{"prompt":"x","metadata":{"a":1}}`, "metadata: is not supported by this model"},
		{"bytedance unknown option", PlatformByteDance, `{"prompt":"x","options":{"foo":1}}`, "options.foo: is not a supported option for this model"},
		{"bytedance camera_fixed type", PlatformByteDance, `{"prompt":"x","options":{"camera_fixed":"yes"}}`, "options.camera_fixed: must be a boolean"},
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

func TestTranslatePPVideoUnifiedRequestToGuangyingSubmitsFramesAsPlainImages(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformGuangying, []byte(`{
		"prompt": "人物缓慢转头",
		"assets": [
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/end.png"},
			{"type": "image", "role": "first_frame", "name": "开场.png", "url": "https://cdn.example.com/start.png"},
			{"type": "image", "url": "https://cdn.example.com/ref.png"}
		]
	}`))

	require.NoError(t, err)
	// The frame roles are accepted but not forwarded: this platform has no frame
	// marker, so the images go out as ordinary assets in the order they were sent.
	require.JSONEq(t, `{
		"prompt": "人物缓慢转头",
		"assets": [
			{"type": "image", "name": "image1", "url": "https://cdn.example.com/end.png"},
			{"type": "image", "name": "开场.png", "url": "https://cdn.example.com/start.png"},
			{"type": "image", "name": "image2", "url": "https://cdn.example.com/ref.png"}
		]
	}`, string(translated))
	require.False(t, gjson.GetBytes(translated, "assets.0.role").Exists())
}

// A unified first/last-frame request produces exactly what the native request
// with start_frame_url / end_frame_url produces today.
func TestUnifiedGuangyingFramesMatchNativeFrameFieldsThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	publicModel := "Seedance 2.5-全参720P"
	account := &Account{
		Platform: PlatformGuangying,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{publicModel: publicModel},
		},
	}
	translated, err := TranslatePPVideoUnifiedRequest(PlatformGuangying, []byte(`{
		"model": "Seedance 2.5-全参720P",
		"prompt": "人物缓慢转头",
		"duration": 5,
		"resolution": "720p",
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/start.png"},
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/end.png"}
		]
	}`))
	require.NoError(t, err)

	translatedPrepared, translatedPublic, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	nativePrepared, nativePublic, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, []byte(`{
		"model": "Seedance 2.5-全参720P",
		"prompt": "人物缓慢转头",
		"duration": 5,
		"resolution": "720p",
		"start_frame_url": "https://cdn.example.com/start.png",
		"end_frame_url": "https://cdn.example.com/end.png"
	}`))
	require.NoError(t, err)

	require.JSONEq(t, string(nativePrepared), string(translatedPrepared))
	require.Equal(t, nativePublic, translatedPublic)
	require.Equal(t, "image1", gjson.GetBytes(translatedPrepared, "assets.0.name").String())
	require.Equal(t, "image2", gjson.GetBytes(translatedPrepared, "assets.1.name").String())
}

func TestPPVideoUnifiedErrorExposesTheOffendingParam(t *testing.T) {
	t.Parallel()

	_, err := TranslatePPVideoUnifiedRequest(PlatformGuangying, []byte(`{
		"prompt": "x",
		"assets": [
			{"type": "image", "url": "https://cdn.example.com/a.png"},
			{"type": "image", "role": "cover", "url": "https://cdn.example.com/b.png"}
		]
	}`))

	var unified *PPVideoUnifiedError
	require.ErrorAs(t, err, &unified)
	require.Equal(t, "assets[1].role", unified.Param)
}
