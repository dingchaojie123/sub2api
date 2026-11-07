package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// PPVideoFailureBodyMaxChars 是读取失败任务上游响应时的截取上限（按字符计），只用于提取失败原因。
const PPVideoFailureBodyMaxChars = 8192

// PPVideoFailureReasonMaxRunes 是展示给用户的失败原因长度上限。
const PPVideoFailureReasonMaxRunes = 500

// 各上游放置失败原因的字段并不统一。按可靠性从高到低排列；
// 通用字段（message / msg / detail）放在后面，且只会在任务已经失败时使用。
var ppVideoFailureReasonFields = []string{
	"output.error_message",
	"output.error",
	"error_message",
	"errorMessage",
	"error.message",
	"error.msg",
	"error_msg",
	"fail_reason",
	"failReason",
	"failure_reason",
	"failure.message",
	"reason",
	"message",
	"msg",
	"detail",
	"error",
}

var ppVideoFailureReasonPrefixes = []string{"", "data.", "data.data.", "task.", "result."}

// 这些词不携带任何信息，不能当作失败原因。
var ppVideoUninformativeReasons = map[string]struct{}{
	"success": {}, "succeeded": {}, "ok": {}, "done": {}, "completed": {},
	"failed": {}, "fail": {}, "failure": {}, "error": {}, "null": {}, "none": {},
}

// PPVideoFailureReasonFromResponse 从上游保存的响应里尽力提取失败原因，找不到返回空串。
// 只读取字符串类型的字段，避免把错误码、对象整体当成原因。
func PPVideoFailureReasonFromResponse(body string) string {
	raw := []byte(body)
	if len(raw) == 0 || !gjson.ValidBytes(raw) {
		return ""
	}
	for _, field := range ppVideoFailureReasonFields {
		for _, prefix := range ppVideoFailureReasonPrefixes {
			value := gjson.GetBytes(raw, prefix+field)
			if !value.Exists() || value.Type != gjson.String {
				continue
			}
			reason := sanitizePPVideoFailureReason(value.String())
			if reason == "" {
				continue
			}
			if _, skip := ppVideoUninformativeReasons[strings.ToLower(reason)]; skip {
				continue
			}
			return reason
		}
	}
	return ""
}

func sanitizePPVideoFailureReason(reason string) string {
	reason = strings.ToValidUTF8(reason, "")
	reason = strings.ReplaceAll(reason, "\x00", "")
	reason = strings.TrimSpace(reason)
	runes := []rune(reason)
	if len(runes) > PPVideoFailureReasonMaxRunes {
		return strings.TrimSpace(string(runes[:PPVideoFailureReasonMaxRunes]))
	}
	return reason
}

// FailureReason 返回失败任务展示用的失败原因：优先用任务自身记录的错误信息，
// 没有时再从保存的上游响应里提取。
func (r *PPVideoTaskRecord) FailureReason() string {
	if r == nil {
		return ""
	}
	if message := strings.TrimSpace(r.LastErrorMessage); message != "" {
		return message
	}
	return PPVideoFailureReasonFromResponse(r.FailureResponseBody)
}
