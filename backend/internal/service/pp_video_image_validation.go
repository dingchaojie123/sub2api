package service

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/tidwall/gjson"
	_ "golang.org/x/image/webp"
)

const miniMaxH3ImageProbeBytes int64 = 1 << 20

var miniMaxH3ImageProbeClient = &http.Client{
	Timeout: 3 * time.Second,
	Transport: &http.Transport{
		DialContext:           safeDialContext,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       30 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !miniMaxH3ImageProbeURLAllowed(req) {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

// ValidatePPVideoInputMedia checks known input limits before reserving funds.
// CompShare has its own image preprocessing and must retain its own contract.
func ValidatePPVideoInputMedia(ctx context.Context, platform string, body []byte) error {
	if platform != PlatformMiniMaxH3 || PPVideoModelFromBody(body) != MiniMaxH3VideoDefaultModel {
		return nil
	}
	return validateMiniMaxH3ImageDimensions(ctx, body, miniMaxH3ImageProbeClient)
}

func validateMiniMaxH3ImageDimensions(ctx context.Context, body []byte, client *http.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	for index, item := range gjson.GetBytes(body, "input.content").Array() {
		if item.Get("type").String() != "image_url" {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		cfg, ok := probeMiniMaxH3ImageDimensions(ctx, client, item.Get("image_url.url").String())
		// A local fetch failure does not prove the provider cannot fetch the URL.
		if !ok {
			continue
		}
		if cfg.Width < 256 || cfg.Height < 256 || cfg.Width > 5760 || cfg.Height > 5760 {
			return fmt.Errorf("MiniMax-H3 input.content[%d].image_url: image size %dx%d, expected each side in [256, 5760] pixels; upload the original image or resize it proportionally before submitting", index, cfg.Width, cfg.Height)
		}
	}
	return nil
}

func miniMaxH3ImageProbeURLAllowed(req *http.Request) bool {
	u := req.URL
	if (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || isBlockedHostname(u.Hostname()) {
		return false
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		return ip.IsGlobalUnicast() && !isPrivateIP(ip)
	}
	return true
}

func probeMiniMaxH3ImageDimensions(ctx context.Context, client *http.Client, rawURL string) (image.Config, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil || !miniMaxH3ImageProbeURLAllowed(req) {
		return image.Config{}, false
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", miniMaxH3ImageProbeBytes-1))
	resp, err := client.Do(req)
	if err != nil {
		return image.Config{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return image.Config{}, false
	}
	cfg, _, err := image.DecodeConfig(io.LimitReader(resp.Body, miniMaxH3ImageProbeBytes))
	return cfg, err == nil
}
