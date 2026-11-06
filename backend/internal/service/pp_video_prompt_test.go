package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPPVideoPromptFromRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "flat prompt", body: `{"model":"m","prompt":"  a cat on a roof  "}`, want: "a cat on a roof"},
		{name: "nested input prompt", body: `{"input":{"prompt":"nested"}}`, want: "nested"},
		{name: "parameters prompt", body: `{"parameters":{"prompt":"params"}}`, want: "params"},
		{
			name: "seedance content array keeps only text parts",
			body: `{"content":[{"type":"text","text":"scene one"},{"type":"image_url","image_url":{"url":"https://x/y.png"}},{"type":"text","text":"scene two"}]}`,
			want: "scene one\nscene two",
		},
		{name: "prompt wins over content", body: `{"prompt":"flat","content":[{"type":"text","text":"array"}]}`, want: "flat"},
		{name: "no prompt", body: `{"model":"m","duration":5}`, want: ""},
		{name: "content without text", body: `{"content":[{"type":"image_url","image_url":{"url":"u"}}]}`, want: ""},
		{name: "invalid json", body: `not json`, want: ""},
		{name: "empty body", body: ``, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, PPVideoPromptFromRequest([]byte(tt.body)))
		})
	}
}

func TestPPVideoPromptFromRequestStripsNULAndInvalidUTF8(t *testing.T) {
	body := []byte("{\"prompt\":\"ab\\u0000cd\"}")
	require.Equal(t, "abcd", PPVideoPromptFromRequest(body))

	require.Equal(t, "ok", sanitizePPVideoPrompt("o\xffk"))
}

func TestPPVideoPromptFromRequestTruncatesByRune(t *testing.T) {
	long := strings.Repeat("字", PPVideoPromptMaxRunes+50)
	got := PPVideoPromptFromRequest([]byte(`{"prompt":"` + long + `"}`))

	require.Equal(t, PPVideoPromptMaxRunes, len([]rune(got)))
	require.True(t, strings.HasPrefix(long, got))
}
