package service

import (
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

// PPVideoPromptMaxRunes 是写入任务记录的提示词长度上限（按字符计）。
const PPVideoPromptMaxRunes = 4000

// PPVideoPromptFromRequest 从用户提交的视频生成请求体里提取提示词，仅用于任务记录页展示。
//
// 兼容两类请求：
//   - 平铺字段：prompt / input.prompt / parameters.prompt / data.prompt / text；
//   - Seedance 风格的 content 数组：把其中 type=text 的片段按顺序用换行拼接。
//
// 结果去掉首尾空白、NUL 与非法 UTF-8（PostgreSQL TEXT 不接受），并截断到 PPVideoPromptMaxRunes。
func PPVideoPromptFromRequest(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	prompt := extractPPVideoText(body, "prompt", "input.prompt", "parameters.prompt", "data.prompt", "text")
	if prompt == "" {
		prompt = ppVideoPromptFromContentArray(body)
	}
	return sanitizePPVideoPrompt(prompt)
}

func ppVideoPromptFromContentArray(body []byte) string {
	content := gjson.GetBytes(body, "content")
	if !content.IsArray() {
		return ""
	}
	parts := make([]string, 0, 2)
	content.ForEach(func(_, item gjson.Result) bool {
		if !strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "text") {
			return true
		}
		if text := strings.TrimSpace(item.Get("text").String()); text != "" {
			parts = append(parts, text)
		}
		return true
	})
	return strings.Join(parts, "\n")
}

func sanitizePPVideoPrompt(prompt string) string {
	prompt = strings.ToValidUTF8(prompt, "")
	prompt = strings.ReplaceAll(prompt, "\x00", "")
	prompt = strings.TrimSpace(prompt)
	if utf8.RuneCountInString(prompt) <= PPVideoPromptMaxRunes {
		return prompt
	}
	runes := []rune(prompt)
	return strings.TrimSpace(string(runes[:PPVideoPromptMaxRunes]))
}
