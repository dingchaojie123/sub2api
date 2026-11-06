package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 「视频任务」页把视频取回成 blob 后交给 <video> 播放。<video> 受 media-src 约束，
// 未配置时回退到 default-src 'self'，而它不允许 blob:，视频会黑屏。
func TestDefaultCSPPolicyAllowsBlobMedia(t *testing.T) {
	directives := map[string]string{}
	for _, part := range strings.Split(DefaultCSPPolicy, ";") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		directives[fields[0]] = strings.Join(fields[1:], " ")
	}

	mediaSrc, ok := directives["media-src"]
	require.True(t, ok, "media-src must be set explicitly, otherwise blob videos fall back to default-src 'self' and are blocked")
	require.Contains(t, strings.Fields(mediaSrc), "blob:")
	require.Contains(t, strings.Fields(mediaSrc), "'self'")
	require.Equal(t, "'self'", directives["default-src"], "default-src must stay restrictive")
}
