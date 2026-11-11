package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func videoResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Header:     http.Header{"Content-Type": []string{"video/mp4"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestForwardPPVideoTaskUserContent_StreamsStoredDirectURL(t *testing.T) {
	const videoURL = "https://cdn.example.com/result.mp4?sig=abc"
	upstream := &oponeUpstreamRecorder{responses: []*http.Response{videoResponse(http.StatusPartialContent, "video-bytes")}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{ID: 7, Platform: PlatformGuangying, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "guangying-secret"}}
	task := &PPVideoTask{
		TaskID: "task-1", Platform: PlatformGuangying, AccountID: 7, Status: PPVideoTaskStatusSucceeded,
		ResponseBody: `{"status":"succeeded","video_url":"` + videoURL + `"}`,
	}

	resp, err := svc.ForwardPPVideoTaskUserContent(context.Background(), account, task, "bytes=0-", `"etag"`)

	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "video-bytes", string(data))
	require.Equal(t, http.StatusPartialContent, resp.StatusCode)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, videoURL, upstream.requests[0].URL.String())
	require.Empty(t, upstream.requests[0].Header.Get("Authorization"), "account credentials must never reach the storage host")
	require.Equal(t, "bytes=0-", upstream.requests[0].Header.Get("Range"))
	require.Equal(t, `"etag"`, upstream.requests[0].Header.Get("If-Range"))
}

func TestForwardPPVideoTaskUserContent_FindsNestedURL(t *testing.T) {
	upstream := &oponeUpstreamRecorder{responses: []*http.Response{videoResponse(http.StatusOK, "ok")}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{ID: 7, Platform: PlatformKling, Type: AccountTypeAPIKey}
	task := &PPVideoTask{
		TaskID: "task-2", Platform: PlatformKling, AccountID: 7, Status: PPVideoTaskStatusSucceeded,
		ResponseBody: `{"data":{"task_result":{"videos":[{"url":"https://cdn.example.com/k.mp4"}]}}}`,
	}

	resp, err := svc.ForwardPPVideoTaskUserContent(context.Background(), account, task, "", "")

	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, "https://cdn.example.com/k.mp4", upstream.requests[0].URL.String())
}

func TestForwardPPVideoTaskUserContent_RejectsTasksWithoutUsableURL(t *testing.T) {
	account := &Account{ID: 7, Platform: PlatformGuangying, Type: AccountTypeAPIKey}
	tests := []struct {
		name string
		body string
	}{
		{"no url in the stored response", `{"status":"succeeded"}`},
		{"plain http url is not fetched", `{"video_url":"http://cdn.example.com/v.mp4"}`},
		{"private address is blocked", `{"video_url":"https://10.0.0.5/v.mp4"}`},
		{"empty stored response", ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &oponeUpstreamRecorder{}
			svc := &OpenAIGatewayService{httpUpstream: upstream}
			task := &PPVideoTask{TaskID: "task-3", Platform: PlatformGuangying, AccountID: 7, ResponseBody: tt.body}

			resp, err := svc.ForwardPPVideoTaskUserContent(context.Background(), account, task, "", "")

			require.Error(t, err)
			require.Nil(t, resp)
			require.Empty(t, upstream.requests)
		})
	}
}

func TestForwardPPVideoTaskUserContent_RequiresTheOriginalAccount(t *testing.T) {
	upstream := &oponeUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	task := &PPVideoTask{
		TaskID: "task-4", Platform: PlatformGuangying, AccountID: 7,
		ResponseBody: `{"video_url":"https://cdn.example.com/v.mp4"}`,
	}

	_, err := svc.ForwardPPVideoTaskUserContent(context.Background(),
		&Account{ID: 8, Platform: PlatformGuangying}, task, "", "")
	require.Error(t, err)

	_, err = svc.ForwardPPVideoTaskUserContent(context.Background(),
		&Account{ID: 7, Platform: PlatformKling}, task, "", "")
	require.Error(t, err)

	_, err = svc.ForwardPPVideoTaskUserContent(context.Background(), nil, task, "", "")
	require.Error(t, err)

	_, err = svc.ForwardPPVideoTaskUserContent(context.Background(), &Account{ID: 7, Platform: PlatformGuangying}, nil, "", "")
	require.Error(t, err)

	require.Empty(t, upstream.requests)
}

func TestForwardPPVideoTaskUserContent_AuthenticatedPlatformsKeepUsingTheContentProxy(t *testing.T) {
	upstream := &oponeUpstreamRecorder{responses: []*http.Response{videoResponse(http.StatusOK, "proxied")}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID: 9, Platform: PlatformCTMOSeedance, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "ctmo-key"},
	}
	task := &PPVideoTask{TaskID: "task-5", Platform: PlatformCTMOSeedance, AccountID: 9, Status: PPVideoTaskStatusSucceeded}

	resp, err := svc.ForwardPPVideoTaskUserContent(context.Background(), account, task, "", "")

	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "Bearer ctmo-key", upstream.requests[0].Header.Get("Authorization"))
}

func TestPPVideoTaskUsesAuthenticatedContent(t *testing.T) {
	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformCTMOSeedance))
	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformXingAPISeedance))
	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformJijiuSeedance))
	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformMaiTokenSeedance))
	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformOPONE))
	require.False(t, ppVideoTaskUsesAuthenticatedContent(PlatformGuangying))
	require.False(t, ppVideoTaskUsesAuthenticatedContent(PlatformKling))
}

func TestForwardMaiTokenSeedanceTaskContentUsesOriginalAccountAndHEAD(t *testing.T) {
	upstream := &oponeUpstreamRecorder{responses: []*http.Response{videoResponse(http.StatusPartialContent, "")}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID: 19, Platform: PlatformMaiTokenSeedance, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "mai-key"},
	}
	task := &PPVideoTask{TaskID: "task/one", Platform: PlatformMaiTokenSeedance, AccountID: 19, Status: PPVideoTaskStatusSucceeded, BillingStatus: PPVideoBillingStatusCaptured}

	resp, err := svc.ForwardPPVideoTaskContentWithMethod(context.Background(), http.MethodHead, account, task, "bytes=0-99", `"etag"`)

	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Len(t, upstream.requests, 1)
	require.Equal(t, http.MethodHead, upstream.requests[0].Method)
	require.Equal(t, "https://api.mai-token.com/v1/videos/task%2Fone/content", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer mai-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-99", upstream.requests[0].Header.Get("Range"))
	require.Equal(t, `"etag"`, upstream.requests[0].Header.Get("If-Range"))
}

func TestForwardMaiTokenSeedanceTaskContentRequiresCapturedBilling(t *testing.T) {
	t.Parallel()

	for _, billingStatus := range []string{PPVideoBillingStatusHeld, PPVideoBillingStatusSettling} {
		t.Run(billingStatus, func(t *testing.T) {
			upstream := &oponeUpstreamRecorder{}
			svc := &OpenAIGatewayService{httpUpstream: upstream}
			account := &Account{ID: 19, Platform: PlatformMaiTokenSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "mai-key"}}
			task := &PPVideoTask{
				TaskID: "task/one", Platform: PlatformMaiTokenSeedance, AccountID: 19,
				Status: PPVideoTaskStatusSucceeded, BillingStatus: billingStatus,
			}

			resp, err := svc.ForwardPPVideoTaskUserContent(context.Background(), account, task, "", "")

			require.Error(t, err)
			require.Nil(t, resp)
			require.Empty(t, upstream.requests)
		})
	}
}
