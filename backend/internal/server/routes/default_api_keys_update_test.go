package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestDefaultAPIKeysExistingUserAutomaticallyFillsMissing(t *testing.T) {
	for _, existingCount := range []int{0, 2, 4} {
		t.Run(fmt.Sprintf("existing_%d", existingCount), func(t *testing.T) {
			f := newCustomerAccountFixture(t)
			ctx := context.Background()
			ddl, err := migrations.FS.ReadFile("192_user_default_api_keys.sql")
			require.NoError(t, err)
			_, err = f.client.ExecContext(ctx, strings.ReplaceAll(string(ddl), "'text', 'image', 'video'", "'text', 'image', 'video', 'audio'"))
			require.NoError(t, err)
			names := []string{"OC--ChatGPT【文本模型】", "OC--ChatGPT【生图】", "OC--Seedance【视频】", "OC--Qwen-TTS【音频】"}
			existingIDs := make(map[int]int64)
			for i, name := range names {
				group, err := f.client.Group.Create().SetName(name).Save(ctx)
				require.NoError(t, err)
				// Another customer's keys must never be adopted.
				_, err = f.client.APIKey.Create().SetUserID(f.otherID).SetName(name).SetKey(fmt.Sprintf("sk-other-%d", i)).SetGroupID(group.ID).Save(ctx)
				require.NoError(t, err)
				if i < existingCount {
					key, err := f.client.APIKey.Create().SetUserID(f.ownerID).SetName(name).SetKey(fmt.Sprintf("sk-existing-%d", i)).SetGroupID(group.ID).Save(ctx)
					require.NoError(t, err)
					existingIDs[i] = key.ID
				}
			}
			for _, token := range []string{"", "sk-model-key"} {
				w := f.request(http.MethodGet, "/api/v1/keys/defaults", token, "")
				require.Equal(t, http.StatusUnauthorized, w.Code)
			}
			var firstIDs []int64
			for _, method := range []string{http.MethodGet, http.MethodGet, http.MethodPost} {
				w := f.request(method, "/api/v1/keys/defaults", f.token, "")
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
				var body struct {
					Data []struct {
						Purpose string      `json:"purpose"`
						APIKey  *dto.APIKey `json:"api_key"`
					} `json:"data"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Len(t, body.Data, 4)
				ids := make([]int64, 0, 4)
				for i, item := range body.Data {
					require.NotNil(t, item.APIKey)
					require.Equal(t, []string{"text", "image", "video", "audio"}[i], item.Purpose)
					require.Equal(t, f.ownerID, item.APIKey.UserID)
					require.Equal(t, names[i], item.APIKey.Name)
					if id, ok := existingIDs[i]; ok {
						require.Equal(t, id, item.APIKey.ID)
						require.Equal(t, fmt.Sprintf("sk-existing-%d", i), item.APIKey.Key)
					}
					ids = append(ids, item.APIKey.ID)
				}
				if firstIDs == nil {
					firstIDs = ids
				} else {
					require.Equal(t, firstIDs, ids)
				}
				count, err := f.client.APIKey.Query().Where(apikey.UserIDEQ(f.ownerID)).Count(ctx)
				require.NoError(t, err)
				require.Equal(t, 4, count)
			}
		})
	}
}

func TestDefaultAPIKeysUpdateAndFreshRead(t *testing.T) {
	f := newCustomerAccountFixture(t)
	ctx := context.Background()
	ddl, err := migrations.FS.ReadFile("192_user_default_api_keys.sql")
	require.NoError(t, err)
	// SQLite fixtures use the final constraint instead of PostgreSQL ALTER CONSTRAINT.
	_, err = f.client.ExecContext(ctx, strings.ReplaceAll(string(ddl), "'text', 'image', 'video'", "'text', 'image', 'video', 'audio'"))
	require.NoError(t, err)
	textGroup, err := f.client.Group.Create().SetName("OC--ChatGPT【文本模型】").Save(ctx)
	require.NoError(t, err)
	imageGroup, err := f.client.Group.Create().SetName("OC--ChatGPT【生图】").Save(ctx)
	require.NoError(t, err)
	customTextGroup, err := f.client.Group.Create().SetName("Custom Qwen Text").SetPlatform(service.PlatformQwen).Save(ctx)
	require.NoError(t, err)
	customImageGroup, err := f.client.Group.Create().SetName("Custom Image").SetPlatform(service.PlatformOpenAI).SetAllowImageGeneration(true).Save(ctx)
	require.NoError(t, err)
	videoGroup, err := f.client.Group.Create().SetName("OC--Seedance【视频】").SetPlatform(service.PlatformSeedance).Save(ctx)
	require.NoError(t, err)
	customVideoGroup, err := f.client.Group.Create().SetName("Custom Kling Video").SetPlatform(service.PlatformKling).Save(ctx)
	require.NoError(t, err)
	customAudioGroup, err := f.client.Group.Create().SetName("Custom Qwen TTS").SetPlatform(service.PlatformQwenTTS).Save(ctx)
	require.NoError(t, err)
	for _, name := range []string{"OC--Seedance【视频】", "OC--Qwen-TTS【音频】"} {
		if name == "OC--Seedance【视频】" {
			continue
		}
		_, err := f.client.Group.Create().SetName(name).Save(ctx)
		require.NoError(t, err)
	}
	oldKey, err := f.client.APIKey.Create().SetUserID(f.ownerID).SetName("old").SetKey("sk-old-default").SetGroupID(textGroup.ID).Save(ctx)
	require.NoError(t, err)
	newKey, err := f.client.APIKey.Create().SetUserID(f.ownerID).SetName("new").SetKey("sk-new-default").SetGroupID(textGroup.ID).SetQuota(50).SetQuotaUsed(4).Save(ctx)
	require.NoError(t, err)
	customTextKey, err := f.client.APIKey.Create().SetUserID(f.ownerID).SetName("custom text").SetKey("sk-custom-text-default").SetGroupID(customTextGroup.ID).Save(ctx)
	require.NoError(t, err)
	imageKey, err := f.client.APIKey.Create().SetUserID(f.ownerID).SetName("image").SetKey("sk-image-default").SetGroupID(imageGroup.ID).Save(ctx)
	require.NoError(t, err)
	customImageKey, err := f.client.APIKey.Create().SetUserID(f.ownerID).SetName("custom image").SetKey("sk-custom-image-default").SetGroupID(customImageGroup.ID).Save(ctx)
	require.NoError(t, err)
	videoKey, err := f.client.APIKey.Create().SetUserID(f.ownerID).SetName("custom video").SetKey("sk-custom-video-default").SetGroupID(customVideoGroup.ID).Save(ctx)
	require.NoError(t, err)
	customAudioKey, err := f.client.APIKey.Create().SetUserID(f.ownerID).SetName("custom audio").SetKey("sk-custom-audio-default").SetGroupID(customAudioGroup.ID).Save(ctx)
	require.NoError(t, err)
	_, err = f.client.ExecContext(ctx, "INSERT INTO user_default_api_keys (user_id, purpose, api_key_id) VALUES ($1, 'text', $2), ($1, 'image', $3)", f.ownerID, oldKey.ID, imageKey.ID)
	require.NoError(t, err)
	expectedImageDefaultID := imageKey.ID
	readDefaultID := func() int64 {
		w := f.request(http.MethodGet, "/api/v1/keys/defaults", f.token, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		var body struct {
			Data []struct {
				Purpose string      `json:"purpose"`
				APIKey  *dto.APIKey `json:"api_key"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Len(t, body.Data, 4)
		require.Equal(t, expectedImageDefaultID, body.Data[1].APIKey.ID)
		require.Equal(t, "text", body.Data[0].Purpose)
		if body.Data[0].APIKey == nil {
			return 0
		}
		return body.Data[0].APIKey.ID
	}
	require.Equal(t, oldKey.ID, readDefaultID())
	w := f.request(http.MethodGet, "/api/v1/keys/defaults/video", f.token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var videoBody struct {
		Data struct {
			Purpose string      `json:"purpose"`
			APIKey  *dto.APIKey `json:"api_key"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &videoBody))
	require.Equal(t, "video", videoBody.Data.Purpose)
	require.NotNil(t, videoBody.Data.APIKey)
	require.Equal(t, "OC--Seedance【视频】", videoBody.Data.APIKey.Name)
	w = f.request(http.MethodGet, "/api/v1/keys/defaults/unknown", f.token, "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "INVALID_DEFAULT_API_KEY_PURPOSE")

	body := fmt.Sprintf(`{"api_key_id":%d,"user_id":%d}`, newKey.ID, f.otherID)
	for _, token := range []string{"", "sk-model-key", f.otherToken} {
		w = f.request(http.MethodPut, "/api/v1/keys/defaults/text", token, body)
		if token == f.otherToken {
			require.Equal(t, 404, w.Code, w.Body.String())
		} else {
			require.Equal(t, 401, w.Code)
		}
	}
	for _, tc := range []struct {
		purpose, body string
		status        int
	}{
		{"unknown", body, 400},
		{"text", `{}`, 400},
		{"text", `{"api_key_id":0}`, 400},
		{"text", `{"api_key_id":-1}`, 400},
		{"text", `{"api_key_id":9999999}`, 404},
		{"text", fmt.Sprintf(`{"api_key_id":%d}`, imageKey.ID), 400},
	} {
		w := f.request(http.MethodPut, "/api/v1/keys/defaults/"+tc.purpose, f.token, tc.body)
		require.Equal(t, tc.status, w.Code, w.Body.String())
	}
	require.Equal(t, oldKey.ID, readDefaultID())
	for i := 0; i < 2; i++ {
		w := f.request(http.MethodPut, "/api/v1/keys/defaults/text", f.token, body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		require.Contains(t, w.Body.String(), "sk-new-default")
		require.Equal(t, newKey.ID, readDefaultID())
	}
	w = f.request(http.MethodPut, "/api/v1/keys/defaults/text", f.token, fmt.Sprintf(`{"api_key_id":%d}`, customTextKey.ID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "sk-custom-text-default")
	w = f.request(http.MethodPut, "/api/v1/keys/defaults/image", f.token, fmt.Sprintf(`{"api_key_id":%d}`, customImageKey.ID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "sk-custom-image-default")
	expectedImageDefaultID = customImageKey.ID
	w = f.request(http.MethodPut, "/api/v1/keys/defaults/audio", f.token, fmt.Sprintf(`{"api_key_id":%d}`, customAudioKey.ID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "sk-custom-audio-default")
	w = f.request(http.MethodPut, "/api/v1/keys/defaults/audio", f.token, fmt.Sprintf(`{"api_key_id":%d}`, customTextKey.ID))
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "DEFAULT_API_KEY_GROUP_MISMATCH")
	w = f.request(http.MethodPut, "/api/v1/keys/defaults/video", f.token, fmt.Sprintf(`{"api_key_id":%d}`, videoKey.ID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "sk-custom-video-default")
	w = f.request(http.MethodPut, "/api/v1/keys/defaults/video", f.token, fmt.Sprintf(`{"api_key_id":%d}`, imageKey.ID))
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "DEFAULT_API_KEY_GROUP_MISMATCH")
	seedanceKey, err := f.client.APIKey.Query().Where(apikey.GroupIDEQ(videoGroup.ID)).Only(ctx)
	require.NoError(t, err)
	require.NotEqual(t, seedanceKey.ID, videoKey.ID)

	old, err := f.client.APIKey.Get(ctx, oldKey.ID)
	require.NoError(t, err)
	require.Equal(t, "sk-old-default", old.Key)
	require.Equal(t, "active", old.Status)
	require.Nil(t, old.DeletedAt)
	current, err := f.client.APIKey.Get(ctx, newKey.ID)
	require.NoError(t, err)
	require.Equal(t, float64(50), current.Quota)
	require.Equal(t, float64(4), current.QuotaUsed)
	// A deleted default slot can be explicitly reassigned to another owned key.
	require.NoError(t, f.client.APIKey.DeleteOneID(customTextKey.ID).Exec(ctx))
	require.Zero(t, readDefaultID())
	w = f.request(http.MethodPut, "/api/v1/keys/defaults/text", f.token, fmt.Sprintf(`{"api_key_id":%d}`, oldKey.ID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, oldKey.ID, readDefaultID())
}
