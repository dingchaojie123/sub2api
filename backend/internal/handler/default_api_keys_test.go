package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type defaultKeysHandlerRepo struct {
	service.APIKeyRepository
	userIDs []int64
}

func (r *defaultKeysHandlerRepo) ListDefaultAPIKeys(_ context.Context, userID int64) ([]service.DefaultAPIKey, error) {
	r.userIDs = append(r.userIDs, userID)
	return []service.DefaultAPIKey{
		{Purpose: "text", APIKey: &service.APIKey{ID: 1, UserID: userID, Key: "sk-owned-secret", Status: service.StatusActive}},
		{Purpose: "image"},
		{Purpose: "video"},
		{Purpose: "audio", APIKey: &service.APIKey{ID: 4, UserID: userID, Key: "sk-owned-audio", Name: "OC--Qwen-TTS【音频】", Status: service.StatusActive}},
	}, nil
}

func (r *defaultKeysHandlerRepo) CreateDefaultAPIKeys(context.Context, int64, []service.DefaultAPIKey) error {
	panic("existing default slots must not be recreated")
}

type defaultKeysHandlerUserRepo struct{ service.UserRepository }

func (r *defaultKeysHandlerUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	now := time.Now()
	return &service.User{ID: id, Status: service.StatusActive, Role: service.RoleUser, LastActiveAt: &now, TokenVersionResolved: true}, nil
}

func (r *defaultKeysHandlerUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

func TestDefaultAPIKeysJWTEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "default-keys-test-secret", ExpireHour: 1}}
	users := &defaultKeysHandlerUserRepo{}
	auth := service.NewAuthService(nil, users, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	userService := service.NewUserService(users, nil, nil, nil)
	repo := &defaultKeysHandlerRepo{}
	keyService := service.NewAPIKeyService(repo, users, nil, nil, nil, nil, cfg)
	h := NewAPIKeyHandler(keyService)
	router := gin.New()
	secured := router.Group("/api/v1/keys", gin.HandlerFunc(middleware.NewJWTAuthMiddleware(auth, userService, nil, nil)))
	secured.GET("/defaults", h.GetDefaults)
	secured.GET("/defaults/:purpose", h.GetDefault)
	secured.POST("/defaults", h.EnsureDefaults)
	owner, err := users.GetByID(context.Background(), 7)
	require.NoError(t, err)
	token, err := auth.GenerateToken(context.Background(), owner)
	require.NoError(t, err)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		for _, authorization := range []string{"", "Bearer sk-not-a-login-token", "Bearer " + token} {
			t.Run(method+authorization[:min(6, len(authorization))], func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(method, "/api/v1/keys/defaults?user_id=999", nil)
				request.Header.Set("Authorization", authorization)
				router.ServeHTTP(recorder, request)
				if authorization != "Bearer "+token {
					require.Equal(t, http.StatusUnauthorized, recorder.Code)
					require.NotContains(t, recorder.Body.String(), "sk-owned-secret")
					return
				}
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
				var body struct {
					Data []defaultAPIKeyResponse `json:"data"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
				require.Len(t, body.Data, 4)
				require.Equal(t, int64(7), body.Data[0].APIKey.UserID)
				require.Equal(t, "sk-owned-secret", body.Data[0].APIKey.Key)
				require.Nil(t, body.Data[1].APIKey)
				require.Equal(t, "audio", body.Data[3].Purpose)
				require.Equal(t, "sk-owned-audio", body.Data[3].APIKey.Key)
			})
		}
	}
	require.Equal(t, []int64{7, 7}, repo.userIDs)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/keys/defaults/audio?user_id=999", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	var single struct {
		Data defaultAPIKeyResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &single))
	require.Equal(t, "audio", single.Data.Purpose)
	require.Equal(t, int64(7), single.Data.APIKey.UserID)
	require.Equal(t, "sk-owned-audio", single.Data.APIKey.Key)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/keys/defaults/unknown", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "INVALID_DEFAULT_API_KEY_PURPOSE")
}
