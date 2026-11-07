package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ppVideoTaskUsesAuthenticatedContent 表示该平台的视频必须带账号凭证向上游拉取
// （上游不提供公开直链），其余平台的结果以直链形式保存在任务的最新响应里。
func ppVideoTaskUsesAuthenticatedContent(platform string) bool {
	switch strings.TrimSpace(platform) {
	case PlatformCTMOSeedance, PlatformXingAPISeedance, PlatformMaiTokenSeedance, PlatformOPONE:
		return true
	default:
		return false
	}
}

// ForwardPPVideoTaskUserContent 为「视频任务」记录页提供结果视频的字节流，调用方必须已校验任务归属。
//
//   - CTMO / Xingapi / OPONE：沿用带凭证的内容代理；
//   - 其他平台（如光影、Kling、Wan）：读取任务最新响应里的视频直链，经本站拉取。
//     拉取时不携带任何账号凭证，只接受 https，并复用 SSRF 防护与账号代理。
//
// 直链通常带签名且会过期；过期后会返回错误，页面显示「视频加载失败」，用户仍可用 API 重新查询。
func (s *OpenAIGatewayService) ForwardPPVideoTaskUserContent(ctx context.Context, account *Account, task *PPVideoTask, rangeHeader, ifRangeHeader string) (*http.Response, error) {
	if task != nil && ppVideoTaskUsesAuthenticatedContent(task.Platform) {
		return s.ForwardPPVideoTaskContent(ctx, account, task, rangeHeader, ifRangeHeader)
	}
	if s == nil || s.httpUpstream == nil || task == nil || account == nil ||
		account.ID != task.AccountID || account.Platform != task.Platform {
		return nil, fmt.Errorf("original video task account is required")
	}
	videoURL := strings.TrimSpace(ppVideoExtractVideoURL([]byte(task.ResponseBody)))
	if videoURL == "" {
		return nil, fmt.Errorf("video url is not available for this task")
	}
	if resp := s.tryOPONEDelivery(ctx, account, videoURL, rangeHeader, ifRangeHeader); resp != nil {
		return resp, nil
	}
	return nil, fmt.Errorf("video download failed")
}

func (s *OpenAIGatewayService) ForwardPPVideoTaskContentWithMethod(ctx context.Context, method string, account *Account, task *PPVideoTask, rangeHeader, ifRangeHeader string) (*http.Response, error) {
	if s == nil || s.httpUpstream == nil || task == nil || account == nil || account.ID != task.AccountID || account.Platform != task.Platform || task.TaskID == "" {
		return nil, fmt.Errorf("original video task account is required")
	}
	return s.forwardPPVideoContentWithMethod(ctx, method, account, task.TaskID, rangeHeader, ifRangeHeader)
}
