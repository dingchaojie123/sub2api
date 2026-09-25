//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type signupKeysStub struct {
	users []int64
	err   error
}

func (s *signupKeysStub) EnsureDefaultAPIKeys(_ context.Context, userID int64) ([]DefaultAPIKey, error) {
	s.users = append(s.users, userID)
	return nil, s.err
}

func TestDefaultAPIKeysSignupHooks(t *testing.T) {
	for _, flow := range []string{"email", "oauth", "oauth_pair", "oauth_finalize"} {
		t.Run(flow, func(t *testing.T) {
			repo := &userRepoStub{nextID: 51}
			svc := newAuthService(repo, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil, nil)
			keys := &signupKeysStub{}
			svc.defaultAPIKeys = keys
			svc.refreshTokenCache = &refreshTokenCacheStub{}
			ctx := context.Background()
			switch flow {
			case "email":
				_, _, err := svc.Register(ctx, "default@example.com", "password")
				require.NoError(t, err)
			case "oauth":
				_, _, err := svc.LoginOrRegisterOAuth(ctx, "default@example.com", "default")
				require.NoError(t, err)
			case "oauth_pair":
				_, _, err := svc.LoginOrRegisterOAuthWithTokenPair(ctx, "default@example.com", "default", "", "", "linuxdo")
				require.NoError(t, err)
			case "oauth_finalize":
				err := svc.FinalizeOAuthEmailAccount(ctx, &User{ID: 51}, "", "linuxdo", "")
				require.NoError(t, err)
			}
			require.Equal(t, []int64{51}, keys.users)
		})
	}
}

func TestDefaultAPIKeysProvisionFailurePreservesRegistration(t *testing.T) {
	svc := newAuthService(&userRepoStub{nextID: 51}, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil, nil)
	svc.defaultAPIKeys = &signupKeysStub{err: errors.New("group unavailable")}
	token, user, err := svc.Register(context.Background(), "default@example.com", "password")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Equal(t, int64(51), user.ID)
}

func TestDefaultAPIKeysExistingOAuthLoginDoesNotProvision(t *testing.T) {
	svc := newAuthService(&userRepoStub{user: &User{ID: 51, Email: "existing@example.com", Status: StatusActive}}, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil, nil)
	keys := &signupKeysStub{}
	svc.defaultAPIKeys = keys
	_, _, err := svc.LoginOrRegisterOAuth(context.Background(), "existing@example.com", "")
	require.NoError(t, err)
	require.Empty(t, keys.users)
}
