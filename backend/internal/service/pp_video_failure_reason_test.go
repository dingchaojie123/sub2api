package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPPVideoFailureReasonFromResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"guangying style message under data", `{"data":{"id":"job-1","status":"failed","message":"generation failed"}}`, "generation failed"},
		{"top level message", `{"status":"failed","message":"content rejected by moderation"}`, "content rejected by moderation"},
		{"msg field", `{"code":500,"msg":"upstream busy"}`, "upstream busy"},
		{"fail_reason field", `{"status":"FAILURE","fail_reason":"quota exceeded"}`, "quota exceeded"},
		{"nested data.data", `{"data":{"data":{"error_message":"bad reference image"}}}`, "bad reference image"},
		{"error object message", `{"error":{"message":"invalid prompt","code":"invalid_request"}}`, "invalid prompt"},
		{"error as plain string", `{"error":"model overloaded"}`, "model overloaded"},
		{"specific field beats generic message", `{"message":"failed","error_message":"image too small"}`, "image too small"},
		{"uninformative words are skipped", `{"message":"failed","reason":"model timed out"}`, "model timed out"},
		{"numeric error code is not a reason", `{"error":500,"message":"gateway timeout"}`, "gateway timeout"},
		{"only uninformative values", `{"message":"failed","error":"error"}`, ""},
		{"no reason at all", `{"status":"failed","task_id":"x"}`, ""},
		{"not json", `upstream exploded`, ""},
		{"empty", ``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, PPVideoFailureReasonFromResponse(tt.body))
		})
	}
}

func TestPPVideoFailureReasonFromResponseTruncatesAndCleans(t *testing.T) {
	long := strings.Repeat("错", PPVideoFailureReasonMaxRunes+40)
	got := PPVideoFailureReasonFromResponse(`{"message":"` + long + `"}`)
	require.Equal(t, PPVideoFailureReasonMaxRunes, len([]rune(got)))

	require.Equal(t, "ab", PPVideoFailureReasonFromResponse("{\"message\":\"a\\u0000b\"}"))
	require.Equal(t, "trimmed", PPVideoFailureReasonFromResponse(`{"message":"  trimmed  "}`))
}

func TestPPVideoTaskRecordFailureReasonPrefersTheRecordedMessage(t *testing.T) {
	record := &PPVideoTaskRecord{
		LastErrorMessage:    "recorded by the poller",
		FailureResponseBody: `{"message":"from the stored response"}`,
	}
	require.Equal(t, "recorded by the poller", record.FailureReason())

	record.LastErrorMessage = "   "
	require.Equal(t, "from the stored response", record.FailureReason())

	record.FailureResponseBody = ""
	require.Equal(t, "", record.FailureReason())

	var nilRecord *PPVideoTaskRecord
	require.Equal(t, "", nilRecord.FailureReason())
}
