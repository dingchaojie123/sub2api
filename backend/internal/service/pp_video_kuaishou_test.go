package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPreparePPVideoRequestBodyKuaishouSound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantType  string
		wantSound string
		wantError bool
	}{
		{
			name:      "text to video defaults to sound on",
			body:      `{"model":"kling-v3","prompt":"waves breaking on the shore"}`,
			wantType:  "t2v",
			wantSound: "on",
		},
		{
			name:      "image to video defaults to sound on",
			body:      `{"prompt":"waves","image_url":"https://example.com/shore.png"}`,
			wantType:  "i2v",
			wantSound: "on",
		},
		{
			name:      "native image request defaults to sound on",
			body:      `{"input":{"prompt":"waves","first_frame_url":"https://example.com/shore.png"},"parameters":{"kling_v3_type":"i2v"}}`,
			wantType:  "i2v",
			wantSound: "on",
		},
		{
			name:      "explicit audio enabled",
			body:      `{"prompt":"waves","generate_audio":true}`,
			wantType:  "t2v",
			wantSound: "on",
		},
		{
			name:      "explicit audio disabled",
			body:      `{"prompt":"waves","generate_audio":false}`,
			wantType:  "t2v",
			wantSound: "off",
		},
		{
			name:      "top level sound disabled",
			body:      `{"prompt":"waves","sound":"off"}`,
			wantType:  "t2v",
			wantSound: "off",
		},
		{
			name:      "native image sound disabled",
			body:      `{"input":{"prompt":"waves","first_frame_url":"https://example.com/shore.png"},"parameters":{"sound":"off"}}`,
			wantType:  "i2v",
			wantSound: "off",
		},
		{
			name:      "nested audio disabled",
			body:      `{"input":{"prompt":"waves"},"parameters":{"generate_audio":false}}`,
			wantType:  "t2v",
			wantSound: "off",
		},
		{
			name:     "motion control does not default generated sound",
			body:     `{"input":{"img_url":"https://example.com/shore.png","video_url":"https://example.com/motion.mp4"},"parameters":{"keep_original_sound":"yes"}}`,
			wantType: "motion_control",
		},
		{
			name:      "invalid sound remains rejected",
			body:      `{"prompt":"waves","parameters":{"sound":"invalid"}}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body, _, err := PreparePPVideoRequestBody(PlatformKuaishou, PPVideoOperationGeneric, []byte(tt.body))
			if tt.wantError {
				require.ErrorContains(t, err, "Kuaishou sound must be on or off")
				return
			}
			require.NoError(t, err)
			require.Equal(t, "kling-v3", gjson.GetBytes(body, "model").String())
			require.Equal(t, tt.wantType, gjson.GetBytes(body, "parameters.kling_v3_type").String())
			if tt.wantSound == "" {
				require.False(t, gjson.GetBytes(body, "parameters.sound").Exists())
				require.Equal(t, "yes", gjson.GetBytes(body, "parameters.keep_original_sound").String())
			} else {
				require.Equal(t, tt.wantSound, gjson.GetBytes(body, "parameters.sound").String())
			}
			require.False(t, gjson.GetBytes(body, "sound").Exists())
			require.False(t, gjson.GetBytes(body, "generate_audio").Exists())

			reprepared, _, err := PreparePPVideoRequestBody(PlatformKuaishou, PPVideoOperationGeneric, body)
			require.NoError(t, err)
			require.JSONEq(t, string(body), string(reprepared))
		})
	}
}
