package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPPVideoUpstreamPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		platform  string
		operation PPVideoOperation
		taskID    string
		want      string
	}{
		{
			name:      "Seedance submission",
			platform:  PlatformSeedance,
			operation: PPVideoOperationGeneric,
			want:      "/v1/video/generations",
		},
		{
			name:      "Happy Horse task status",
			platform:  PlatformHappyHourse,
			operation: PPVideoOperationGeneric,
			taskID:    "horse-task-1",
			want:      "/v1/video/generations/horse-task-1",
		},
		{
			name:      "Kling text to video task status",
			platform:  PlatformKling,
			operation: PPVideoOperationKlingTextToVideo,
			taskID:    "kling-task-1",
			want:      "/v1/videos/text2video/kling-task-1",
		},
		{
			name:      "Kling image to video submission",
			platform:  PlatformKling,
			operation: PPVideoOperationKlingImageToVideo,
			want:      "/v1/videos/image2video",
		},
		{
			name:      "ByteDance submission",
			platform:  PlatformByteDance,
			operation: PPVideoOperationGeneric,
			want:      "/v1/tasks/submit",
		},
		{
			name:      "ByteDance status",
			platform:  PlatformByteDance,
			operation: PPVideoOperationGeneric,
			taskID:    "task/1",
			want:      "/v1/tasks/status?task_id=task%2F1",
		},
		{
			name:      "Wan3.0 submission",
			platform:  PlatformWan3,
			operation: PPVideoOperationGeneric,
			want:      "/v1/tasks/submit",
		},
		{
			name:      "Wan3.0 status",
			platform:  PlatformWan3,
			operation: PPVideoOperationGeneric,
			taskID:    "task/1",
			want:      "/v1/tasks/status?task_id=task%2F1",
		},
		{
			name:      "MiniMax-H3 submission",
			platform:  PlatformMiniMaxH3,
			operation: PPVideoOperationGeneric,
			want:      "/v1/tasks/submit",
		},
		{
			name:      "MiniMax-H3 status",
			platform:  PlatformMiniMaxH3,
			operation: PPVideoOperationGeneric,
			taskID:    "task/1",
			want:      "/v1/tasks/status?task_id=task%2F1",
		},
		{
			name:      "Pixverse v6 submission",
			platform:  PlatformPixverseV6,
			operation: PPVideoOperationGeneric,
			want:      "/v1/tasks/submit",
		},
		{
			name:      "Pixverse v6 status",
			platform:  PlatformPixverseV6,
			operation: PPVideoOperationGeneric,
			taskID:    "task/1",
			want:      "/v1/tasks/status?task_id=task%2F1",
		},
		{
			name:      "Grok Imagine Video submission",
			platform:  PlatformGrokImagineVideo,
			operation: PPVideoOperationGeneric,
			want:      "/v1/tasks/submit",
		},
		{
			name:      "Grok Imagine Video status",
			platform:  PlatformGrokImagineVideo,
			operation: PPVideoOperationGeneric,
			taskID:    "task/1",
			want:      "/v1/tasks/status?task_id=task%2F1",
		},
		{
			name:      "ByteDance cancellation",
			platform:  PlatformByteDance,
			operation: PPVideoOperationCancel,
			taskID:    "task/1",
			want:      "/v1/tasks/cancel?task_id=task%2F1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path, err := PPVideoUpstreamPath(tt.platform, tt.operation, tt.taskID)
			require.NoError(t, err)
			require.Equal(t, tt.want, path)
		})
	}
}

func TestPPVideoStatusUpstreamPathRejectsBlankTaskID(t *testing.T) {
	t.Parallel()

	_, err := PPVideoStatusUpstreamPath(PlatformSeedance, PPVideoOperationGeneric, "")
	require.ErrorContains(t, err, "task id is required")

	_, err = PPVideoUpstreamPath(PlatformKling, PPVideoOperationGeneric, "")
	require.ErrorContains(t, err, "does not support operation")

	_, err = PPVideoUpstreamPath("openai", PPVideoOperationGeneric, "")
	require.ErrorContains(t, err, "unsupported PP video platform")
}

func TestPPVideoRequestHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		platform  string
		wantAsync string
	}{
		{name: "Seedance", platform: PlatformSeedance},
		{name: "Happy Horse", platform: PlatformHappyHourse, wantAsync: "enable"},
		{name: "Kling", platform: PlatformKling},
		{name: "ByteDance", platform: PlatformByteDance},
		{name: "Wan3.0", platform: PlatformWan3},
		{name: "MiniMax-H3", platform: PlatformMiniMaxH3},
		{name: "Pixverse v6", platform: PlatformPixverseV6},
		{name: "Grok Imagine Video", platform: PlatformGrokImagineVideo},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			headers, err := PPVideoRequestHeaders(tt.platform, "pp-key")
			require.NoError(t, err)
			require.Equal(t, "Bearer pp-key", headers.Get("Authorization"))
			require.Equal(t, "application/json", headers.Get("Accept"))
			require.Equal(t, "application/json", headers.Get("Content-Type"))
			require.Equal(t, tt.wantAsync, headers.Get("X-DashScope-Async"))
		})
	}
}

func TestPPVideoStatusRequestHeadersDoesNotUseSubmissionHeader(t *testing.T) {
	t.Parallel()

	headers, err := PPVideoStatusRequestHeaders(PlatformHappyHourse, "pp-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer pp-key", headers.Get("Authorization"))
	require.Empty(t, headers.Get("X-DashScope-Async"))
}

func TestSiteVideoDefaultModelUsesJimengUpdatedDefault(t *testing.T) {
	t.Parallel()

	require.Equal(t, "by-seedance2.0-933", SiteVideoDefaultModel)
}

func TestParsePPVideoResponseRecognizesTaskIDsAndStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		platform   string
		body       string
		wantTaskID string
		wantStatus string
	}{
		{
			name:       "top level id queued",
			platform:   PlatformSeedance,
			body:       `{"id":"seedance-1","status":"queued"}`,
			wantTaskID: "seedance-1",
			wantStatus: PPVideoTaskStatusProcessing,
		},
		{
			name:       "top level generation id queued",
			platform:   PlatformSeedance,
			body:       `{"generationId":"seedance-1b","status":"queued"}`,
			wantTaskID: "seedance-1b",
			wantStatus: PPVideoTaskStatusProcessing,
		},
		{
			name:       "top level task id failed",
			platform:   PlatformHappyHourse,
			body:       `{"task_id":"horse-1","status":"CANCELLED"}`,
			wantTaskID: "horse-1",
			wantStatus: PPVideoTaskStatusFailed,
		},
		{
			name:       "data id complete",
			platform:   PlatformSeedance,
			body:       `{"data":{"id":"seedance-2","status":"completed"}}`,
			wantTaskID: "seedance-2",
			wantStatus: PPVideoTaskStatusSucceeded,
		},
		{
			name:       "data task id success",
			platform:   PlatformHappyHourse,
			body:       `{"data":{"task_id":"horse-2","status":"SUCCESS"}}`,
			wantTaskID: "horse-2",
			wantStatus: PPVideoTaskStatusSucceeded,
		},
		{
			name:       "data request id running",
			platform:   PlatformSeedance,
			body:       `{"data":{"request_id":"seedance-3","status":"running"}}`,
			wantTaskID: "seedance-3",
			wantStatus: PPVideoTaskStatusProcessing,
		},
		{
			name:       "data generation id running",
			platform:   PlatformSeedance,
			body:       `{"data":{"generation_id":"seedance-4","status":"running"}}`,
			wantTaskID: "seedance-4",
			wantStatus: PPVideoTaskStatusProcessing,
		},
		{
			name:       "Kling final video URL promotes processing status",
			platform:   PlatformKling,
			body:       `{"data":{"task_id":"kling-1","status":"processing","task_result":{"videos":[{"url":"https://cdn.example.com/kling.mp4"}]}}}`,
			wantTaskID: "kling-1",
			wantStatus: PPVideoTaskStatusSucceeded,
		},
		{
			name:       "Kling failed status is not promoted by stale video URL",
			platform:   PlatformKling,
			body:       `{"data":{"task_id":"kling-2","status":"FAILED","task_result":{"videos":[{"url":"https://cdn.example.com/kling.mp4"}]}}}`,
			wantTaskID: "kling-2",
			wantStatus: PPVideoTaskStatusFailed,
		},
		{
			name:       "Kling official succeed status",
			platform:   PlatformKling,
			body:       `{"data":{"task_id":"kling-3","task_status":"succeed"}}`,
			wantTaskID: "kling-3",
			wantStatus: PPVideoTaskStatusSucceeded,
		},
		{
			name:       "ByteDance output task status",
			platform:   PlatformByteDance,
			body:       `{"output":{"task_id":"bd-1","task_status":"Success"}}`,
			wantTaskID: "bd-1",
			wantStatus: PPVideoTaskStatusSucceeded,
		},
		{
			name:       "Wan3.0 output task status",
			platform:   PlatformWan3,
			body:       `{"output":{"task_id":"wan-task-1","task_status":"Success"}}`,
			wantTaskID: "wan-task-1",
			wantStatus: PPVideoTaskStatusSucceeded,
		},
		{
			name:       "Pixverse v6 output task status",
			platform:   PlatformPixverseV6,
			body:       `{"output":{"task_id":"pix-task-1","task_status":"Success"}}`,
			wantTaskID: "pix-task-1",
			wantStatus: PPVideoTaskStatusSucceeded,
		},
		{
			name:       "Grok Imagine Video output task status",
			platform:   PlatformGrokImagineVideo,
			body:       `{"output":{"task_id":"grok-task-1","task_status":"Running"}}`,
			wantTaskID: "grok-task-1",
			wantStatus: PPVideoTaskStatusProcessing,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParsePPVideoResponse(tt.platform, []byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.wantTaskID, result.TaskID)
			require.Equal(t, tt.wantStatus, result.Status)
			require.Equal(t, []byte(tt.body), result.RawBody)
		})
	}
}

func TestParsePPVideoResponseExtractsByteDanceUsageAndFailure(t *testing.T) {
	t.Parallel()

	success, err := ParsePPVideoResponse(PlatformByteDance, []byte(`{
		"output": {
			"task_id": "bd-task-1",
			"task_status": "Success",
			"urls": ["https://cdn.example.com/bytedance.mp4"]
		},
		"usage": {"duration": 5.25}
	}`))
	require.NoError(t, err)
	require.Equal(t, "bd-task-1", success.TaskID)
	require.Equal(t, PPVideoTaskStatusSucceeded, success.Status)
	require.Equal(t, int64(5250), success.GeneratedDurationMilliseconds)

	failure, err := ParsePPVideoResponse(PlatformByteDance, []byte(`{
		"output": {
			"task_id": "bd-task-2",
			"task_status": "Failure",
			"error_message": "content policy rejected"
		}
	}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusFailed, failure.Status)
	require.Equal(t, "content policy rejected", failure.ErrorMessage)
}

func TestParsePPVideoResponseUsesMiniMaxH3ActualOutputDuration(t *testing.T) {
	t.Parallel()

	result, err := ParsePPVideoResponse(PlatformMiniMaxH3, []byte(`{
		"output": {
			"task_id": "h3-task-1",
			"task_status": "Success",
			"urls": ["https://cdn.example.com/minimax-h3.mp4"]
		},
		"usage": {
			"duration": 10,
			"output_video_duration": 9.25,
			"video_count": 1,
			"ratio": "16:9"
		}
	}`))

	require.NoError(t, err)
	require.Equal(t, "h3-task-1", result.TaskID)
	require.Equal(t, PPVideoTaskStatusSucceeded, result.Status)
	require.Equal(t, int64(9250), result.GeneratedDurationMilliseconds)
	require.Equal(t, 1, result.VideoCount)
}

func TestParsePPVideoResponseUsesWan30ActualOutputUsage(t *testing.T) {
	t.Parallel()

	result, err := ParsePPVideoResponse(PlatformWan3, []byte(`{
		"output": {
			"task_id": "wan-task-1",
			"task_status": "Success",
			"urls": ["https://cdn.example.com/wan.mp4"]
		},
		"usage": {
			"duration": 10,
			"output_video_duration": 8.25,
			"video_count": 2,
			"SR": "720P"
		}
	}`))

	require.NoError(t, err)
	require.Equal(t, "wan-task-1", result.TaskID)
	require.Equal(t, PPVideoTaskStatusSucceeded, result.Status)
	require.Equal(t, int64(8250), result.GeneratedDurationMilliseconds)
	require.Equal(t, 2, result.VideoCount)
	require.Equal(t, VideoBillingResolution720P, result.VideoResolution)
}

func TestParsePPVideoResponseUsesPixverseV6UsageAndFailure(t *testing.T) {
	t.Parallel()

	success, err := ParsePPVideoResponse(PlatformPixverseV6, []byte(`{
		"output": {
			"task_id": "pix-task-1",
			"task_status": "Success",
			"urls": ["https://cdn.example.com/pixverse-v6.mp4"]
		},
		"usage": {"duration": 5}
	}`))
	require.NoError(t, err)
	require.Equal(t, "pix-task-1", success.TaskID)
	require.Equal(t, PPVideoTaskStatusSucceeded, success.Status)
	require.Equal(t, int64(5000), success.GeneratedDurationMilliseconds)

	failure, err := ParsePPVideoResponse(PlatformPixverseV6, []byte(`{
		"output": {
			"task_id": "pix-task-2",
			"task_status": "Failure",
			"error_message": "invalid media"
		}
	}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusFailed, failure.Status)
	require.Equal(t, "invalid media", failure.ErrorMessage)
}

func TestParsePPVideoResponseUsesGrokImagineVideoUsageAndFailure(t *testing.T) {
	t.Parallel()

	success, err := ParsePPVideoResponse(PlatformGrokImagineVideo, []byte(`{
		"output": {
			"task_id": "grok-task-1",
			"task_status": "Success",
			"urls": ["https://cdn.example.com/grok.mp4"]
		},
		"usage": {"duration": 8, "output_video_duration": 7.5}
	}`))
	require.NoError(t, err)
	require.Equal(t, "grok-task-1", success.TaskID)
	require.Equal(t, PPVideoTaskStatusSucceeded, success.Status)
	require.Equal(t, int64(7500), success.GeneratedDurationMilliseconds)

	failure, err := ParsePPVideoResponse(PlatformGrokImagineVideo, []byte(`{
		"output": {
			"task_id": "grok-task-2",
			"task_status": "Failure",
			"error_message": "invalid reference media"
		}
	}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusFailed, failure.Status)
	require.Equal(t, "invalid reference media", failure.ErrorMessage)
}

func TestParsePPVideoResponseExtractsProviderMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		platform       string
		body           string
		wantTaskID     string
		wantDuration   int64
		wantVideoCount int
		wantResolution string
	}{
		{
			name:     "Kling nested task result string milliseconds",
			platform: PlatformKling,
			body: `{
				"data": {
					"task_id": "kling-task-1",
					"status": "SUCCESS",
					"data": {
						"task_result": {
							"videos": [{"duration": "5.041"}]
						}
					}
				}
			}`,
			wantTaskID:     "kling-task-1",
			wantDuration:   5041,
			wantVideoCount: 1,
		},
		{
			name:     "Happy Horse usage number duration",
			platform: PlatformHappyHourse,
			body: `{
				"data": {
					"id": "horse-task-1",
					"status": "SUCCESS",
					"data": {
						"usage": {
							"duration": 10.25,
							"video_count": 2,
							"SR": "720P"
						}
					}
				}
			}`,
			wantTaskID:     "horse-task-1",
			wantDuration:   10250,
			wantVideoCount: 2,
			wantResolution: VideoBillingResolution720P,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParsePPVideoResponse(tt.platform, []byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.wantTaskID, result.TaskID)
			require.Equal(t, tt.wantDuration, result.GeneratedDurationMilliseconds)
			require.Equal(t, tt.wantVideoCount, result.VideoCount)
			require.Equal(t, tt.wantResolution, result.VideoResolution)
		})
	}
}

func TestPPVideoBillingMetadataFromRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		platform                 string
		body                     string
		wantDurationMilliseconds int64
		wantDurationSeconds      int
		wantVideoCount           int
		wantResolution           string
		wantKlingMode            string
		wantValidationError      string
	}{
		{
			name:                     "Kling accepts string duration and pro mode",
			platform:                 PlatformKling,
			body:                     `{"model_name":"kling-v3","mode":"pro","duration":"10","video_count":2}`,
			wantDurationMilliseconds: 10000,
			wantDurationSeconds:      10,
			wantVideoCount:           2,
			wantResolution:           VideoBillingResolution1080P,
			wantKlingMode:            "pro",
		},
		{
			name:                     "Kling defaults a missing mode to std",
			platform:                 PlatformKling,
			body:                     `{"model_name":"kling-v3","duration":"10"}`,
			wantDurationMilliseconds: 10000,
			wantDurationSeconds:      10,
			wantVideoCount:           1,
			wantResolution:           VideoBillingResolution720P,
			wantKlingMode:            "std",
		},
		{
			name:                     "Kling derives pro mode from 1080p resolution",
			platform:                 PlatformKling,
			body:                     `{"model_name":"kling-v3","duration":"10","resolution":"1080p"}`,
			wantDurationMilliseconds: 10000,
			wantDurationSeconds:      10,
			wantVideoCount:           1,
			wantResolution:           VideoBillingResolution1080P,
			wantKlingMode:            "pro",
		},
		{
			name:                     "Kling derives 4k mode from resolution",
			platform:                 PlatformKling,
			body:                     `{"model_name":"kling-v3","duration":"10","resolution":"4k"}`,
			wantDurationMilliseconds: 10000,
			wantDurationSeconds:      10,
			wantVideoCount:           1,
			wantResolution:           "4k",
			wantKlingMode:            "4k",
		},
		{
			name:                     "Kling accepts 2x pro billing alias",
			platform:                 PlatformKling,
			body:                     `{"model_name":"kling-v3","mode":"2x-pro","duration":"10"}`,
			wantDurationMilliseconds: 10000,
			wantDurationSeconds:      10,
			wantVideoCount:           1,
			wantResolution:           VideoBillingResolution1080P,
			wantKlingMode:            "2x-pro",
		},
		{
			name:                     "Kling rejects an unknown mode",
			platform:                 PlatformKling,
			body:                     `{"model_name":"kling-v3","mode":"ultra","duration":"10"}`,
			wantDurationMilliseconds: 10000,
			wantDurationSeconds:      10,
			wantVideoCount:           1,
			wantKlingMode:            "ultra",
			wantValidationError:      "unsupported Kling mode",
		},
		{
			name:                     "generic metadata reads decimal duration and resolution",
			platform:                 PlatformHappyHourse,
			body:                     `{"duration":5.5,"n":3,"resolution":"480P"}`,
			wantDurationMilliseconds: 5500,
			wantVideoCount:           3,
			wantResolution:           VideoBillingResolution480P,
		},
		{
			name:                     "Seedance reads metadata duration and resolution",
			platform:                 PlatformSeedance,
			body:                     `{"metadata":{"duration":"5.041","resolution":"1080P"}}`,
			wantDurationMilliseconds: 5041,
			wantVideoCount:           1,
			wantResolution:           VideoBillingResolution1080P,
		},
		{
			name:                     "Wan3.0 reads nested ModelVerse parameters",
			platform:                 PlatformWan3,
			body:                     `{"model":"wan3.0-video-prime","parameters":{"duration":8,"resolution":"720P"}}`,
			wantDurationMilliseconds: 8000,
			wantDurationSeconds:      8,
			wantVideoCount:           1,
			wantResolution:           VideoBillingResolution720P,
		},
		{
			name:                     "Kling 4k is a billable mode",
			platform:                 PlatformKling,
			body:                     `{"mode":"4k","duration":"5"}`,
			wantDurationMilliseconds: 5000,
			wantDurationSeconds:      5,
			wantVideoCount:           1,
			wantResolution:           "4k",
			wantKlingMode:            "4k",
		},
		{
			name:                "nonpositive duration is available for handler validation",
			platform:            PlatformSeedance,
			body:                `{"duration":"0","resolution":"720p"}`,
			wantVideoCount:      1,
			wantResolution:      VideoBillingResolution720P,
			wantValidationError: "duration",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			metadata := PPVideoBillingMetadataFromRequest(tt.platform, []byte(tt.body))
			require.Equal(t, tt.wantDurationMilliseconds, metadata.RequestedDurationMilliseconds)
			require.Equal(t, tt.wantDurationSeconds, metadata.RequestedDurationSeconds)
			require.Equal(t, tt.wantVideoCount, metadata.VideoCount)
			require.Equal(t, tt.wantResolution, metadata.VideoResolution)
			require.Equal(t, tt.wantKlingMode, metadata.KlingMode)
			if tt.wantValidationError == "" {
				require.NoError(t, metadata.ValidationError)
				return
			}
			require.ErrorContains(t, metadata.ValidationError, tt.wantValidationError)
		})
	}
}

func TestPPVideoBillingMetadataReadsFormulaInputs(t *testing.T) {
	t.Parallel()

	metadata := PPVideoBillingMetadataFromRequest(PlatformSeedance, []byte(`{
		"model": "doubao-seedance-2-0-260128",
		"duration": 5,
		"input_video_duration": 2.5,
		"width": 1280,
		"height": 720,
		"fps": 24,
		"generate_audio": true
	}`))

	require.NoError(t, metadata.ValidationError)
	require.Equal(t, "doubao-seedance-2-0-260128", metadata.Model)
	require.Equal(t, int64(2500), metadata.InputVideoDurationMilliseconds)
	require.Equal(t, 1280, metadata.OutputWidth)
	require.Equal(t, 720, metadata.OutputHeight)
	require.InDelta(t, 24, metadata.FrameRate, 1e-12)
	require.True(t, metadata.HasAudio)
}

func TestPPVideoBillingMetadataUsesResolutionDefaultsForSeedanceFormula(t *testing.T) {
	t.Parallel()

	metadata := PPVideoBillingMetadataFromRequest(PlatformSeedance, []byte(`{
		"duration": 5,
		"resolution": "1080p",
		"fps": 30
	}`))

	require.NoError(t, metadata.ValidationError)
	require.Equal(t, 1920, metadata.OutputWidth)
	require.Equal(t, 1080, metadata.OutputHeight)
	require.InDelta(t, 30, metadata.FrameRate, 1e-12)
}

func TestPreparePPVideoRequestBodyAppliesDefaultsForPublicVideoRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		platform       string
		operation      PPVideoOperation
		wantModelPath  string
		wantModel      string
		wantDuration   string
		wantResolution string
		wantMode       string
	}{
		{
			name:           "Seedance generic request",
			platform:       PlatformSeedance,
			operation:      PPVideoOperationGeneric,
			wantModelPath:  "model",
			wantModel:      "doubao-seedance-2-0-260128",
			wantDuration:   "5",
			wantResolution: VideoBillingResolution720P,
		},
		{
			name:           "Happy Horse generic request",
			platform:       PlatformHappyHourse,
			operation:      PPVideoOperationGeneric,
			wantModelPath:  "model",
			wantModel:      "happyhorse-1.0-t2v",
			wantDuration:   "5",
			wantResolution: VideoBillingResolution720P,
		},
		{
			name:          "Kling text request",
			platform:      PlatformKling,
			operation:     PPVideoOperationKlingTextToVideo,
			wantModelPath: "model_name",
			wantModel:     "kling-v3",
			wantDuration:  "5",
			wantMode:      "std",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body, public, err := PreparePPVideoRequestBody(tt.platform, tt.operation, []byte(`{"prompt":"waves"}`))
			require.NoError(t, err)
			require.Empty(t, public.Model)
			require.Equal(t, tt.wantModel, public.UpstreamModel)
			require.Equal(t, 5000, int(public.DurationMilliseconds))
			require.Equal(t, VideoBillingResolution720P, public.Resolution)
			require.Equal(t, 1, public.VideoCount)
			require.Equal(t, "waves", gjson.GetBytes(body, "prompt").String())
			require.Equal(t, tt.wantModel, gjson.GetBytes(body, tt.wantModelPath).String())
			require.Equal(t, tt.wantDuration, gjson.GetBytes(body, "duration").String())
			if tt.wantResolution != "" {
				require.Equal(t, tt.wantResolution, gjson.GetBytes(body, "resolution").String())
			}
			if tt.wantMode != "" {
				require.Equal(t, tt.wantMode, gjson.GetBytes(body, "mode").String())
			}
		})
	}
}

func TestPreparePPVideoRequestBodyNormalizesByteDanceModelVersePayload(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformByteDance,
		PPVideoOperationGeneric,
		[]byte(`{
			"model": "video-v1",
			"input": {
				"content": [
					{"type":"text","text":"a cinematic mountain sunrise"},
					{"type":"image_url","image_url":{"url":"https://example.com/start.png"},"role":"first_frame"}
				]
			},
			"parameters": {
				"duration": 8,
				"resolution": "1080p",
				"ratio": "16:9",
				"generate_audio": true,
				"watermark": false
			},
			"unexpected": "drop me"
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, "video-v1", public.Model)
	require.Equal(t, ByteDanceVideoDefaultModel, public.UpstreamModel)
	require.Equal(t, "a cinematic mountain sunrise", public.Prompt)
	require.Equal(t, int64(8000), public.DurationMilliseconds)
	require.Equal(t, VideoBillingResolution1080P, public.Resolution)
	require.True(t, public.HasImage)
	require.Equal(t, ByteDanceVideoDefaultModel, gjson.GetBytes(body, "model").String())
	require.Equal(t, 2, int(gjson.GetBytes(body, "input.content.#").Int()))
	require.Equal(t, "first_frame", gjson.GetBytes(body, "input.content.1.role").String())
	require.Equal(t, 8, int(gjson.GetBytes(body, "parameters.duration").Int()))
	require.Equal(t, "1080p", gjson.GetBytes(body, "parameters.resolution").String())
	require.Equal(t, "16:9", gjson.GetBytes(body, "parameters.ratio").String())
	require.True(t, gjson.GetBytes(body, "parameters.generate_audio").Bool())
	require.False(t, gjson.GetBytes(body, "unexpected").Exists())
}

func TestPreparePPVideoRequestBodyNormalizesMiniMaxH3ModelVersePayload(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformMiniMaxH3,
		PPVideoOperationGeneric,
		[]byte(`{
			"model": "video-v1",
			"input": {
				"content": [
					{"type":"text","text":"a cinematic mountain sunrise"},
					{"type":"image_url","image_url":{"url":"https://example.com/start.png"},"role":"first_frame"}
				]
			},
			"parameters": {
				"duration": 8,
				"resolution": "2K",
				"ratio": "9:16",
				"aigc_watermark": true
			},
			"unexpected": "drop me"
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, "video-v1", public.Model)
	require.Equal(t, MiniMaxH3VideoDefaultModel, public.UpstreamModel)
	require.Equal(t, "a cinematic mountain sunrise", public.Prompt)
	require.Equal(t, int64(8000), public.DurationMilliseconds)
	require.Equal(t, VideoBillingResolution1080P, public.Resolution)
	require.True(t, public.HasImage)
	require.Equal(t, MiniMaxH3VideoDefaultModel, gjson.GetBytes(body, "model").String())
	require.Equal(t, 2, int(gjson.GetBytes(body, "input.content.#").Int()))
	require.Equal(t, "first_frame", gjson.GetBytes(body, "input.content.1.role").String())
	require.Equal(t, 8, int(gjson.GetBytes(body, "parameters.duration").Int()))
	require.Equal(t, "2K", gjson.GetBytes(body, "parameters.resolution").String())
	require.Equal(t, "adaptive", gjson.GetBytes(body, "parameters.ratio").String())
	require.True(t, gjson.GetBytes(body, "parameters.aigc_watermark").Bool())
	require.False(t, gjson.GetBytes(body, "unexpected").Exists())
}

func TestPreparePPVideoRequestBodyNormalizesMiniMaxHailuo23TextToVideoPayload(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformMiniMaxH3,
		PPVideoOperationGeneric,
		[]byte(`{
			"model": "MiniMax-Hailuo-2.3",
			"input": {
				"prompt": "a cinematic mountain sunrise"
			},
			"parameters": {
				"duration": 10,
				"resolution": "768P",
				"prompt_optimizer": false,
				"fast_pretreatment": true,
				"aigc_watermark": true
			},
			"unexpected": "drop me"
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, MiniMaxHailuo23VideoModel, public.Model)
	require.Equal(t, MiniMaxHailuo23VideoModel, public.UpstreamModel)
	require.Equal(t, "a cinematic mountain sunrise", public.Prompt)
	require.Equal(t, int64(10000), public.DurationMilliseconds)
	require.Equal(t, VideoBillingResolution720P, public.Resolution)
	require.False(t, public.HasImage)
	require.Equal(t, MiniMaxHailuo23VideoModel, gjson.GetBytes(body, "model").String())
	require.Equal(t, "a cinematic mountain sunrise", gjson.GetBytes(body, "input.prompt").String())
	require.False(t, gjson.GetBytes(body, "input.first_frame_image").Exists())
	require.Equal(t, int64(10), gjson.GetBytes(body, "parameters.duration").Int())
	require.Equal(t, "768P", gjson.GetBytes(body, "parameters.resolution").String())
	require.False(t, gjson.GetBytes(body, "parameters.prompt_optimizer").Bool())
	require.True(t, gjson.GetBytes(body, "parameters.fast_pretreatment").Bool())
	require.True(t, gjson.GetBytes(body, "parameters.aigc_watermark").Bool())
	require.False(t, gjson.GetBytes(body, "unexpected").Exists())
}

func TestPreparePPVideoRequestBodyNormalizesMiniMaxHailuo23ImageToVideoPayload(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformMiniMaxH3,
		PPVideoOperationGeneric,
		[]byte(`{
			"model": "MiniMax-Hailuo-2.3",
			"input": {
				"first_frame_image": "data:image/png;base64,ZmFrZQ==",
				"prompt": "a small robot walks through a neon alley"
			},
			"parameters": {
				"duration": 6,
				"resolution": "1080P"
			}
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, int64(6000), public.DurationMilliseconds)
	require.Equal(t, VideoBillingResolution1080P, public.Resolution)
	require.True(t, public.HasImage)
	require.Equal(t, "data:image/png;base64,ZmFrZQ==", gjson.GetBytes(body, "input.first_frame_image").String())
	require.Equal(t, int64(6), gjson.GetBytes(body, "parameters.duration").Int())
	require.Equal(t, "1080P", gjson.GetBytes(body, "parameters.resolution").String())
}

func TestPreparePPVideoRequestBodyRejectsInvalidMiniMaxHailuo23DurationResolution(t *testing.T) {
	t.Parallel()

	_, _, err := PreparePPVideoRequestBody(
		PlatformMiniMaxH3,
		PPVideoOperationGeneric,
		[]byte(`{
			"model": "MiniMax-Hailuo-2.3",
			"input": {"prompt": "a cinematic mountain sunrise"},
			"parameters": {"duration": 10, "resolution": "1080P"}
		}`),
	)

	require.ErrorContains(t, err, "10-second videos only support 768P")
}

func TestPreparePPVideoRequestBodyNormalizesWan30ModelVersePayload(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformWan3,
		PPVideoOperationGeneric,
		[]byte(`{
			"model": "wan3.0-video-prime",
			"input": {
				"prompt": "a cinematic mountain sunrise",
				"media": [
					{"type":"reference_image","url":"HTTPS://example.com/reference.png"}
				]
			},
			"parameters": {
				"duration": 8,
				"resolution": "720p",
				"ratio": "9:16",
				"audio": true,
				"seed": 42,
				"prompt_extend": false,
				"watermark": true
			},
			"unexpected": "drop me"
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, Wan30VideoPrimeModel, public.Model)
	require.Equal(t, Wan30VideoPrimeModel, public.UpstreamModel)
	require.Equal(t, "a cinematic mountain sunrise", public.Prompt)
	require.Equal(t, int64(8000), public.DurationMilliseconds)
	require.Equal(t, VideoBillingResolution720P, public.Resolution)
	require.Equal(t, 1, public.VideoCount)
	require.Equal(t, Wan30VideoPrimeModel, gjson.GetBytes(body, "model").String())
	require.Equal(t, "a cinematic mountain sunrise", gjson.GetBytes(body, "input.prompt").String())
	require.Equal(t, 1, int(gjson.GetBytes(body, "input.media.#").Int()))
	require.Equal(t, "reference_image", gjson.GetBytes(body, "input.media.0.type").String())
	require.Equal(t, "HTTPS://example.com/reference.png", gjson.GetBytes(body, "input.media.0.url").String())
	require.Equal(t, 8, int(gjson.GetBytes(body, "parameters.duration").Int()))
	require.Equal(t, "720P", gjson.GetBytes(body, "parameters.resolution").String())
	require.Equal(t, "9:16", gjson.GetBytes(body, "parameters.ratio").String())
	require.True(t, gjson.GetBytes(body, "parameters.audio").Bool())
	require.Equal(t, int64(42), gjson.GetBytes(body, "parameters.seed").Int())
	require.False(t, gjson.GetBytes(body, "parameters.prompt_extend").Bool())
	require.True(t, gjson.GetBytes(body, "parameters.watermark").Bool())
	require.False(t, gjson.GetBytes(body, "unexpected").Exists())
}

func TestPreparePPVideoRequestBodyNormalizesPixverseV6Payload(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformPixverseV6,
		PPVideoOperationGeneric,
		[]byte(`{
			"model": "video-v1",
			"input": {
				"prompt": "a cinematic mountain sunrise",
				"first_frame_url": "https://example.com/start.png",
				"last_frame_url": "data:image/png;base64,ZmFrZQ=="
			},
			"parameters": {
				"duration": 8,
				"resolution": "540p",
				"aspect_ratio": "9:16",
				"generate_audio": 1,
				"seed": 42
			},
			"unexpected": "drop me"
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, "video-v1", public.Model)
	require.Equal(t, PixverseV6VideoDefaultModel, public.UpstreamModel)
	require.Equal(t, "a cinematic mountain sunrise", public.Prompt)
	require.Equal(t, int64(8000), public.DurationMilliseconds)
	require.Equal(t, VideoBillingResolution720P, public.Resolution)
	require.True(t, public.HasImage)
	require.Equal(t, PixverseV6VideoDefaultModel, gjson.GetBytes(body, "model").String())
	require.Equal(t, "https://example.com/start.png", gjson.GetBytes(body, "input.first_frame_url").String())
	require.Equal(t, "data:image/png;base64,ZmFrZQ==", gjson.GetBytes(body, "input.last_frame_url").String())
	require.Equal(t, 8, int(gjson.GetBytes(body, "parameters.duration").Int()))
	require.Equal(t, "540p", gjson.GetBytes(body, "parameters.resolution").String())
	require.Equal(t, int64(1), gjson.GetBytes(body, "parameters.generate_audio").Int())
	require.Equal(t, int64(42), gjson.GetBytes(body, "parameters.seed").Int())
	require.False(t, gjson.GetBytes(body, "parameters.aspect_ratio").Exists())
	require.False(t, gjson.GetBytes(body, "unexpected").Exists())
}

func TestPreparePPVideoRequestBodyNormalizesPixverseV6TextToVideoPayload(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformPixverseV6,
		PPVideoOperationGeneric,
		[]byte(`{
			"input": {"prompt": "a cinematic mountain sunrise"},
			"parameters": {"duration": 5, "resolution": "360p", "aspect_ratio": "21:9"}
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, VideoBillingResolution480P, public.Resolution)
	require.False(t, public.HasImage)
	require.Equal(t, "21:9", gjson.GetBytes(body, "parameters.aspect_ratio").String())
	require.Equal(t, int64(1), gjson.GetBytes(body, "parameters.generate_audio").Int())
}

func TestPreparePPVideoRequestBodyNormalizesGrokImagineVideoPayload(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformGrokImagineVideo,
		PPVideoOperationGeneric,
		[]byte(`{
			"model": "video-v1",
			"input": {
				"prompt": "a cinematic mountain sunrise",
				"reference_urls": ["https://example.com/reference-1.png", "https://example.com/reference-2.png"]
			},
			"parameters": {
				"duration": 8,
				"resolution": "720p",
				"aspect_ratio": "9:16"
			},
			"unexpected": "drop me"
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, "video-v1", public.Model)
	require.Equal(t, GrokImagineVideoDefaultModel, public.UpstreamModel)
	require.Equal(t, "a cinematic mountain sunrise", public.Prompt)
	require.Equal(t, int64(8000), public.DurationMilliseconds)
	require.Equal(t, VideoBillingResolution720P, public.Resolution)
	require.True(t, public.HasImage)
	require.JSONEq(t, `{
		"model": "grok-imagine-video",
		"input": {
			"prompt": "a cinematic mountain sunrise",
			"reference_urls": ["https://example.com/reference-1.png", "https://example.com/reference-2.png"]
		},
		"parameters": {
			"duration": 8,
			"resolution": "720p",
			"aspect_ratio": "9:16"
		}
	}`, string(body))
}

func TestPreparePPVideoRequestBodyUsesGrokImagineVideoDefaultsAndRejectsMixedReferences(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformGrokImagineVideo,
		PPVideoOperationGeneric,
		[]byte(`{"input":{"prompt":"waves","img_url":"https://example.com/input.png"}}`),
	)
	require.NoError(t, err)
	require.Equal(t, int64(8000), public.DurationMilliseconds)
	require.Equal(t, VideoBillingResolution480P, public.Resolution)
	require.Equal(t, 8, int(gjson.GetBytes(body, "parameters.duration").Int()))
	require.Equal(t, "480p", gjson.GetBytes(body, "parameters.resolution").String())
	require.Equal(t, "16:9", gjson.GetBytes(body, "parameters.aspect_ratio").String())

	_, _, err = PreparePPVideoRequestBody(
		PlatformGrokImagineVideo,
		PPVideoOperationGeneric,
		[]byte(`{"input":{"prompt":"waves","img_url":"https://example.com/input.png","reference_urls":["https://example.com/reference.png"]}}`),
	)
	require.ErrorContains(t, err, "mutually exclusive")
}

func TestPreparePPVideoRequestBodyRejectsGrokImagineVideoTextOnlyPayload(t *testing.T) {
	t.Parallel()

	_, _, err := PreparePPVideoRequestBody(
		PlatformGrokImagineVideo,
		PPVideoOperationGeneric,
		[]byte(`{"model":"grok-imagine-video","input":{"prompt":"a sunrise over the ocean"},"parameters":{"duration":5,"resolution":"720p","aspect_ratio":"16:9"}}`),
	)

	require.ErrorContains(t, err, "text-to-video is not supported")
}

func TestPreparePPVideoRequestBodyRejectsInvalidMiniMaxH3MediaModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "two text items",
			body: `{"input":{"content":[{"type":"text","text":"one"},{"type":"text","text":"two"}]},"parameters":{"duration":5,"resolution":"768P","ratio":"16:9"}}`,
			want: "exactly one",
		},
		{
			name: "frame and reference image",
			body: `{"input":{"content":[{"type":"text","text":"scene"},{"type":"image_url","image_url":{"url":"https://example.com/start.png"},"role":"first_frame"},{"type":"image_url","image_url":{"url":"https://example.com/ref.png"},"role":"reference_image"}]},"parameters":{"duration":5,"resolution":"768P"}}`,
			want: "cannot combine",
		},
		{
			name: "audio without visual reference",
			body: `{"input":{"content":[{"type":"text","text":"scene"},{"type":"audio_url","audio_url":{"url":"https://example.com/audio.mp3"},"role":"reference_audio"}]},"parameters":{"duration":5,"resolution":"768P"}}`,
			want: "requires a reference image or reference video",
		},
		{
			name: "non public media URL",
			body: `{"input":{"content":[{"type":"text","text":"scene"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"},"role":"first_frame"}]},"parameters":{"duration":5,"resolution":"768P"}}`,
			want: "public http or https URL",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := PreparePPVideoRequestBody(PlatformMiniMaxH3, PPVideoOperationGeneric, []byte(tt.body))
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestPreparePPVideoRequestBodyRejectsInvalidWan30MediaModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "duration below supported range",
			body: `{"input":{"prompt":"scene"},"parameters":{"duration":1}}`,
			want: "from 2 to 30",
		},
		{
			name: "frame and reference image",
			body: `{"input":{"prompt":"scene","media":[{"type":"first_frame","url":"https://example.com/start.png"},{"type":"reference_image","url":"https://example.com/ref.png"}]},"parameters":{"duration":5}}`,
			want: "cannot combine",
		},
		{
			name: "file and link together",
			body: `{"input":{"prompt":"scene","media":[{"type":"file","url":"https://example.com/source.mp4"},{"type":"link","url":"https://example.com/source"}]},"parameters":{"duration":5}}`,
			want: "not both",
		},
		{
			name: "invalid audio type",
			body: `{"input":{"prompt":"scene"},"parameters":{"duration":5,"audio":"true"}}`,
			want: "audio must be boolean",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := PreparePPVideoRequestBody(PlatformWan3, PPVideoOperationGeneric, []byte(tt.body))
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestPreparePPVideoRequestBodyRejectsInvalidPixverseV6Payload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "duration over supported range",
			body: `{"input":{"prompt":"scene"},"parameters":{"duration":16}}`,
			want: "from 1 to 15",
		},
		{
			name: "first frame without last frame",
			body: `{"input":{"prompt":"scene","first_frame_url":"https://example.com/start.png"},"parameters":{"duration":5}}`,
			want: "must be provided together",
		},
		{
			name: "unsupported resolution",
			body: `{"input":{"prompt":"scene"},"parameters":{"duration":5,"resolution":"4k"}}`,
			want: "resolution must be 360p, 540p, 720p, or 1080p",
		},
		{
			name: "unsupported text aspect ratio",
			body: `{"input":{"prompt":"scene"},"parameters":{"duration":5,"aspect_ratio":"adaptive"}}`,
			want: "aspect_ratio is not supported",
		},
		{
			name: "seed exceeds range",
			body: `{"input":{"prompt":"scene"},"parameters":{"duration":5,"seed":2147483648}}`,
			want: "seed must be an integer from 0 to 2147483647",
		},
		{
			name: "video requires public URL",
			body: `{"input":{"prompt":"scene","video_url":"file:///tmp/source.mp4"},"parameters":{"duration":5}}`,
			want: "video_url must be an HTTP(S) URL",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := PreparePPVideoRequestBody(PlatformPixverseV6, PPVideoOperationGeneric, []byte(tt.body))
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestPreparePPVideoRequestBodyLimitsSeedancePrompt(t *testing.T) {
	t.Parallel()

	prompt := strings.Repeat("画", ppVideoSeedancePromptMaxRunes)
	body, public, err := PreparePPVideoRequestBody(
		PlatformSeedance,
		PPVideoOperationGeneric,
		[]byte(fmt.Sprintf(`{"prompt":%q}`, prompt)),
	)

	require.NoError(t, err)
	require.Equal(t, prompt, public.Prompt)
	require.Equal(t, ppVideoSeedancePromptMaxRunes, len([]rune(gjson.GetBytes(body, "prompt").String())))

	overlong := strings.Repeat("画", ppVideoSeedancePromptMaxRunes+1)
	_, _, err = PreparePPVideoRequestBody(
		PlatformSeedance,
		PPVideoOperationGeneric,
		[]byte(fmt.Sprintf(`{"prompt":%q}`, overlong)),
	)

	require.ErrorContains(t, err, "Seedance prompt must be at most 2000 characters")
}

func TestPreparePPVideoRequestBodyNormalizesKlingUnifiedImageRequest(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformKling,
		PPVideoOperationKlingImageToVideo,
		[]byte(`{
			"model": "kling-v3",
			"prompt": "animate the scene",
			"image_url": "https://example.com/reference.png",
			"images": ["https://example.com/fallback.png"],
			"start_frame_url": "https://example.com/start.png",
			"end_frame_url": "https://example.com/end.png",
			"duration": 5,
			"resolution": "1080p",
			"generate_audio": true,
			"n": 2,
			"video_count": 2,
			"output_width": 1920,
			"output_height": 1080,
			"fps": 30
		}`),
	)

	require.NoError(t, err)
	require.True(t, public.HasImage)
	require.Equal(t, "kling-v3", gjson.GetBytes(body, "model_name").String())
	require.Equal(t, "https://example.com/start.png", gjson.GetBytes(body, "image").String())
	require.Equal(t, "https://example.com/end.png", gjson.GetBytes(body, "image_tail").String())
	require.Equal(t, "pro", gjson.GetBytes(body, "mode").String())
	require.Equal(t, "on", gjson.GetBytes(body, "sound").String())
	require.Equal(t, "5", gjson.GetBytes(body, "duration").String())
	requireNoKlingPublicFieldsLeaked(t, body)
}

func TestPreparePPVideoRequestBodyMapsKlingImageURLToImage(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformKling,
		PPVideoOperationKlingImageToVideo,
		[]byte(`{"prompt":"animate","image_url":"https://example.com/reference.png","duration":5}`),
	)

	require.NoError(t, err)
	require.True(t, public.HasImage)
	require.Equal(t, "https://example.com/reference.png", gjson.GetBytes(body, "image").String())
	require.False(t, gjson.GetBytes(body, "image_url").Exists())
}

func TestPreparePPVideoRequestBodyNormalizesKlingUnifiedTextRequest(t *testing.T) {
	t.Parallel()

	body, _, err := PreparePPVideoRequestBody(
		PlatformKling,
		PPVideoOperationKlingTextToVideo,
		[]byte(`{
			"model": "video-v1",
			"prompt": "waves",
			"duration": 6,
			"resolution": "4k",
			"mode": "2x-pro",
			"generate_audio": false
		}`),
	)

	require.NoError(t, err)
	require.Equal(t, "kling-v3", gjson.GetBytes(body, "model_name").String())
	require.Equal(t, "pro", gjson.GetBytes(body, "mode").String())
	require.Equal(t, "off", gjson.GetBytes(body, "sound").String())
	require.False(t, gjson.GetBytes(body, "image").Exists())
	require.False(t, gjson.GetBytes(body, "image_tail").Exists())
	requireNoKlingPublicFieldsLeaked(t, body)
}

func TestPreparePPVideoRequestBodyAllowsKlingPromptAtLimit(t *testing.T) {
	t.Parallel()

	prompt := strings.Repeat("画", ppVideoKlingPromptMaxRunes)
	body, public, err := PreparePPVideoRequestBody(
		PlatformKling,
		PPVideoOperationKlingTextToVideo,
		[]byte(fmt.Sprintf(`{"prompt":%q}`, prompt)),
	)

	require.NoError(t, err)
	require.Equal(t, prompt, public.Prompt)
	require.Equal(t, ppVideoKlingPromptMaxRunes, len([]rune(gjson.GetBytes(body, "prompt").String())))
}

func TestPreparePPVideoRequestBodyRejectsOverlongKlingPromptFields(t *testing.T) {
	t.Parallel()

	overlong := strings.Repeat("画", ppVideoKlingPromptMaxRunes+1)
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "top level prompt",
			body: fmt.Sprintf(`{"prompt":%q}`, overlong),
			want: "Kling prompt must be at most 2500 characters",
		},
		{
			name: "nested prompt",
			body: fmt.Sprintf(`{"data":{"prompt":%q}}`, overlong),
			want: "Kling prompt must be at most 2500 characters",
		},
		{
			name: "negative prompt",
			body: fmt.Sprintf(`{"prompt":"waves","negative_prompt":%q}`, overlong),
			want: "Kling negative_prompt must be at most 2500 characters",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := PreparePPVideoRequestBody(PlatformKling, PPVideoOperationKlingTextToVideo, []byte(tt.body))

			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestPreparePPVideoRequestBodyRejectsUnsupportedKlingDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "decimal", body: `{"prompt":"waves","duration":5.5}`},
		{name: "too short", body: `{"prompt":"waves","duration":2}`},
		{name: "too long", body: `{"prompt":"waves","duration":16}`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := PreparePPVideoRequestBody(PlatformKling, PPVideoOperationKlingTextToVideo, []byte(tt.body))

			require.ErrorContains(t, err, "Kling duration must be an integer number of seconds from 3 to 15")
		})
	}
}

func requireNoKlingPublicFieldsLeaked(t *testing.T, body []byte) {
	t.Helper()

	for _, field := range []string{
		"model",
		"image_url",
		"images",
		"start_frame_url",
		"end_frame_url",
		"resolution",
		"generate_audio",
		"with_audio",
		"audio",
		"enable_audio",
		"n",
		"video_count",
		"count",
		"output_width",
		"output_height",
		"width",
		"height",
		"fps",
		"frame_rate",
	} {
		require.False(t, gjson.GetBytes(body, field).Exists(), "field %q leaked in %s", field, string(body))
	}
}

func TestPreparePPVideoRequestBodyRejectsJimengModelOnPPSeedance(t *testing.T) {
	t.Parallel()

	_, _, err := PreparePPVideoRequestBody(
		PlatformSeedance,
		PPVideoOperationGeneric,
		[]byte(`{"model":"by-seedance2.0-933","prompt":"waves"}`),
	)

	require.EqualError(t, err, `model "by-seedance2.0-933" is not supported by platform "seedance"`)
}

func TestPreparePPVideoRequestBodyForAccountAppliesModelMapping(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformSeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"seedance-default": "doubao-seedance-2-0-260128",
			},
		},
	}

	body, public, err := PreparePPVideoRequestBodyForAccount(
		account,
		PPVideoOperationGeneric,
		[]byte(`{"prompt":"waves"}`),
	)

	require.NoError(t, err)
	require.Empty(t, public.Model)
	require.Equal(t, "doubao-seedance-2-0-260128", public.UpstreamModel)
	require.Equal(t, "doubao-seedance-2-0-260128", gjson.GetBytes(body, "model").String())
}

func TestPreparePPVideoRequestBodyForAccountNormalizesLegacySeedanceAliasMapping(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformSeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				SiteVideoDefaultModel: SiteVideoDefaultModel,
			},
		},
	}

	body, public, err := PreparePPVideoRequestBodyForAccount(
		account,
		PPVideoOperationGeneric,
		[]byte(`{"prompt":"waves"}`),
	)

	require.NoError(t, err)
	require.Empty(t, public.Model)
	require.Equal(t, "doubao-seedance-2-0-260128", public.UpstreamModel)
	require.Equal(t, "doubao-seedance-2-0-260128", gjson.GetBytes(body, "model").String())
}

func TestPreparePPVideoRequestBodyNormalizesSeedanceSingleImageAlias(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformSeedance,
		PPVideoOperationGeneric,
		[]byte(`{
			"prompt": "保持参考素材中的人物自然向前走",
			"image": "https://example.com/compat-image.jpg",
			"videos": ["https://example.com/reference-video.mp4"],
			"audios": ["https://example.com/reference-audio.mp3"],
			"aspect_ratio": "9:16",
			"generate_audio": true,
			"seed": 12345
		}`),
	)

	require.NoError(t, err)
	require.Empty(t, public.Model)
	require.True(t, public.HasImage)
	require.Equal(t, "doubao-seedance-2-0-260128", gjson.GetBytes(body, "model").String())
	require.Equal(t, "https://example.com/compat-image.jpg", gjson.GetBytes(body, "images.0").String())
	require.False(t, gjson.GetBytes(body, "image").Exists())
	require.Equal(t, "https://example.com/reference-video.mp4", gjson.GetBytes(body, "videos.0").String())
	require.Equal(t, "https://example.com/reference-audio.mp3", gjson.GetBytes(body, "audios.0").String())
	require.Equal(t, "9:16", gjson.GetBytes(body, "aspect_ratio").String())
	require.True(t, gjson.GetBytes(body, "generate_audio").Bool())
	require.Equal(t, int64(12345), gjson.GetBytes(body, "seed").Int())
}

func TestPreparePPVideoRequestBodyRejectsAmbiguousSeedanceMediaInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "multiple reference image fields",
			body: `{
				"images":["https://example.com/reference.png"],
				"image":"https://example.com/compat.png"
			}`,
			want: "Seedance accepts only one reference image field",
		},
		{
			name: "reference image and first frame",
			body: `{
				"image_url":"https://example.com/reference.png",
				"start_frame_url":"https://example.com/start.png"
			}`,
			want: "Seedance cannot combine reference images with start_frame_url or end_frame_url",
		},
		{
			name: "reference image and last frame",
			body: `{
				"images":["https://example.com/reference.png"],
				"end_frame_url":"https://example.com/end.png"
			}`,
			want: "Seedance cannot combine reference images with start_frame_url or end_frame_url",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := PreparePPVideoRequestBody(
				PlatformSeedance,
				PPVideoOperationGeneric,
				[]byte(tt.body),
			)

			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestPreparePPVideoRequestBodyAllowsSeedanceFrameControlMode(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformSeedance,
		PPVideoOperationGeneric,
		[]byte(`{
			"prompt":"镜头从首帧平稳过渡到尾帧",
			"start_frame_url":"https://example.com/start.png",
			"end_frame_url":"https://example.com/end.png"
		}`),
	)

	require.NoError(t, err)
	require.False(t, public.HasImage)
	require.Equal(t, "https://example.com/start.png", gjson.GetBytes(body, "start_frame_url").String())
	require.Equal(t, "https://example.com/end.png", gjson.GetBytes(body, "end_frame_url").String())
}

func TestPreparePPVideoRequestBodyTreatsLegacyVideoV1AsDefaultAlias(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformKling,
		PPVideoOperationKlingTextToVideo,
		[]byte(`{"model":"video-v1","prompt":"waves"}`),
	)

	require.NoError(t, err)
	require.Equal(t, "video-v1", public.Model)
	require.Equal(t, "kling-v3", gjson.GetBytes(body, "model_name").String())
	require.False(t, gjson.GetBytes(body, "model").Exists())
}

func TestPreparePPVideoRequestBodyPassesUpdatedSeedanceModelNames(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformSeedance,
		PPVideoOperationGeneric,
		[]byte(`{"model":"seedance2.0-431","prompt":"waves","duration":10}`),
	)

	require.NoError(t, err)
	require.Equal(t, "seedance2.0-431", public.Model)
	require.Equal(t, "seedance2.0-431", public.UpstreamModel)
	require.Equal(t, "seedance2.0-431", gjson.GetBytes(body, "model").String())
	require.Equal(t, "10", gjson.GetBytes(body, "duration").String())
}

func TestPreparePPVideoRequestBodyRejectsForeignPPVideoModelFamilies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		platform  string
		operation PPVideoOperation
		model     string
	}{
		{
			name:      "Kling rejects Seedance model",
			platform:  PlatformKling,
			operation: PPVideoOperationKlingTextToVideo,
			model:     "doubao-seedance-2-0-mini-260615",
		},
		{
			name:      "Kling rejects Seedance alias without mapping",
			platform:  PlatformKling,
			operation: PPVideoOperationKlingTextToVideo,
			model:     "seedance2.0-900",
		},
		{
			name:      "Kling rejects Jimeng model",
			platform:  PlatformKling,
			operation: PPVideoOperationKlingTextToVideo,
			model:     "by-seedance2.0-933",
		},
		{
			name:      "Happy Horse rejects Kling model",
			platform:  PlatformHappyHourse,
			operation: PPVideoOperationGeneric,
			model:     "kling-v3",
		},
		{
			name:      "Seedance rejects Kling model",
			platform:  PlatformSeedance,
			operation: PPVideoOperationGeneric,
			model:     "kling-v3",
		},
		{
			name:      "Seedance rejects Happy Horse model",
			platform:  PlatformSeedance,
			operation: PPVideoOperationGeneric,
			model:     "happyhorse-1.0-t2v",
		},
		{
			name:      "Wan3.0 rejects MiniMax-H3 model",
			platform:  PlatformWan3,
			operation: PPVideoOperationGeneric,
			model:     MiniMaxH3VideoDefaultModel,
		},
		{
			name:      "Pixverse v6 rejects MiniMax-H3 model",
			platform:  PlatformPixverseV6,
			operation: PPVideoOperationGeneric,
			model:     MiniMaxH3VideoDefaultModel,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := PreparePPVideoRequestBody(
				tt.platform,
				tt.operation,
				[]byte(fmt.Sprintf(`{"model":%q,"prompt":"waves"}`, tt.model)),
			)

			require.EqualError(t, err, fmt.Sprintf(
				`model %q is not supported by platform %q`,
				tt.model,
				tt.platform,
			))
		})
	}
}

func TestPreparePPVideoRequestBodyAllowsCustomPPVideoModelAliases(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformKling,
		PPVideoOperationKlingTextToVideo,
		[]byte(`{"model":"customer-kling-default","prompt":"waves"}`),
	)

	require.NoError(t, err)
	require.Equal(t, "customer-kling-default", public.Model)
	require.Equal(t, "customer-kling-default", public.UpstreamModel)
	require.Equal(t, "customer-kling-default", gjson.GetBytes(body, "model_name").String())
}

func TestFilterPPVideoPublicModelsForPlatformRemovesForeignVideoModels(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		[]string{"kling-v3"},
		filterPPVideoPublicModelsForPlatform(PlatformKling, []string{
			"kling-v3",
			"happyhorse-1.0-t2v",
			"doubao-seedance-2-0-260128",
			"by-seedance2.0-933",
		}),
	)
	require.Equal(t,
		[]string{"happyhorse-1.0-t2v"},
		filterPPVideoPublicModelsForPlatform(PlatformHappyHourse, []string{
			"kling-v3",
			"happyhorse-1.0-t2v",
			"seedance2.0-431",
			"by-seedance2.0-933",
		}),
	)
	require.Equal(t,
		[]string{"video-v1", "doubao-seedance-2-0-260128", "seedance2.0-431"},
		filterPPVideoPublicModelsForPlatform(PlatformSeedance, []string{
			"video-v1",
			"by-seedance2.0-933",
			"doubao-seedance-2-0-260128",
			"seedance2.0-431",
			"kling-v3",
		}),
	)
}

func TestPreparePPVideoRequestBodyChoosesHappyHorseImageModel(t *testing.T) {
	t.Parallel()

	body, public, err := PreparePPVideoRequestBody(
		PlatformHappyHourse,
		PPVideoOperationGeneric,
		[]byte(`{"model":"video-v1","prompt":"animate","image_url":"https://example.com/in.png","duration":8,"resolution":"1080p"}`),
	)

	require.NoError(t, err)
	require.True(t, public.HasImage)
	require.Equal(t, "happyhorse-1.0-i2v", gjson.GetBytes(body, "model").String())
	require.Equal(t, "8", gjson.GetBytes(body, "duration").String())
	require.Equal(t, VideoBillingResolution1080P, gjson.GetBytes(body, "resolution").String())
}

func TestPPVideoBillingMetadataUsesPreparedDefaults(t *testing.T) {
	t.Parallel()

	body, _, err := PreparePPVideoRequestBody(PlatformSeedance, PPVideoOperationGeneric, []byte(`{"model":"video-v1","prompt":"waves"}`))
	require.NoError(t, err)

	metadata := PPVideoBillingMetadataFromRequest(PlatformSeedance, body)

	require.NoError(t, metadata.ValidationError)
	require.Equal(t, int64(5000), metadata.RequestedDurationMilliseconds)
	require.Equal(t, 5, metadata.RequestedDurationSeconds)
	require.Equal(t, VideoBillingResolution720P, metadata.VideoResolution)
}

func TestNormalizePPVideoPublicResponseAddsStableFields(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"data": {
			"task_id": "provider-task-1",
			"status": "SUCCESS",
			"task_result": {
				"videos": [{"url": "https://cdn.example.com/video.mp4", "duration": "5.041"}]
			}
		}
	}`)
	parsed, err := ParsePPVideoResponse(PlatformSeedance, raw)
	require.NoError(t, err)
	public := PPVideoPublicRequest{
		UpstreamModel:        "doubao-seedance-2-0-260128",
		DurationMilliseconds: 5000,
		VideoCount:           1,
		Resolution:           VideoBillingResolution720P,
	}

	body := NormalizePPVideoPublicResponse(PlatformSeedance, raw, public, parsed)

	require.JSONEq(t, `{
		"data": {
			"task_id": "provider-task-1",
			"status": "SUCCESS",
			"video_url": "https://cdn.example.com/video.mp4",
			"url": "https://cdn.example.com/video.mp4",
			"result_url": "https://cdn.example.com/video.mp4",
			"videos": [{
				"url": "https://cdn.example.com/video.mp4",
				"video_url": "https://cdn.example.com/video.mp4"
			}],
			"task_result": {
				"videos": [{"url": "https://cdn.example.com/video.mp4", "duration": "5.041"}]
			}
		},
		"id": "provider-task-1",
		"task_id": "provider-task-1",
		"object": "video.generation.task",
		"status": "succeeded",
		"model": "doubao-seedance-2-0-260128",
		"video_url": "https://cdn.example.com/video.mp4",
		"url": "https://cdn.example.com/video.mp4",
		"result_url": "https://cdn.example.com/video.mp4",
		"videos": [{
			"url": "https://cdn.example.com/video.mp4",
			"video_url": "https://cdn.example.com/video.mp4"
		}],
		"usage": {
			"duration_ms": 5041,
			"duration_seconds": 5.041,
			"video_count": 1,
			"resolution": "720p"
		}
	}`, string(body))
}

func TestNormalizePPVideoPublicResponsePreservesSignedVideoURL(t *testing.T) {
	t.Parallel()

	signedURL := "https://dashscope-a717.oss-accelerate.aliyuncs.com/dashscope-a717/video.mp4?Expires=1788683524&OSSAccessKeyId=test-key&Signature=2%2FLgM4d3%2BoIHPIiYm73A1kAFNe0%3D"
	raw := []byte(`{
		"output": {
			"video_url": "` + signedURL + `"
		},
		"status": "SUCCEEDED",
		"task_id": "wan-task-1"
	}`)
	parsed, err := ParsePPVideoResponse(PlatformWan3, raw)
	require.NoError(t, err)

	body := NormalizePPVideoPublicResponse(PlatformWan3, raw, PPVideoPublicRequest{
		Model:                Wan30VideoDefaultModel,
		DurationMilliseconds: 5000,
		VideoCount:           1,
		Resolution:           VideoBillingResolution720P,
	}, parsed)

	require.Equal(t, signedURL, gjson.GetBytes(body, "video_url").String())
	require.Equal(t, signedURL, gjson.GetBytes(body, "data.video_url").String())
	require.Equal(t, signedURL, gjson.GetBytes(body, "videos.0.url").String())
	require.NotContains(t, gjson.GetBytes(body, "video_url").String(), " ")
}

func TestNormalizePPVideoPublicResponseExtractsProviderVideoURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		platform   string
		raw        string
		wantTaskID string
		wantURL    string
	}{
		{
			name:     "Kling result url",
			platform: PlatformKling,
			raw: `{
				"data": {
					"task_id": "kling-task-1",
					"status": "SUCCESS",
					"result_url": "https://cdn.example.com/kling.mp4"
				}
			}`,
			wantTaskID: "kling-task-1",
			wantURL:    "https://cdn.example.com/kling.mp4",
		},
		{
			name:     "Seedance nested content video url",
			platform: PlatformSeedance,
			raw: `{
				"data": {
					"task_id": "seedance-task-1",
					"status": "SUCCESS",
					"data": {
						"content": {
							"video_url": "https://cdn.example.com/seed.mp4"
						}
					}
				}
			}`,
			wantTaskID: "seedance-task-1",
			wantURL:    "https://cdn.example.com/seed.mp4",
		},
		{
			name:     "Kling nested result video url",
			platform: PlatformKling,
			raw: `{
				"data": {
					"task_id": "kling-task-2",
					"task_status": "succeed",
					"result": {
						"video_url": "https://cdn.example.com/kling-result.mp4"
					}
				}
			}`,
			wantTaskID: "kling-task-2",
			wantURL:    "https://cdn.example.com/kling-result.mp4",
		},
		{
			name:     "Kling output video urls",
			platform: PlatformKling,
			raw: `{
				"data": {
					"task_id": "kling-task-3",
					"task_status": "succeed",
					"output": {
						"video_urls": ["https://cdn.example.com/kling-output.mp4"]
					}
				}
			}`,
			wantTaskID: "kling-task-3",
			wantURL:    "https://cdn.example.com/kling-output.mp4",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := []byte(tt.raw)
			parsed, err := ParsePPVideoResponse(tt.platform, raw)
			require.NoError(t, err)
			require.Equal(t, tt.wantTaskID, parsed.TaskID)
			require.Equal(t, PPVideoTaskStatusSucceeded, parsed.Status)

			body := NormalizePPVideoPublicResponse(tt.platform, raw, PPVideoPublicRequest{
				UpstreamModel:        "doubao-seedance-2-0-260128",
				DurationMilliseconds: 5000,
				VideoCount:           1,
				Resolution:           VideoBillingResolution720P,
			}, parsed)

			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "video_url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "data.video_url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "result_url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "videos.0.url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "videos.0.video_url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "data.url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "data.result_url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "data.videos.0.url").String())
			require.Equal(t, tt.wantURL, gjson.GetBytes(body, "data.videos.0.video_url").String())
			require.Equal(t, tt.wantTaskID, gjson.GetBytes(body, "id").String())
			require.Equal(t, tt.wantTaskID, gjson.GetBytes(body, "task_id").String())
		})
	}
}

func TestNormalizePPVideoPublicResponsePreservesExplicitPlatformModel(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"task_id":"provider-task-2","status":"processing"}`)
	parsed, err := ParsePPVideoResponse(PlatformSeedance, raw)
	require.NoError(t, err)

	body := NormalizePPVideoPublicResponse(PlatformSeedance, raw, PPVideoPublicRequest{
		Model:         "seedance2.0-431",
		UpstreamModel: "seedance2.0-431",
	}, parsed)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, "seedance2.0-431", payload["model"])
	require.NotEqual(t, SiteVideoDefaultModel, payload["model"])
}
