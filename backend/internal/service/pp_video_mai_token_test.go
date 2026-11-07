package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMaiTokenSeedanceProtocol(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformMaiTokenSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformMaiTokenSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)
	headers, err := PPVideoRequestHeaders(PlatformMaiTokenSeedance, "mai-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer mai-key", headers.Get("Authorization"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))
}

func TestMaiTokenSeedanceResponseStatesAndPublicURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantStatus string
	}{
		{name: "queued", body: `{"id":"task-1","status":"queued","result":{"url":"https://api.mai-token.com/v1/videos/task-1/content"}}`, wantStatus: PPVideoTaskStatusProcessing},
		{name: "in progress", body: `{"task_id":"task-2","status":"in_progress"}`, wantStatus: PPVideoTaskStatusProcessing},
		{name: "completed", body: `{"id":"task-3","status":"completed","usage":{"output_video_duration":5,"reference_video_duration":5.06195,"video_count":1},"result":{"url":"https://api.mai-token.com/v1/videos/task-3/content"}}`, wantStatus: PPVideoTaskStatusSucceeded},
		{name: "completed without URL", body: `{"id":"task-4","status":"completed","usage":{"output_video_duration":5}}`, wantStatus: PPVideoTaskStatusSucceeded},
		{name: "failed", body: `{"id":"task-5","status":"failed","error":{"message":"rejected"}}`, wantStatus: PPVideoTaskStatusFailed},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := ParsePPVideoResponse(PlatformMaiTokenSeedance, []byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, parsed.Status)
			require.NotEmpty(t, parsed.TaskID)
			public := NormalizePPVideoPublicResponse(
				PlatformMaiTokenSeedance,
				[]byte(tt.body),
				PPVideoPublicRequest{Model: MaiTokenSeedanceModel},
				parsed,
			)
			if tt.wantStatus == PPVideoTaskStatusSucceeded {
				require.Equal(t, "/v1/videos/"+parsed.TaskID+"/content", gjson.GetBytes(public, "result.url").String())
				require.NotContains(t, string(public), "api.mai-token.com")
			} else {
				require.False(t, gjson.GetBytes(public, "result.url").Exists())
				require.False(t, gjson.GetBytes(public, "video_url").Exists())
			}
		})
	}
}

func TestMaiTokenSeedanceModelSyncDefaultsToDocumentedBaseURL(t *testing.T) {
	t.Parallel()
	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	req, err := svc.buildPPVideoUpstreamModelsRequest(context.Background(), &Account{
		Platform: PlatformMaiTokenSeedance,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "mai-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.mai-token.com/v1/models", req.URL.String())
	require.Equal(t, "Bearer mai-key", req.Header.Get("Authorization"))
}

func TestForwardMaiTokenSeedanceSubmissionUsesMultipartAndStableIdempotency(t *testing.T) {
	req := newMaiTokenMultipartTestRequest(t, "client-boundary", map[string][]string{
		"model": {MaiTokenSeedanceModel}, "prompt": {"lake"}, "duration": {"4"},
	}, []maiTokenMultipartTestUpload{{Field: "first_image", Name: "first.jpg", ContentType: "image/jpeg", Body: "jpeg-bytes"}})
	submission, err := PrepareMaiTokenMultipartSubmission(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = submission.Cleanup() })
	upstream := &httpUpstreamRecorder{responses: []*http.Response{{
		StatusCode: http.StatusAccepted,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"id":"mai-task-1","status":"queued"}`)),
	}}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{ID: 77, Platform: PlatformMaiTokenSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "mai-key"}}
	public := PPVideoPublicRequest{Model: MaiTokenSeedanceModel, UpstreamModel: MaiTokenSeedanceModel, DurationMilliseconds: 4000, VideoCount: 1, Resolution: "480p", UpstreamIdempotencyKey: "video-local-1"}

	result, err := svc.ForwardPPVideoSubmissionBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "", submission, public)
	require.NoError(t, err)
	require.Equal(t, "mai-task-1", result.ResponseID)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.mai-token.com/v1/videos", upstream.requests[0].URL.String())
	require.Equal(t, "video-local-1", upstream.requests[0].Header.Get("Idempotency-Key"))
	require.Contains(t, upstream.requests[0].Header.Get("Content-Type"), "multipart/form-data; boundary=")
	require.Contains(t, string(upstream.bodies[0]), "jpeg-bytes")
}

func TestMaiTokenSeedancePlatformRegistration(t *testing.T) {
	t.Parallel()

	require.Equal(t, "mai-token-seedance", PlatformMaiTokenSeedance)
	require.Equal(t, "https://api.mai-token.com", MaiTokenSeedanceDefaultBaseURL)
	require.Equal(t, "sdtj-2.5-480p", MaiTokenSeedanceModel)
	require.True(t, IsPPVideoPlatform(PlatformMaiTokenSeedance))
	require.True(t, IsAllowedQuotaPlatform(PlatformMaiTokenSeedance))
}

func TestMaiTokenSeedanceFiltersExactModel(t *testing.T) {
	t.Parallel()

	models := []string{
		"sdtj-2.5-480p",
		"SDTJ-2.5-480P",
		"seedance-2.5",
		"gpt-image-2",
	}
	want := []string{"sdtj-2.5-480p"}
	require.Equal(t, want, filterUpstreamModelsForPlatform(PlatformMaiTokenSeedance, models))
	require.Equal(t, want, filterPPVideoPublicModelsForPlatform(PlatformMaiTokenSeedance, models))
}

func TestPrepareMaiTokenSeedanceVideoV1Request(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"contract_version":"video-v1",
		"model":"sdtj-2.5-480p",
		"prompt":"lake",
		"ratio":"3:4",
		"references":[
			{"type":"image","role":"reference_image","source":"https://example.com/a.jpg"},
			{"type":"video","role":"reference_video","source":"https://example.com/a.mp4"},
			{"type":"audio","role":"reference_audio","source":"https://example.com/a.mp3"}
		],
		"options":{"generate_audio":false}
	}`)

	prepared, public, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, body)
	require.NoError(t, err)
	require.Equal(t, MaiTokenSeedanceModel, public.Model)
	require.Equal(t, MaiTokenSeedanceModel, public.UpstreamModel)
	require.Equal(t, int64(4000), public.DurationMilliseconds)
	require.Equal(t, "480p", public.Resolution)
	require.Equal(t, 1, public.VideoCount)
	require.JSONEq(t, `{
		"model":"sdtj-2.5-480p",
		"prompt":"lake",
		"duration":4,
		"resolution":"480p",
		"ratio":"3:4",
		"referenceImages":["https://example.com/a.jpg"],
		"referenceVideos":["https://example.com/a.mp4"],
		"referenceAudios":["https://example.com/a.mp3"],
		"generate_audio":false
	}`, string(prepared))
}

func TestPrepareMaiTokenSeedanceNativeAliases(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"sdtj-2.5-480p",
		"prompt":"lake",
		"seconds":6,
		"aspect_ratio":"9:16",
		"images":["https://example.com/a.jpg","https://example.com/a.jpg"],
		"video_url":"https://example.com/a.mp4",
		"audioUrls":["https://example.com/a.mp3"],
		"generateAudio":false,
		"watermark":false
	}`)

	prepared, public, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, body)
	require.NoError(t, err)
	require.Equal(t, int64(6000), public.DurationMilliseconds)
	require.JSONEq(t, `{
		"model":"sdtj-2.5-480p",
		"prompt":"lake",
		"duration":6,
		"resolution":"480p",
		"ratio":"9:16",
		"referenceImages":["https://example.com/a.jpg","https://example.com/a.jpg"],
		"referenceVideos":["https://example.com/a.mp4"],
		"referenceAudios":["https://example.com/a.mp3"],
		"generate_audio":false,
		"watermark":false
	}`, string(prepared))
}

func TestPrepareMaiTokenSeedanceLegacyContentAndFrames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "legacy content",
			body: `{
				"model":"sdtj-2.5-480p",
				"content":[
					{"type":"text","text":"lake"},
					{"type":"image_url","image_url":{"url":"https://example.com/a.jpg"}},
					{"type":"video_url","video_url":{"url":"https://example.com/a.mp4"}},
					{"type":"audio_url","audio_url":{"url":"https://example.com/a.mp3"}}
				]
			}`,
			want: `{
				"model":"sdtj-2.5-480p",
				"prompt":"lake",
				"duration":4,
				"resolution":"480p",
				"referenceImages":["https://example.com/a.jpg"],
				"referenceVideos":["https://example.com/a.mp4"],
				"referenceAudios":["https://example.com/a.mp3"]
			}`,
		},
		{
			name: "first and last frames",
			body: `{
				"model":"sdtj-2.5-480p",
				"prompt":"lake",
				"start_image_url":"https://example.com/first.jpg",
				"end_image_url":"https://example.com/last.jpg"
			}`,
			want: `{
				"model":"sdtj-2.5-480p",
				"prompt":"lake",
				"duration":4,
				"resolution":"480p",
				"first_image":"https://example.com/first.jpg",
				"last_image":"https://example.com/last.jpg"
			}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, []byte(tt.body))
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(prepared))
		})
	}
}

func TestPrepareMaiTokenSeedanceRejectsInvalidCoreFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing model", body: `{"prompt":"lake"}`, want: "model"},
		{name: "model", body: `{"model":"seedance-2.5","prompt":"lake"}`, want: "model"},
		{name: "prompt", body: `{"model":"sdtj-2.5-480p","prompt":" "}`, want: "prompt"},
		{name: "duration below minimum", body: `{"model":"sdtj-2.5-480p","prompt":"lake","duration":3}`, want: "duration"},
		{name: "duration above maximum", body: `{"model":"sdtj-2.5-480p","prompt":"lake","duration":31}`, want: "duration"},
		{name: "fractional duration", body: `{"model":"sdtj-2.5-480p","prompt":"lake","duration":4.5}`, want: "duration"},
		{name: "string duration", body: `{"model":"sdtj-2.5-480p","prompt":"lake","duration":"4"}`, want: "duration"},
		{name: "resolution", body: `{"model":"sdtj-2.5-480p","prompt":"lake","resolution":"720p"}`, want: "resolution"},
		{name: "resolution must be canonical", body: `{"model":"sdtj-2.5-480p","prompt":"lake","resolution":"480P"}`, want: "resolution"},
		{name: "watermark type", body: `{"model":"sdtj-2.5-480p","prompt":"lake","watermark":0}`, want: "boolean"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, []byte(tt.body))
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestPrepareMaiTokenSeedanceRejectsUnsupportedFields(t *testing.T) {
	t.Parallel()

	for _, field := range []string{
		"seed", "draft", "callback", "callback_url", "callback_token",
		"return_last_frame", "n", "count",
	} {
		field := field
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			body := []byte(`{"model":"sdtj-2.5-480p","prompt":"lake","` + field + `":1}`)
			_, _, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, body)
			require.ErrorContains(t, err, field)
		})
	}
}

func TestPrepareMaiTokenSeedanceRejectsUnsafeReferenceURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{name: "data URI", url: "data:image/png;base64,AAAA"},
		{name: "file URI", url: "file:///tmp/a.jpg"},
		{name: "unsupported scheme", url: "ftp://example.com/a.jpg"},
		{name: "localhost", url: "http://localhost/a.jpg"},
		{name: "private IPv4", url: "http://10.0.0.1/a.jpg"},
		{name: "loopback IPv6", url: "http://[::1]/a.jpg"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload := map[string]any{
				"model":           MaiTokenSeedanceModel,
				"prompt":          "lake",
				"referenceImages": []string{tt.url},
			}
			body, err := json.Marshal(payload)
			require.NoError(t, err)
			_, _, err = PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, body)
			require.ErrorContains(t, err, "public HTTP(S) URL")
		})
	}
}

func TestPrepareMaiTokenSeedanceInputReferenceRemainsReferenceImage(t *testing.T) {
	t.Parallel()

	prepared, _, err := PreparePPVideoRequestBody(
		PlatformMaiTokenSeedance,
		PPVideoOperationGeneric,
		[]byte(`{"model":"sdtj-2.5-480p","prompt":"lake","input_reference":"https://example.com/reference.jpg"}`),
	)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"sdtj-2.5-480p",
		"prompt":"lake",
		"duration":4,
		"resolution":"480p",
		"referenceImages":["https://example.com/reference.jpg"]
	}`, string(prepared))
}

func TestPrepareMaiTokenSeedanceRejectsMalformedAndConflictingReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "references object",
			body: `{"model":"sdtj-2.5-480p","prompt":"lake","references":{"type":"image","role":"reference_image","source":"https://example.com/a.jpg"}}`,
			want: "references",
		},
		{
			name: "different first frame aliases",
			body: `{"model":"sdtj-2.5-480p","prompt":"lake","first_image":"https://example.com/a.jpg","start_frame":"https://example.com/b.jpg"}`,
			want: "first frame",
		},
		{
			name: "last frame without first",
			body: `{"model":"sdtj-2.5-480p","prompt":"lake","last_image":"https://example.com/b.jpg"}`,
			want: "requires first_image",
		},
		{
			name: "frame and reference media",
			body: `{"model":"sdtj-2.5-480p","prompt":"lake","first_image":"https://example.com/a.jpg","referenceImages":["https://example.com/reference.jpg"]}`,
			want: "cannot combine",
		},
		{
			name: "string boolean",
			body: `{"model":"sdtj-2.5-480p","prompt":"lake","generate_audio":"false"}`,
			want: "boolean",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, []byte(tt.body))
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestPrepareMaiTokenSeedanceRejectsReferenceLimits(t *testing.T) {
	t.Parallel()

	makeURLs := func(count int, suffix string) []string {
		values := make([]string, count)
		for index := range values {
			values[index] = "https://example.com/" + string(rune('a'+index%26)) + suffix
		}
		return values
	}
	tests := []struct {
		field string
		items []string
		want  string
	}{
		{field: "referenceImages", items: makeURLs(31, ".jpg"), want: "30 reference images"},
		{field: "referenceVideos", items: makeURLs(11, ".mp4"), want: "10 reference videos"},
		{field: "referenceAudios", items: makeURLs(11, ".mp3"), want: "10 reference audios"},
	}
	for _, tt := range tests {
		payload := map[string]any{"model": MaiTokenSeedanceModel, "prompt": "lake", tt.field: tt.items}
		body, err := json.Marshal(payload)
		require.NoError(t, err)
		_, _, err = PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, body)
		require.ErrorContains(t, err, tt.want)
	}
}

func TestPrepareMaiTokenSeedanceRejectsNonGenerationOperation(t *testing.T) {
	t.Parallel()

	_, _, err := PreparePPVideoRequestBody(
		PlatformMaiTokenSeedance,
		PPVideoOperationCancel,
		[]byte(`{"model":"sdtj-2.5-480p","prompt":"lake"}`),
	)
	require.ErrorContains(t, err, "does not support operation")
}
