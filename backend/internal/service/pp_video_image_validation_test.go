package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type miniMaxImageRoundTripFunc func(*http.Request) (*http.Response, error)

func (f miniMaxImageRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func miniMaxImageTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, width, height))))
	return buf.Bytes()
}

func miniMaxImageTestBody(t *testing.T, urls ...string) []byte {
	t.Helper()
	content := []any{map[string]any{"type": "text", "text": "Use <Picture 1> and <Picture 2>."}}
	for _, url := range urls {
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}, "role": "reference_image"})
	}
	body, err := json.Marshal(map[string]any{"model": "MiniMax-H3", "input": map[string]any{"content": content}, "parameters": map[string]any{"duration": 11, "resolution": "768P", "ratio": "9:16"}})
	require.NoError(t, err)
	return body
}

func TestMiniMaxH3ImageDimensionsBeforeSubmit(t *testing.T) {
	for _, tt := range []struct {
		width, height int
		valid         bool
	}{
		{384, 216, false}, {216, 384, false},
		{255, 256, false}, {256, 255, false},
		{256, 256, true}, {512, 288, true}, {2048, 1152, true},
		{5760, 256, true}, {256, 5760, true},
		{5761, 256, false}, {256, 5761, false},
	} {
		t.Run(fmt.Sprintf("%dx%d", tt.width, tt.height), func(t *testing.T) {
			first := miniMaxImageTestPNG(t, 512, 288)
			second := miniMaxImageTestPNG(t, tt.width, tt.height)
			calls := 0
			client := &http.Client{Transport: miniMaxImageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "bytes=0-1048575", req.Header.Get("Range"))
				require.Empty(t, req.Header.Get("Authorization"))
				require.Empty(t, req.Header.Get("Cookie"))
				require.Equal(t, "credential=test%2F20260929&signature=example", req.URL.RawQuery)
				data := first
				if calls == 2 {
					data = second
				}
				return &http.Response{StatusCode: http.StatusPartialContent, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}
			body := miniMaxImageTestBody(t, "https://example.com/first.png?credential=test%2F20260929&signature=example", "https://example.com/second.png?credential=test%2F20260929&signature=example")
			err := validateMiniMaxH3ImageDimensions(context.Background(), body, client)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "input.content[2].image_url")
				require.ErrorContains(t, err, fmt.Sprintf("image size %dx%d", tt.width, tt.height))
				require.ErrorContains(t, err, "[256, 5760]")
				require.NotContains(t, err.Error(), "signature")
			}
			require.Equal(t, 2, calls)
		})
	}
}

func TestMiniMaxH3ImageProbeFailureRemainsUpstreamValidated(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError, http.StatusOK} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client := &http.Client{Transport: miniMaxImageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("unrecognized image"))}, nil
			})}
			require.NoError(t, validateMiniMaxH3ImageDimensions(context.Background(), miniMaxImageTestBody(t, "https://example.com/image.png"), client))
		})
	}
	client := &http.Client{Transport: miniMaxImageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}
	require.NoError(t, validateMiniMaxH3ImageDimensions(context.Background(), miniMaxImageTestBody(t, "https://example.com/image.png"), client))
}

func TestMiniMaxH3ImageProbeBlocksPrivateURLsAndRedirects(t *testing.T) {
	for _, rawURL := range []string{"http://127.0.0.1/image.png", "http://10.0.0.1/image.png", "http://169.254.169.254/latest", "http://[::1]/image.png", "http://224.0.0.1/image.png", "http://localhost/image.png", "https://user:pass@example.com/image.png", "file:///tmp/image.png"} {
		client := &http.Client{Transport: miniMaxImageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("unsafe URL must not reach transport")
			return nil, nil
		})}
		_, ok := probeMiniMaxH3ImageDimensions(context.Background(), client, rawURL)
		require.False(t, ok, rawURL)
	}
	calls := 0
	client := &http.Client{
		CheckRedirect: miniMaxH3ImageProbeClient.CheckRedirect,
		Transport: miniMaxImageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"http://127.0.0.1/image.png"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}
	_, ok := probeMiniMaxH3ImageDimensions(context.Background(), client, "https://example.com/image.png")
	require.False(t, ok)
	require.Equal(t, 1, calls)
	_, err := safeDialContext(context.Background(), "tcp", "127.0.0.1:80")
	require.ErrorContains(t, err, "SSRF")
}

func TestMiniMaxH3ImageValidationPlatformIsolation(t *testing.T) {
	body := miniMaxImageTestBody(t, "https://example.com/image.png")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, platform := range []string{PlatformMiniMaxH3CompShare, PlatformByteDance, PlatformKling} {
		require.NoError(t, ValidatePPVideoInputMedia(ctx, platform, body))
	}
	require.NoError(t, ValidatePPVideoInputMedia(ctx, PlatformMiniMaxH3, []byte(`{"model":"MiniMax-Hailuo-2.3"}`)))
	require.NoError(t, ValidatePPVideoInputMedia(ctx, PlatformMiniMaxH3, body))
}

func TestMiniMaxH3ImageFailurePreservesProviderReason(t *testing.T) {
	body := []byte(`{"output":{"task_id":"h3-test","task_status":"Failure","error_message":"task failed, reason:content[2].image_url: invalid param: image size 384x216, expected each side in [256, 5760]"}}`)
	parsed, err := ParsePPVideoResponse(PlatformMiniMaxH3, body)
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusFailed, parsed.Status)
	require.Contains(t, parsed.ErrorMessage, "384x216")
}
