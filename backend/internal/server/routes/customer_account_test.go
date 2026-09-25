package routes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type customerAccountFixture struct {
	router     *gin.Engine
	client     *dbent.Client
	ownerID    int64
	otherID    int64
	token      string
	otherToken string
}

func newCustomerAccountFixture(t *testing.T) *customerAccountFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })
	// This SQL-owned table is used by JWT profile loading and is not in the Ent schema.
	_, err = db.Exec(`CREATE TABLE user_avatars (
		user_id INTEGER PRIMARY KEY,
		storage_provider TEXT NOT NULL DEFAULT 'database',
		storage_key TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		content_type TEXT NOT NULL DEFAULT '',
		byte_size INTEGER NOT NULL DEFAULT 0,
		sha256 TEXT NOT NULL DEFAULT ''
	)`)
	require.NoError(t, err)
	users := repository.NewUserRepository(client, db)
	codes := repository.NewRedeemCodeRepository(client)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "customer-account-test-secret", ExpireHour: 1}}
	auth := service.NewAuthService(client, users, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	userService := service.NewUserService(users, nil, nil, nil)
	redeemService := service.NewRedeemService(codes, users, nil, nil, nil, client, nil, nil)
	keyService := service.NewAPIKeyService(repository.NewAPIKeyRepository(client, db), users, repository.NewGroupRepository(client, db), nil, nil, nil, cfg)
	router := gin.New()
	RegisterUserRoutes(router.Group("/api/v1"), &handler.Handlers{
		User:   handler.NewUserHandler(userService, auth, nil, nil, nil, nil),
		Redeem: handler.NewRedeemHandler(redeemService),
		APIKey: handler.NewAPIKeyHandler(keyService),
	}, middleware.NewJWTAuthMiddleware(auth, userService, nil, nil), middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil)
	f := &customerAccountFixture{router: router, client: client}
	for i, email := range []string{"owner@example.com", "other@example.com"} {
		u, err := client.User.Create().SetEmail(email).SetPasswordHash("hash").SetStatus(service.StatusActive).SetBalance(12).Save(ctx)
		require.NoError(t, err)
		_, err = client.ExecContext(ctx, "UPDATE users SET display_balance = ?, frozen_balance = ? WHERE id = ?", 18, 3, u.ID)
		require.NoError(t, err)
		user, err := users.GetByID(ctx, u.ID)
		require.NoError(t, err)
		token, err := auth.GenerateToken(ctx, user)
		require.NoError(t, err)
		if i == 0 {
			f.ownerID, f.token = u.ID, token
		} else {
			f.otherID, f.otherToken = u.ID, token
		}
	}
	expiredAt := time.Now().Add(-time.Hour)
	for _, code := range []service.RedeemCode{
		{Code: "BALANCE-12", Type: service.RedeemTypeBalance, Value: 12, Status: service.StatusUnused, Notes: "private operator note"},
		{Code: "INVITE", Type: service.RedeemTypeInvitation, Status: service.StatusUnused},
		{Code: "EXPIRED", Type: service.RedeemTypeBalance, Value: 12, Status: service.StatusUnused, ExpiresAt: &expiredAt},
	} {
		require.NoError(t, codes.Create(ctx, &code))
	}
	return f
}

func (f *customerAccountFixture) request(method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func assertCustomerBalance(t *testing.T, w *httptest.ResponseRecorder, userID int64, display float64) {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, map[string]any{"user_id": float64(userID), "display_balance": display}, body.Data)
	require.NotContains(t, w.Body.String(), "password")
	require.NotContains(t, w.Body.String(), "email")
}

func TestCustomerAccountRedeemAndBalance(t *testing.T) {
	f := newCustomerAccountFixture(t)
	path := fmt.Sprintf("/api/v1/user/balance?user_id=%d", f.otherID)
	assertCustomerBalance(t, f.request(http.MethodGet, path, f.token, ""), f.ownerID, 18)
	w := f.request(http.MethodPost, "/api/v1/redeem", f.token, fmt.Sprintf(`{"code":"  BALANCE-12  ","user_id":%d}`, f.otherID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.NotContains(t, w.Body.String(), "private operator note")
	var redeemed struct {
		Data struct {
			Status string `json:"status"`
			UsedBy int64  `json:"used_by"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &redeemed))
	require.Equal(t, "used", redeemed.Data.Status)
	require.Equal(t, f.ownerID, redeemed.Data.UsedBy)
	assertCustomerBalance(t, f.request(http.MethodGet, path, f.token, ""), f.ownerID, 36)
	assertCustomerBalance(t, f.request(http.MethodGet, path, f.otherToken, ""), f.otherID, 18)
	owner, err := f.client.User.Get(context.Background(), f.ownerID)
	require.NoError(t, err)
	require.Equal(t, float64(24), owner.Balance)
	for _, token := range []string{f.token, f.otherToken} {
		w = f.request(http.MethodPost, "/api/v1/redeem", token, `{"code":"BALANCE-12"}`)
		require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), "REDEEM_CODE_USED")
	}
	assertCustomerBalance(t, f.request(http.MethodGet, path, f.token, ""), f.ownerID, 36)
	w = f.request(http.MethodGet, "/api/v1/redeem/history", f.token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "BALANCE-12")
	w = f.request(http.MethodGet, "/api/v1/redeem/history", f.otherToken, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "BALANCE-12")
}

func TestCustomerAccountAuthenticationAndRedeemErrors(t *testing.T) {
	f := newCustomerAccountFixture(t)
	for _, token := range []string{"", "sk-model-key", "invalid-jwt"} {
		for _, path := range []string{"/api/v1/user/balance", "/api/v1/redeem/history", "/api/v1/redeem"} {
			method := http.MethodGet
			if path == "/api/v1/redeem" {
				method = http.MethodPost
			}
			w := f.request(method, path, token, `{"code":"BALANCE-12"}`)
			require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		body   string
		status int
		reason string
	}{
		{`{}`, 400, ""},
		{`{"code":"  "}`, 400, ""},
		{`{"code":12}`, 400, ""},
		{`{"code":"MISSING"}`, 404, "REDEEM_CODE_NOT_FOUND"},
		{`{"code":"EXPIRED"}`, 409, "REDEEM_CODE_EXPIRED"},
		{`{"code":"INVITE"}`, 400, "REDEEM_CODE_UNSUPPORTED_TYPE"},
	} {
		w := f.request(http.MethodPost, "/api/v1/redeem", f.token, tc.body)
		require.Equal(t, tc.status, w.Code, w.Body.String())
		if tc.reason != "" {
			require.Contains(t, w.Body.String(), tc.reason)
		}
	}
	assertCustomerBalance(t, f.request(http.MethodGet, "/api/v1/user/balance", f.token, ""), f.ownerID, 18)
	_, err := f.client.ExecContext(context.Background(), "UPDATE users SET balance = ?, display_balance = ?, frozen_balance = ? WHERE id = ?", -0.25, 0, 0, f.ownerID)
	require.NoError(t, err)
	assertCustomerBalance(t, f.request(http.MethodGet, "/api/v1/user/balance", f.token, ""), f.ownerID, 0)
}
