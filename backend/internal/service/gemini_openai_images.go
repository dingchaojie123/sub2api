package service

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// BuildGeminiImageRequestBody converts the OpenAI Images API generation shape
// into the native Gemini generateContent request shape.
func BuildGeminiImageRequestBody(req *OpenAIImagesRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("parsed images request is required")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if req.N != 1 {
		return nil, fmt.Errorf("Gemini image groups support n=1 only")
	}

	generationConfig := map[string]any{
		"responseModalities": []string{"TEXT", "IMAGE"},
	}
	imageConfig := map[string]any{
		"aspectRatio": "1:1",
	}
	if aspectRatio := geminiAspectRatioFromOpenAIImageSize(req.Size); aspectRatio != "" {
		imageConfig["aspectRatio"] = aspectRatio
	}
	if req.ExplicitSize && strings.TrimSpace(req.SizeTier) != "" {
		imageConfig["imageSize"] = req.SizeTier
	}
	if len(imageConfig) > 0 {
		generationConfig["imageConfig"] = imageConfig
	}

	parts := []map[string]any{
		{"text": strings.TrimSpace(req.Prompt)},
	}
	for _, upload := range req.Uploads {
		if len(upload.Data) == 0 {
			continue
		}
		mimeType := strings.TrimSpace(upload.ContentType)
		if mimeType == "" {
			mimeType = http.DetectContentType(upload.Data)
		}
		parts = append(parts, map[string]any{
			"inlineData": map[string]any{
				"mimeType": mimeType,
				"data":     base64.StdEncoding.EncodeToString(upload.Data),
			},
		})
	}

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": parts,
			},
		},
		"generationConfig": generationConfig,
	}
	return json.Marshal(payload)
}

// ConvertGeminiImageResponse converts native Gemini image parts into the
// OpenAI Images API response shape. Streaming requests are returned as SSE
// events after the upstream Gemini stream has been fully collected.
func ConvertGeminiImageResponse(body []byte, stream bool, responseFormat string) ([]byte, error) {
	images := extractGeminiImageResponseParts(body)
	if len(images) == 0 {
		return nil, fmt.Errorf("Gemini response did not contain an image")
	}

	data := make([]map[string]string, 0, len(images))
	for _, image := range images {
		dataURL := fmt.Sprintf("data:%s;base64,%s", image.MimeType, image.Data)
		if strings.EqualFold(strings.TrimSpace(responseFormat), "url") {
			data = append(data, map[string]string{"url": dataURL})
		} else {
			data = append(data, map[string]string{"b64_json": image.Data})
		}
	}

	if stream {
		var out bytes.Buffer
		for _, item := range data {
			event, err := json.Marshal(item)
			if err != nil {
				return nil, err
			}
			_, _ = fmt.Fprintf(&out, "data: %s\n\n", event)
		}
		out.WriteString("data: [DONE]\n\n")
		return out.Bytes(), nil
	}

	return json.Marshal(map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
	})
}

type geminiImageResponsePart struct {
	MimeType string
	Data     string
}

func extractGeminiImageResponseParts(body []byte) []geminiImageResponsePart {
	if len(body) == 0 {
		return nil
	}

	var images []geminiImageResponsePart
	seen := make(map[string]struct{})
	consume := func(value any) {}
	consume = func(value any) {
		switch current := value.(type) {
		case []any:
			for _, item := range current {
				consume(item)
			}
		case map[string]any:
			for key, item := range current {
				if key == "inlineData" || key == "inline_data" {
					if inline, ok := item.(map[string]any); ok {
						mimeType := strings.TrimSpace(geminiImageStringValue(inline, "mimeType", "mime_type"))
						data := strings.TrimSpace(geminiImageStringValue(inline, "data"))
						if data != "" {
							if mimeType == "" {
								mimeType = "image/png"
							}
							dedupeKey := mimeType + ":" + data
							if _, exists := seen[dedupeKey]; !exists {
								seen[dedupeKey] = struct{}{}
								images = append(images, geminiImageResponsePart{
									MimeType: mimeType,
									Data:     data,
								})
							}
						}
					}
					continue
				}
				consume(item)
			}
		}
	}

	var payload any
	if json.Unmarshal(body, &payload) == nil {
		consume(payload)
	}

	// Native streaming responses are newline-delimited SSE. This also handles
	// a mixed response where an upstream wrapper emits a final [DONE] marker.
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payloadText := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payloadText == "" || payloadText == "[DONE]" {
			continue
		}
		var event any
		if json.Unmarshal([]byte(payloadText), &event) == nil {
			consume(event)
		}
	}
	return images
}

func geminiImageStringValue(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok {
			return text
		}
	}
	return ""
}

func geminiAspectRatioFromOpenAIImageSize(size string) string {
	size = strings.TrimSpace(strings.ToLower(size))
	if size == "" {
		return ""
	}
	if strings.Contains(size, ":") {
		switch size {
		case "1:1", "2:3", "3:2", "3:4", "4:3", "9:16", "16:9":
			return size
		default:
			return ""
		}
	}

	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return ""
	}
	width, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return ""
	}

	type ratio struct {
		name  string
		value float64
	}
	candidates := []ratio{
		{name: "1:1", value: 1},
		{name: "2:3", value: 2.0 / 3.0},
		{name: "3:2", value: 3.0 / 2.0},
		{name: "3:4", value: 3.0 / 4.0},
		{name: "4:3", value: 4.0 / 3.0},
		{name: "9:16", value: 9.0 / 16.0},
		{name: "16:9", value: 16.0 / 9.0},
	}
	target := float64(width) / float64(height)
	best := candidates[0]
	bestDistance := absFloat(target - best.value)
	for _, candidate := range candidates[1:] {
		if distance := absFloat(target - candidate.value); distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	return best.name
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
