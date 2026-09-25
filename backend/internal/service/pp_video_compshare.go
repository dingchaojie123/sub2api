package service

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	CompShareVideoDefaultBaseURL = "https://cp.compshare.cn"
	CompShareVideoLiteModel      = "minimax-h3-lite"
)

func compShareVideoPrices(resolution string, apiKey *APIKey) (float64, float64, error) {
	if apiKey == nil || apiKey.Group == nil {
		return 0, 0, fmt.Errorf("CompShare group pricing is required")
	}
	group := apiKey.Group
	var price *float64
	switch strings.ToLower(resolution) {
	case "480p":
		price = group.VideoPrice480P
	case "768p":
		price = group.VideoPrice720P
	case "1080p":
		price = group.VideoPrice1080P
	case "2k":
		price = group.VideoPrice2K
	case "4k":
		price = group.VideoPrice4K
	}
	valid := func(value *float64) bool {
		return value != nil && *value >= 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
	}
	if !valid(price) || !valid(group.VideoPrice720P) {
		return 0, 0, fmt.Errorf("configure CompShare %s and 768P prices in the group before generating video", resolution)
	}
	return *price, *group.VideoPrice720P, nil
}

func compShareVideoModelAllowed(model string) bool {
	return ppVideoIsPublicDefaultAlias(model) || model == MiniMaxH3VideoDefaultModel || model == CompShareVideoLiteModel
}

func (s *AccountTestService) testCompShareVideoAccount(c *gin.Context, account *Account) error {
	if s.httpUpstream == nil {
		return s.sendErrorAndEnd(c, "HTTP upstream not configured")
	}
	if account.Type != AccountTypeAPIKey || strings.TrimSpace(account.GetOpenAIApiKey()) == "" {
		return s.sendErrorAndEnd(c, "CompShare requires an API key account")
	}
	baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
	if baseURL == "" {
		baseURL = CompShareVideoDefaultBaseURL
	}
	validated, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return s.sendErrorAndEnd(c, "Invalid CompShare base URL")
	}
	baseURL, err = normalizePPVideoBaseURL(validated)
	if err != nil {
		return s.sendErrorAndEnd(c, "Invalid CompShare base URL")
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, baseURL+"/minimax/v2/query/point_usage_summary", nil)
	if err != nil {
		return s.sendErrorAndEnd(c, "Invalid CompShare request URL")
	}
	req.Header, err = PPVideoRequestHeaders(account.Platform, account.GetOpenAIApiKey())
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}
	account.ApplyHeaderOverrides(req.Header)
	resp, err := s.doUpstreamModelsRequest(req, upstreamModelsProxyURL(account), account)
	if err != nil {
		return s.sendErrorAndEnd(c, "CompShare balance probe failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, upstreamModelsBodyLimit+1))
	if err != nil || int64(len(body)) > upstreamModelsBodyLimit || resp.StatusCode < 200 || resp.StatusCode >= 300 || !gjson.ValidBytes(body) {
		return s.sendErrorAndEnd(c, fmt.Sprintf("CompShare balance probe failed (HTTP %d)", resp.StatusCode))
	}
	points := gjson.GetBytes(body, "available_points")
	if points.Type != gjson.Number {
		return s.sendErrorAndEnd(c, "CompShare balance response is missing available_points")
	}
	s.sendEvent(c, TestEvent{Type: "content", Text: fmt.Sprintf("CompShare connection succeeded; available points: %d", points.Int()), Data: map[string]any{"available_points": points.Int(), "models": []string{MiniMaxH3VideoDefaultModel, CompShareVideoLiteModel}}})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func compShareVideoUpstreamModel(public PPVideoPublicRequest, account *Account) string {
	if account != nil {
		if model, ok := account.ResolveMappedModel(public.Model); ok && compShareVideoModelAllowed(model) && !ppVideoIsPublicDefaultAlias(model) {
			return model
		}
	}
	if public.Model == CompShareVideoLiteModel {
		return CompShareVideoLiteModel
	}
	return MiniMaxH3VideoDefaultModel
}

func normalizePPVideoCompSharePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if len(body) > 72*1024*1024 {
		return nil, fmt.Errorf("CompShare request body must not exceed 72 MiB")
	}
	if !compShareVideoModelAllowed(public.Model) {
		return nil, fmt.Errorf("model %q is not supported by CompShare", public.Model)
	}
	duration := public.DurationMilliseconds
	if err := validateCompShareVideoDuration(duration); err != nil {
		return nil, err
	}
	for _, path := range []string{"duration", "parameters.duration"} {
		if value := gjson.GetBytes(body, path); value.Exists() && (value.Type != gjson.Number || value.Float() != float64(duration)/1000) {
			return nil, fmt.Errorf("CompShare duration must be an integer from 4 to 30")
		}
	}
	content := gjson.GetBytes(body, "content")
	if !content.Exists() {
		content = gjson.GetBytes(body, "input.content")
	}
	var source []any
	if content.Exists() {
		if !content.IsArray() || json.Unmarshal([]byte(content.Raw), &source) != nil || len(source) == 0 {
			return nil, fmt.Errorf("CompShare content must be a non-empty array")
		}
	} else if public.Prompt != "" {
		source = []any{map[string]any{"type": "text", "text": public.Prompt}}
	} else {
		return nil, fmt.Errorf("CompShare content is required")
	}
	items, prompt, hasImage, err := normalizeCompShareVideoContent(source)
	if err != nil {
		return nil, err
	}
	resolution := extractPPVideoText(body, "resolution", "parameters.resolution")
	// CompShare explicitly treats all non-standard spellings as native 768P.
	switch resolution {
	case "480P", "768P", "1080P", "2K", "4K":
	default:
		resolution = "768P"
	}
	ratio := extractPPVideoText(body, "ratio", "parameters.ratio", "aspect_ratio")
	if ratio == "" || (ratio == "adaptive" && !hasImage) {
		ratio = "16:9"
	}
	switch ratio {
	case "adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16":
	default:
		return nil, fmt.Errorf("CompShare ratio is not supported")
	}
	if public.VideoCount != 1 {
		return nil, fmt.Errorf("CompShare supports one video per task")
	}
	payload := map[string]any{
		"model": public.UpstreamModel, "content": items,
		"duration": duration / 1000, "resolution": resolution, "ratio": ratio,
	}
	for _, name := range []string{"use_context_ir", "mute_audio", "aigc_watermark"} {
		value, exists := ppVideoJSONValue(body, name, "parameters."+name)
		if !exists {
			continue
		}
		flag, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("CompShare %s must be boolean", name)
		}
		payload[name] = flag
	}
	for _, name := range []string{"skill_id", "callback_url", "callback_token"} {
		value, exists := ppVideoJSONValue(body, name, "parameters."+name)
		if !exists {
			continue
		}
		valueString, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("CompShare %s must be a string", name)
		}
		payload[name] = valueString
	}
	if skill, _ := payload["skill_id"].(string); skill != "" && payload["use_context_ir"] != true {
		return nil, fmt.Errorf("CompShare skill_id requires use_context_ir=true")
	}
	callbackURL, _ := payload["callback_url"].(string)
	callbackToken, _ := payload["callback_token"].(string)
	if utf8.RuneCountInString(callbackToken) > 512 || (callbackToken != "" && callbackURL == "") {
		return nil, fmt.Errorf("CompShare callback_token requires callback_url and must not exceed 512 characters")
	}
	if callbackURL != "" && (utf8.RuneCountInString(callbackURL) > 1024 || !compShareVideoURLValid(callbackURL, false)) {
		return nil, fmt.Errorf("CompShare callback_url must be an absolute HTTP(S) URL up to 1024 characters")
	}
	public.Prompt, public.HasImage = prompt, hasImage
	public.Resolution = strings.ToLower(resolution)
	return payload, nil
}

func validateCompShareVideoDuration(duration int64) error {
	if duration < 4000 || duration > 30000 || duration%1000 != 0 {
		return fmt.Errorf("CompShare duration must be an integer from 4 to 30 seconds")
	}
	return nil
}

func normalizeCompShareVideoContent(source []any) ([]any, string, bool, error) {
	items := make([]any, 0, len(source))
	texts := []string{}
	counts := map[string]int{}
	for _, raw := range source {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, "", false, fmt.Errorf("CompShare content items must be objects")
		}
		kind, _ := item["type"].(string)
		if kind == "text" {
			text, ok := item["text"].(string)
			if !ok {
				return nil, "", false, fmt.Errorf("CompShare text must be a string")
			}
			texts = append(texts, text)
			items = append(items, map[string]any{"type": kind, "text": text})
			continue
		}
		role, roleExists := item["role"].(string)
		if _, exists := item["role"]; exists && !roleExists {
			return nil, "", false, fmt.Errorf("CompShare media role must be a string")
		}
		switch kind {
		case "image_url":
			if role == "" {
				role = "first_frame"
			}
			if role != "first_frame" && role != "last_frame" && role != "reference_image" {
				return nil, "", false, fmt.Errorf("CompShare image role is not supported")
			}
		case "video_url":
			if role == "" {
				role = "reference_video"
			}
			if role != "reference_video" {
				return nil, "", false, fmt.Errorf("CompShare video role is not supported")
			}
		case "audio_url":
			if role == "" {
				role = "reference_audio"
			}
			if role != "reference_audio" {
				return nil, "", false, fmt.Errorf("CompShare audio role is not supported")
			}
		default:
			return nil, "", false, fmt.Errorf("CompShare content type %q is not supported", kind)
		}
		media, ok := item[kind].(map[string]any)
		if !ok {
			return nil, "", false, fmt.Errorf("CompShare %s must contain url", kind)
		}
		mediaURL, _ := media["url"].(string)
		if !compShareVideoURLValid(mediaURL, true) {
			return nil, "", false, fmt.Errorf("CompShare media must be an HTTP(S) or Data URL")
		}
		counts[role]++
		items = append(items, map[string]any{"type": kind, kind: map[string]any{"url": mediaURL}, "role": role})
	}
	frames := counts["first_frame"] + counts["last_frame"]
	refs := counts["reference_image"] + counts["reference_video"] + counts["reference_audio"]
	prompt := strings.Join(texts, "\n")
	if utf8.RuneCountInString(prompt) > 7000 || (frames+refs == 0 && strings.TrimSpace(prompt) == "") {
		return nil, "", false, fmt.Errorf("CompShare text must be at most 7000 characters and is required without media")
	}
	if counts["first_frame"] > 1 || counts["last_frame"] > 1 || counts["reference_image"] > 9 || counts["reference_video"] > 3 || counts["reference_audio"] > 3 || refs > 12 {
		return nil, "", false, fmt.Errorf("CompShare content exceeds media limits")
	}
	if frames > 0 && refs > 0 {
		return nil, "", false, fmt.Errorf("CompShare cannot combine frames with reference media")
	}
	if counts["reference_audio"] > 0 && counts["reference_image"]+counts["reference_video"] == 0 {
		return nil, "", false, fmt.Errorf("CompShare reference audio requires a reference image or video")
	}
	return items, prompt, frames+counts["reference_image"] > 0, nil
}

func compShareVideoURLValid(raw string, allowData bool) bool {
	if allowData && strings.HasPrefix(raw, "data:") {
		header, data, ok := strings.Cut(raw, ",")
		return ok && len(header) > 5 && data != ""
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil
}

func parseCompShareVideoResponse(body []byte) (PPVideoResponse, error) {
	if code := gjson.GetBytes(body, "RetCode"); code.Exists() && code.Int() != 0 {
		return PPVideoResponse{}, fmt.Errorf("CompShare: %s", extractPPVideoText(body, "Message"))
	}
	if gjson.GetBytes(body, "type").String() == "error" {
		return PPVideoResponse{}, fmt.Errorf("CompShare: %s", extractPPVideoText(body, "error.message"))
	}
	result := PPVideoResponse{
		TaskID:                         extractPPVideoText(body, "task.id", "task_id"),
		Status:                         NormalizePPVideoTaskStatus(extractPPVideoText(body, "task.status", "status")),
		GeneratedDurationMilliseconds:  extractPPVideoDurationMilliseconds(body, "task.usage.output_seconds", "task.duration"),
		InputVideoDurationMilliseconds: extractPPVideoDurationMilliseconds(body, "task.usage.input_seconds"),
		VideoResolution:                strings.ToLower(extractPPVideoText(body, "task.resolution")),
		ErrorMessage:                   extractPPVideoText(body, "task.error.message"),
		VideoCount:                     1,
		RawBody:                        append([]byte(nil), body...),
	}
	if result.Status == "" || (result.Status == PPVideoTaskStatusSucceeded && !compShareVideoURLValid(extractPPVideoText(body, "task.content.url"), false)) {
		result.Status = PPVideoTaskStatusProcessing
	}
	return result, nil
}
