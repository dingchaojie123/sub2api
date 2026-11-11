package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// The three models the XingAPI Seedance platform serves to customers.
var ppVideoUnifiedXingAPIModels = []string{"seedance-933", "seedance2.0-933-C", "seedance2.5-V"}

func TestTranslatePPVideoUnifiedRequestToXingAPI(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformXingAPISeedance, []byte(`{
		"model": "seedance-933",
		"prompt": "以第一张图片为人物参考，第二张图片为场景参考",
		"duration": 5,
		"resolution": "720p",
		"ratio": "16:9",
		"generate_audio": true,
		"seed": 7,
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "image", "role": "reference", "name": "scene.jpg", "url": "https://cdn.example.com/scene.jpg"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	// Media is grouped by type, in the order sent; names are not forwarded; the
	// compatibility fields seconds and aspect_ratio are never produced.
	require.JSONEq(t, `{
		"model": "seedance-933",
		"prompt": "以第一张图片为人物参考，第二张图片为场景参考",
		"duration": 5,
		"resolution": "720p",
		"ratio": "16:9",
		"generate_audio": true,
		"seed": 7,
		"images": ["https://cdn.example.com/person.jpg", "https://cdn.example.com/scene.jpg"],
		"videos": ["https://cdn.example.com/motion.mp4"],
		"audios": ["https://cdn.example.com/voice.mp3"]
	}`, string(translated))
	for _, forbidden := range []string{"assets", "seconds", "aspect_ratio", "metadata", "content"} {
		require.False(t, gjson.GetBytes(translated, forbidden).Exists(), forbidden)
	}
}

func TestTranslatePPVideoUnifiedRequestToXingAPIMinimalAndAssetOnly(t *testing.T) {
	t.Parallel()

	minimal, err := TranslatePPVideoUnifiedRequest(PlatformXingAPISeedance, []byte(`{"model":"seedance2.5-V","prompt":"海边奔跑的猫","duration":5}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"seedance2.5-V","prompt":"海边奔跑的猫","duration":5}`, string(minimal))

	// A request may carry only reference media, like the native request can.
	assetOnly, err := TranslatePPVideoUnifiedRequest(PlatformXingAPISeedance, []byte(`{
		"model": "seedance2.0-933-C",
		"duration": 10,
		"assets": [{"type": "image", "url": "https://cdn.example.com/a.png"}]
	}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"seedance2.0-933-C","duration":10,"images":["https://cdn.example.com/a.png"]}`, string(assetOnly))

	// An explicit false must survive: it is a different request from an absent flag.
	muted, err := TranslatePPVideoUnifiedRequest(PlatformXingAPISeedance, []byte(`{"model":"seedance-933","prompt":"x","duration":5,"generate_audio":false}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"seedance-933","prompt":"x","duration":5,"generate_audio":false}`, string(muted))
}

// A unified request must behave exactly like the equivalent native request: same
// upstream body, same public request and same billing metadata.
func TestUnifiedXingAPIRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	for _, model := range ppVideoUnifiedXingAPIModels {
		model := model
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			requireUnifiedMatchesNative(t, PlatformXingAPISeedance,
				`{
					"model": "`+model+`",
					"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
					"duration": 8,
					"ratio": "9:16",
					"resolution": "720p",
					"assets": [
						{"type": "image", "name": "cat.png", "url": "https://cdn.example.com/cat.png"},
						{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
					]
				}`,
				`{
					"model": "`+model+`",
					"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
					"duration": 8,
					"ratio": "9:16",
					"resolution": "720p",
					"images": ["https://cdn.example.com/cat.png"],
					"audios": ["https://cdn.example.com/voice.mp3"]
				}`,
			)
		})
	}
}

func TestUnifiedXingAPIRequestReachesTheUpstreamWithMappedModel(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformXingAPISeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"seedance-933": "vendor-seedance-933"},
		},
	}
	translated, err := TranslatePPVideoUnifiedRequest(PlatformXingAPISeedance, []byte(`{
		"model": "seedance-933", "prompt": "lake", "duration": 5,
		"assets": [{"type": "image", "url": "https://cdn.example.com/a.png"}]
	}`))
	require.NoError(t, err)

	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, translated)

	require.NoError(t, err)
	require.Equal(t, "seedance-933", public.Model)
	require.Equal(t, "vendor-seedance-933", public.UpstreamModel)
	require.Equal(t, "vendor-seedance-933", gjson.GetBytes(prepared, "model").String())
	require.Equal(t, "https://cdn.example.com/a.png", gjson.GetBytes(prepared, "images.0").String())
}

func TestTranslatePPVideoUnifiedRequestRejectsInvalidXingAPIRequests(t *testing.T) {
	t.Parallel()

	const (
		firstFrame = `{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}`
		lastFrame  = `{"type":"image","role":"last_frame","url":"https://cdn.example.com/b.png"}`
		reference  = `{"type":"image","url":"https://cdn.example.com/c.png"}`
	)

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","duration":5}`, "model: is required for this model group"},
		{"missing duration", `{"model":"seedance-933","prompt":"x"}`, "duration: is required for this model group"},
		{"first frame", `{"model":"seedance-933","prompt":"x","assets":[` + firstFrame + `]}`, "assets[0].role: first_frame is not supported by this model"},
		{"last frame", `{"model":"seedance-933","prompt":"x","assets":[` + reference + `,` + lastFrame + `]}`, "assets[1].role: last_frame is not supported by this model"},
		{"watermark", `{"model":"seedance-933","prompt":"x","watermark":true}`, "watermark: is not supported by this model"},
		{"callback_url", `{"model":"seedance-933","prompt":"x","callback_url":"https://example.com/cb"}`, "callback_url: is not supported by this model"},
		{"callback_token", `{"model":"seedance-933","prompt":"x","callback_token":"t"}`, "callback_token: is not supported by this model"},
		{"metadata", `{"model":"seedance-933","prompt":"x","metadata":{"a":1}}`, "metadata: is not supported by this model"},
		{"option", `{"model":"seedance-933","prompt":"x","options":{"foo":1}}`, "options.foo: is not a supported option for this model"},
		{"nothing to render", `{"model":"seedance-933"}`, "prompt: prompt or at least one asset is required"},
		{"duplicate asset names are still refused", `{"model":"seedance-933","prompt":"x","assets":[{"type":"image","name":"a.png","url":"https://cdn.example.com/1.png"},{"type":"image","name":"a.png","url":"https://cdn.example.com/2.png"}]}`, "assets[1].name"},
		{"native field names are not accepted", `{"model":"seedance-933","prompt":"x","aspect_ratio":"16:9"}`, "aspect_ratio: unknown field"},
		{"native seconds is not accepted", `{"model":"seedance-933","prompt":"x","seconds":5}`, "seconds: unknown field"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := TranslatePPVideoUnifiedRequest(PlatformXingAPISeedance, []byte(tc.body))
			require.Error(t, err)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
