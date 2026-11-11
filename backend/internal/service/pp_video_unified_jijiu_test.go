package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTranslatePPVideoUnifiedRequestToJijiu(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformJijiuSeedance, []byte(`{
		"model": "`+jijiuTestModelD+`",
		"prompt": "以第一张图片为人物参考",
		"duration": 5,
		"resolution": "1080p",
		"ratio": "16:9",
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "image", "role": "reference", "url": "https://cdn.example.com/scene.jpg"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	// Media is grouped by type, in the order sent; names are not forwarded.
	require.JSONEq(t, `{
		"model": "`+jijiuTestModelD+`",
		"prompt": "以第一张图片为人物参考",
		"duration": 5,
		"resolution": "1080p",
		"ratio": "16:9",
		"images": ["https://cdn.example.com/person.jpg", "https://cdn.example.com/scene.jpg"],
		"videos": ["https://cdn.example.com/motion.mp4"],
		"audios": ["https://cdn.example.com/voice.mp3"]
	}`, string(translated))
	for _, forbidden := range []string{"assets", "seconds", "aspect_ratio", "first_frame", "last_frame", "content"} {
		require.False(t, gjson.GetBytes(translated, forbidden).Exists(), forbidden)
	}

	minimal, err := TranslatePPVideoUnifiedRequest(PlatformJijiuSeedance, []byte(`{"model":"`+jijiuTestModel301+`","prompt":"海边奔跑的猫","duration":5}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"`+jijiuTestModel301+`","prompt":"海边奔跑的猫","duration":5}`, string(minimal))
}

// A unified request must behave exactly like the equivalent native request: same upstream
// body, same public request and same billing metadata.
func TestUnifiedJijiuRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	t.Run("reference media on a plain line", func(t *testing.T) {
		t.Parallel()
		requireUnifiedMatchesNative(t, PlatformJijiuSeedance,
			`{
				"model": "`+jijiuTestModel301+`",
				"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
				"duration": 8,
				"ratio": "9:16",
				"assets": [
					{"type": "image", "name": "cat.png", "url": "https://cdn.example.com/cat.png"},
					{"type": "video", "url": "https://cdn.example.com/run.mp4"},
					{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
				]
			}`,
			`{
				"model": "`+jijiuTestModel301+`",
				"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
				"duration": 8,
				"ratio": "9:16",
				"images": ["https://cdn.example.com/cat.png"],
				"videos": ["https://cdn.example.com/run.mp4"],
				"audios": ["https://cdn.example.com/voice.mp3"]
			}`,
		)
	})
	t.Run("a renamed line asked for by its public name", func(t *testing.T) {
		t.Parallel()
		requireUnifiedMatchesNative(t, PlatformJijiuSeedance,
			`{"model":"`+jijiuPublic301+`","prompt":"lake","duration":6,"assets":[{"type":"image","url":"https://cdn.example.com/a.png"}]}`,
			`{"model":"`+jijiuPublic301+`","prompt":"lake","duration":6,"images":["https://cdn.example.com/a.png"]}`,
		)
	})
	t.Run("the C line writes video_refs and audio_refs", func(t *testing.T) {
		t.Parallel()
		requireUnifiedMatchesNative(t, PlatformJijiuSeedance,
			`{
				"model": "`+jijiuTestModelC+`", "prompt": "lake", "duration": 6,
				"assets": [
					{"type": "video", "url": "https://cdn.example.com/run.mp4"},
					{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
				]
			}`,
			`{
				"model": "`+jijiuTestModelC+`", "prompt": "lake", "duration": 6,
				"video_refs": ["https://cdn.example.com/run.mp4"],
				"audio_refs": ["https://cdn.example.com/voice.mp3"]
			}`,
		)
	})
	t.Run("the 480p-only F line", func(t *testing.T) {
		t.Parallel()
		requireUnifiedMatchesNative(t, PlatformJijiuSeedance,
			`{"model":"`+jijiuTestModelF+`","prompt":"lake","duration":4,"assets":[{"type":"audio","url":"https://cdn.example.com/v.mp3"}]}`,
			`{"model":"`+jijiuTestModelF+`","prompt":"lake","duration":4,"audios":["https://cdn.example.com/v.mp3"]}`,
		)
	})
	t.Run("a model with a resolution choice", func(t *testing.T) {
		t.Parallel()
		requireUnifiedMatchesNative(t, PlatformJijiuSeedance,
			`{"model":"`+jijiuTestModel933+`","prompt":"lake","duration":10,"resolution":"480p"}`,
			`{"model":"`+jijiuTestModel933+`","prompt":"lake","duration":10,"resolution":"480p"}`,
		)
	})
}

func TestUnifiedJijiuRequestReachesTheUpstreamWithMappedModel(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformJijiuSeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{jijiuTestModel301: jijiuTestModel30},
		},
	}
	translated, err := TranslatePPVideoUnifiedRequest(PlatformJijiuSeedance, []byte(`{
		"model": "`+jijiuTestModel301+`", "prompt": "lake", "duration": 5,
		"assets": [{"type": "image", "url": "https://cdn.example.com/a.png"}]
	}`))
	require.NoError(t, err)

	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, translated)

	require.NoError(t, err)
	require.Equal(t, jijiuPublic301, public.Model)
	require.Equal(t, jijiuTestModel30, public.UpstreamModel)
	require.Equal(t, jijiuTestModel30, gjson.GetBytes(prepared, "model").String())
	require.Equal(t, "https://cdn.example.com/a.png", gjson.GetBytes(prepared, "images.0").String())
}

func TestTranslatePPVideoUnifiedRequestRejectsInvalidJijiuRequests(t *testing.T) {
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
		{"missing prompt", `{"model":"m","duration":5}`, "prompt: is required for this model"},
		{"missing prompt with assets", `{"model":"m","duration":5,"assets":[` + reference + `]}`, "prompt: is required for this model"},
		{"missing duration", `{"model":"m","prompt":"x"}`, "duration: is required for this model group"},
		{"first frame", `{"model":"m","prompt":"x","duration":5,"assets":[` + firstFrame + `]}`, "assets[0].role: first_frame is not supported by this model"},
		{"last frame", `{"model":"m","prompt":"x","duration":5,"assets":[` + reference + `,` + lastFrame + `]}`, "assets[1].role: last_frame is not supported by this model"},
		{"generate_audio", `{"model":"m","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio: is not supported by this model"},
		{"seed", `{"model":"m","prompt":"x","duration":5,"seed":1}`, "seed: is not supported by this model"},
		{"watermark", `{"model":"m","prompt":"x","duration":5,"watermark":true}`, "watermark: is not supported by this model"},
		{"callback_url", `{"model":"m","prompt":"x","duration":5,"callback_url":"https://example.com/cb"}`, "callback_url: is not supported by this model"},
		{"metadata", `{"model":"m","prompt":"x","duration":5,"metadata":{"a":1}}`, "metadata: is not supported by this model"},
		{"option", `{"model":"m","prompt":"x","duration":5,"options":{"foo":1}}`, "options.foo: is not a supported option for this model"},
		{"duplicate asset names are still refused", `{"model":"m","prompt":"x","duration":5,"assets":[{"type":"image","name":"a.png","url":"https://cdn.example.com/1.png"},{"type":"image","name":"a.png","url":"https://cdn.example.com/2.png"}]}`, "assets[1].name"},
		{"native field names are not accepted", `{"model":"m","prompt":"x","duration":5,"aspect_ratio":"16:9"}`, "aspect_ratio: unknown field"},
		{"native media fields are not accepted", `{"model":"m","prompt":"x","duration":5,"images":["https://cdn.example.com/a.png"]}`, "images: unknown field"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := TranslatePPVideoUnifiedRequest(PlatformJijiuSeedance, []byte(tc.body))
			require.Error(t, err)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// The per-model limits are enforced by the native normalizer after translation, so a unified
// request fails with the same message as a native one.
func TestUnifiedJijiuRequestIsCheckedAgainstTheModelLikeNative(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"resolution the model does not have", `{"model":"` + jijiuTestModelA + `","prompt":"x","duration":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		{"length outside the range", `{"model":"` + jijiuTestModel933 + `","prompt":"x","duration":20}`, "from 4 to 15 seconds"},
		{"reference video on a model without them", `{"model":"` + jijiuTestModelA + `","prompt":"x","duration":5,"assets":[{"type":"video","url":"https://cdn.example.com/a.mp4"}]}`, "does not support reference videos"},
		{"too many images", `{"model":"` + jijiuTestModel933 + `","prompt":"x","duration":5,"assets":` + ppVideoUnifiedTestImageAssets(10) + `}`, "at most 9 reference images"},
		{"an unknown model", `{"model":"gpt-image-2.6","prompt":"x","duration":5}`, "is not available"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			translated, err := TranslatePPVideoUnifiedRequest(PlatformJijiuSeedance, []byte(tc.body))
			require.NoError(t, err)
			_, _, err = PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric, translated)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
