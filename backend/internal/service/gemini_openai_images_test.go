package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildGeminiImageRequestBody(t *testing.T) {
	req := &OpenAIImagesRequest{
		Model:        "gemini-3.1-flash-image",
		Prompt:       "a red bicycle in a studio",
		Size:         "1024x1536",
		SizeTier:     "1K",
		ExplicitSize: true,
		N:            1,
	}

	body, err := BuildGeminiImageRequestBody(req)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"contents":[{"role":"user","parts":[{"text":"a red bicycle in a studio"}]}],
		"generationConfig":{
			"responseModalities":["TEXT","IMAGE"],
			"imageConfig":{"aspectRatio":"2:3","imageSize":"1K"}
		}
	}`, string(body))
}

func TestBuildGeminiImageRequestBody_IncludesMultipartUploads(t *testing.T) {
	req := &OpenAIImagesRequest{
		Endpoint: openAIImagesEditsEndpoint,
		Model:    "gemini-3.1-flash-image",
		Prompt:   "use this reference",
		N:        1,
		Uploads: []OpenAIImagesUpload{
			{
				FieldName:   "image[]",
				FileName:    "reference.png",
				ContentType: "image/png",
				Data:        []byte("png"),
			},
		},
	}

	body, err := BuildGeminiImageRequestBody(req)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"contents":[{"role":"user","parts":[
			{"text":"use this reference"},
			{"inlineData":{"mimeType":"image/png","data":"cG5n"}}
		]}],
		"generationConfig":{
			"responseModalities":["TEXT","IMAGE"],
			"imageConfig":{"aspectRatio":"1:1"}
		}
	}`, string(body))
}

func TestValidateOpenAIImagesModelAcceptsGeminiImageModel(t *testing.T) {
	require.NoError(t, validateOpenAIImagesModel("gemini-3.1-flash-image"))
	require.NoError(t, validateOpenAIImagesModel("models/gemini-3.1-flash-image"))
	require.NoError(t, validateOpenAIImagesModel("gemini-3.7-flash"))
}

func TestBuildGeminiImageRequestBodyRejectsUnsupportedShape(t *testing.T) {
	_, err := BuildGeminiImageRequestBody(&OpenAIImagesRequest{Prompt: "two images", N: 2})
	require.EqualError(t, err, "Gemini image groups support n=1 only")
}

func TestConvertGeminiImageResponse(t *testing.T) {
	body := []byte(`{
		"candidates":[{
			"content":{"parts":[
				{"text":"done"},
				{"inlineData":{"mimeType":"image/png","data":"QUJD"}}
			]}
		}]
	}`)

	converted, err := ConvertGeminiImageResponse(body, false, "")
	require.NoError(t, err)

	var response struct {
		Created int64 `json:"created"`
		Data    []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(converted, &response))
	require.NotZero(t, response.Created)
	require.Len(t, response.Data, 1)
	require.Equal(t, "QUJD", response.Data[0].B64JSON)
}

func TestConvertGeminiImageResponseFromSSEDeduplicatesCumulativeParts(t *testing.T) {
	body := []byte(strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"QUJD"}}]}}]}`,
		`data: {"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"QUJD"}},{"inlineData":{"mimeType":"image/webp","data":"REVG"}}]}}]}`,
		`data: [DONE]`,
		"",
	}, "\n"))

	converted, err := ConvertGeminiImageResponse(body, true, "url")
	require.NoError(t, err)
	require.Contains(t, string(converted), `"url":"data:image/png;base64,QUJD"`)
	require.Contains(t, string(converted), `"url":"data:image/webp;base64,REVG"`)
	require.Equal(t, 1, strings.Count(string(converted), "data:image/png;base64,QUJD"))
	require.Contains(t, string(converted), "data: [DONE]")
}
