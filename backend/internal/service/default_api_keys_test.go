package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type defaultKeysRepoStub struct {
	APIKeyRepository
	items     []DefaultAPIKey
	writes    int
	err       error
	candidate *APIKey
}

func (r *defaultKeysRepoStub) ListDefaultAPIKeys(context.Context, int64) ([]DefaultAPIKey, error) {
	return r.items, r.err
}

func (r *defaultKeysRepoStub) CreateDefaultAPIKeys(_ context.Context, _ int64, keys []DefaultAPIKey) error {
	r.writes++
	r.items = append(r.items, keys...)
	return nil
}

func (r *defaultKeysRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	return r.candidate, r.err
}

func (r *defaultKeysRepoStub) SetDefaultAPIKey(_ context.Context, _ int64, purpose string, _, _ int64) (*APIKey, error) {
	r.writes++
	for i := range r.items {
		if r.items[i].Purpose == purpose {
			r.items[i].APIKey = r.candidate
			return r.candidate, nil
		}
	}
	r.items = append(r.items, DefaultAPIKey{Purpose: purpose, APIKey: r.candidate})
	return r.candidate, nil
}

type defaultKeysUserRepo struct {
	UserRepository
	user *User
}

func (r *defaultKeysUserRepo) GetByID(context.Context, int64) (*User, error) { return r.user, nil }

type defaultKeysGroupRepo struct {
	GroupRepository
	groups []Group
}

func (r *defaultKeysGroupRepo) ListActive(context.Context) ([]Group, error) { return r.groups, nil }

func (r *defaultKeysGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	for i := range r.groups {
		if r.groups[i].ID == id {
			return &r.groups[i], nil
		}
	}
	return nil, ErrGroupNotFound
}

type defaultKeysSubscriptionRepo struct {
	UserSubscriptionRepository
	err error
}

func (r *defaultKeysSubscriptionRepo) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return &UserSubscription{}, r.err
}

func newDefaultKeysTestService() (*APIKeyService, *defaultKeysRepoStub, *defaultKeysGroupRepo) {
	repo := &defaultKeysRepoStub{}
	groups := &defaultKeysGroupRepo{}
	for i, spec := range defaultAPIKeyGroups {
		groups.groups = append(groups.groups, Group{ID: int64(i + 1), Name: spec.Name, Status: StatusActive})
	}
	return &APIKeyService{
		apiKeyRepo:  repo,
		userRepo:    &defaultKeysUserRepo{user: &User{ID: 7, Status: StatusActive}},
		groupRepo:   groups,
		userSubRepo: &defaultKeysSubscriptionRepo{err: ErrUserNotFound},
		cfg:         &config.Config{},
	}, repo, groups
}

func TestEnsureDefaultAPIKeysCreatesFourAndPreservesExistingSlots(t *testing.T) {
	svc, repo, _ := newDefaultKeysTestService()
	keys, err := svc.EnsureDefaultAPIKeys(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, keys, 4)
	credentials := map[string]bool{}
	for i, item := range keys {
		require.Equal(t, defaultAPIKeyGroups[i].Purpose, item.Purpose)
		require.Equal(t, defaultAPIKeyGroups[i].Name, item.APIKey.Name)
		require.Equal(t, int64(7), item.APIKey.UserID)
		require.Equal(t, int64(i+1), *item.APIKey.GroupID)
		require.Equal(t, StatusActive, item.APIKey.Status)
		require.Len(t, item.APIKey.Key, 67)
		credentials[item.APIKey.Key] = true
	}
	require.Len(t, credentials, 4)
	repo.items[0].APIKey = nil
	repo.items[1].APIKey.Status = "inactive"
	repo.items[2].APIKey.Name = "renamed"
	keys, err = svc.EnsureDefaultAPIKeys(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 1, repo.writes)
	require.Nil(t, keys[0].APIKey)
	require.Equal(t, "inactive", keys[1].APIKey.Status)
	require.Equal(t, "renamed", keys[2].APIKey.Name)
}

func TestEnsureDefaultAPIKeysValidatesEveryGroupBeforeWriting(t *testing.T) {
	for _, mode := range []string{"missing", "duplicate", "exclusive", "subscription", "repository_error", "inactive_user"} {
		t.Run(mode, func(t *testing.T) {
			svc, repo, groups := newDefaultKeysTestService()
			switch mode {
			case "missing":
				groups.groups = groups.groups[:3]
			case "duplicate":
				groups.groups = append(groups.groups, groups.groups[3])
			case "exclusive":
				groups.groups[3].IsExclusive = true
			case "subscription":
				groups.groups[3].SubscriptionType = SubscriptionTypeSubscription
			case "repository_error":
				repo.err = errors.New("unavailable")
			case "inactive_user":
				svc.userRepo.(*defaultKeysUserRepo).user.Status = "disabled"
			}
			_, err := svc.EnsureDefaultAPIKeys(context.Background(), 7)
			require.Error(t, err)
			require.Zero(t, repo.writes)
		})
	}
}

func TestEnsureDefaultAPIKeysHonorsGroupGrants(t *testing.T) {
	svc, _, groups := newDefaultKeysTestService()
	groups.groups[0].IsExclusive = true
	groups.groups[1].SubscriptionType = SubscriptionTypeSubscription
	svc.userRepo.(*defaultKeysUserRepo).user.AllowedGroups = []int64{1}
	svc.userSubRepo.(*defaultKeysSubscriptionRepo).err = nil
	keys, err := svc.EnsureDefaultAPIKeys(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, keys, 4)
}

func TestEnsureDefaultAPIKeysAddsAudioToExistingDefaults(t *testing.T) {
	svc, repo, groups := newDefaultKeysTestService()
	existingKey := &APIKey{UserID: 7, Key: "sk-existing-text", Name: "renamed", Status: "inactive"}
	repo.items = []DefaultAPIKey{
		{Purpose: "text", APIKey: existingKey},
		{Purpose: "image"},
		{Purpose: "video"},
	}
	// Only the new group needs to remain available when filling a missing slot.
	groups.groups = groups.groups[3:]
	keys, err := svc.EnsureDefaultAPIKeys(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, keys, 4)
	require.Same(t, existingKey, keys[0].APIKey)
	require.Nil(t, keys[1].APIKey)
	require.Nil(t, keys[2].APIKey)
	require.Equal(t, "audio", keys[3].Purpose)
	require.Equal(t, "OC--Qwen-TTS【音频】", keys[3].APIKey.Name)
	require.Equal(t, int64(4), *keys[3].APIKey.GroupID)
	audioKey := keys[3].APIKey.Key
	keys, err = svc.EnsureDefaultAPIKeys(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, audioKey, keys[3].APIKey.Key)
	require.Equal(t, 1, repo.writes)
}

func TestUpdateDefaultAPIKeyValidatesCandidate(t *testing.T) {
	for _, mode := range []string{"purpose", "id", "other_owner", "no_group", "wrong_group", "inactive_group", "exclusive", "subscription", "inactive_user", "inactive_key", "expired", "quota", "missing"} {
		t.Run(mode, func(t *testing.T) {
			svc, repo, groups := newDefaultKeysTestService()
			groupID := int64(1)
			repo.candidate = &APIKey{ID: 11, UserID: 7, GroupID: &groupID, Status: StatusActive}
			purpose, id := "text", int64(11)
			switch mode {
			case "purpose":
				purpose = "unknown"
			case "id":
				id = 0
			case "other_owner":
				repo.candidate.UserID = 8
			case "no_group":
				repo.candidate.GroupID = nil
			case "wrong_group":
				purpose = "audio"
			case "inactive_group":
				groups.groups[0].Status = "inactive"
			case "exclusive":
				groups.groups[0].IsExclusive = true
			case "subscription":
				groups.groups[0].SubscriptionType = SubscriptionTypeSubscription
			case "inactive_user":
				svc.userRepo.(*defaultKeysUserRepo).user.Status = "disabled"
			case "inactive_key":
				repo.candidate.Status = "inactive"
			case "expired":
				expired := time.Now().Add(-time.Minute)
				repo.candidate.ExpiresAt = &expired
			case "quota":
				repo.candidate.Quota, repo.candidate.QuotaUsed = 10, 10
			case "missing":
				repo.err = ErrAPIKeyNotFound
			}
			_, err := svc.UpdateDefaultAPIKey(context.Background(), 7, purpose, id)
			require.Error(t, err)
			require.Zero(t, repo.writes)
			if mode == "other_owner" {
				require.ErrorIs(t, err, ErrAPIKeyNotFound)
			}
		})
	}
}

func TestUpdateDefaultAPIKeyAllowsAnyVideoPlatformGroup(t *testing.T) {
	for _, platform := range []string{PlatformKling, PlatformHappyHourse, PlatformSeedance, PlatformByteDance, PlatformWan3, PlatformMiniMaxH3, PlatformMiniMaxH3CompShare, PlatformPixverseV6, PlatformGrokImagineVideo, PlatformKuaishou} {
		t.Run(platform, func(t *testing.T) {
			svc, repo, groups := newDefaultKeysTestService()
			groupID := int64(99)
			groups.groups = append(groups.groups, Group{ID: groupID, Name: "custom video group " + platform, Platform: platform, Status: StatusActive})
			repo.candidate = &APIKey{ID: 77, UserID: 7, GroupID: &groupID, Key: "sk-video", Status: StatusActive}

			updated, err := svc.UpdateDefaultAPIKey(context.Background(), 7, "video", 77)
			require.NoError(t, err)
			require.Equal(t, "video", updated.Purpose)
			require.Equal(t, repo.candidate, updated.APIKey)
		})
	}
}

func TestUpdateDefaultAPIKeyRejectsNonVideoPlatformForVideoPurpose(t *testing.T) {
	svc, repo, groups := newDefaultKeysTestService()
	groupID := int64(99)
	groups.groups = append(groups.groups, Group{ID: groupID, Name: "custom text group", Platform: PlatformOpenAI, Status: StatusActive})
	repo.candidate = &APIKey{ID: 77, UserID: 7, GroupID: &groupID, Key: "sk-text", Status: StatusActive}

	_, err := svc.UpdateDefaultAPIKey(context.Background(), 7, "video", 77)
	require.ErrorIs(t, err, ErrDefaultAPIKeyGroupMismatch)
	require.Zero(t, repo.writes)
}

func TestUpdateDefaultAPIKeyAllowsPurposeCompatibleGroups(t *testing.T) {
	for _, tc := range []struct {
		name    string
		purpose string
		group   Group
	}{
		{name: "text openai", purpose: "text", group: Group{Name: "Custom Text", Platform: PlatformOpenAI}},
		{name: "text qwen", purpose: "text", group: Group{Name: "Custom Qwen Text", Platform: PlatformQwen}},
		{name: "image enabled", purpose: "image", group: Group{Name: "Custom Image", Platform: PlatformOpenAI, AllowImageGeneration: true}},
		{name: "audio qwen tts", purpose: "audio", group: Group{Name: "Custom Audio", Platform: PlatformQwenTTS}},
		{name: "audio minimax speech", purpose: "audio", group: Group{Name: "Custom MiniMax Speech", Platform: PlatformMiniMaxSpeech}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, groups := newDefaultKeysTestService()
			groupID := int64(99)
			tc.group.ID = groupID
			tc.group.Status = StatusActive
			groups.groups = append(groups.groups, tc.group)
			repo.candidate = &APIKey{ID: 77, UserID: 7, GroupID: &groupID, Key: "sk-compatible", Status: StatusActive}

			updated, err := svc.UpdateDefaultAPIKey(context.Background(), 7, tc.purpose, 77)
			require.NoError(t, err)
			require.Equal(t, tc.purpose, updated.Purpose)
			require.Equal(t, repo.candidate, updated.APIKey)
		})
	}
}

func TestUpdateDefaultAPIKeyRejectsPurposeIncompatibleGroups(t *testing.T) {
	for _, tc := range []struct {
		name    string
		purpose string
		group   Group
	}{
		{name: "video rejects text", purpose: "video", group: Group{Name: "Custom Text", Platform: PlatformOpenAI}},
		{name: "image rejects disabled image group", purpose: "image", group: Group{Name: "Custom Text", Platform: PlatformOpenAI}},
		{name: "audio rejects text", purpose: "audio", group: Group{Name: "Custom Text", Platform: PlatformOpenAI}},
		{name: "text rejects video", purpose: "text", group: Group{Name: "Custom Video", Platform: PlatformKling}},
		{name: "text rejects audio", purpose: "text", group: Group{Name: "Custom Audio", Platform: PlatformQwenTTS}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, groups := newDefaultKeysTestService()
			groupID := int64(99)
			tc.group.ID = groupID
			tc.group.Status = StatusActive
			groups.groups = append(groups.groups, tc.group)
			repo.candidate = &APIKey{ID: 77, UserID: 7, GroupID: &groupID, Key: "sk-incompatible", Status: StatusActive}

			_, err := svc.UpdateDefaultAPIKey(context.Background(), 7, tc.purpose, 77)
			require.ErrorIs(t, err, ErrDefaultAPIKeyGroupMismatch)
			require.Zero(t, repo.writes)
		})
	}
}

func TestUpdateDefaultAPIKeyChangesOnlyAssociation(t *testing.T) {
	svc, repo, _ := newDefaultKeysTestService()
	keys, err := svc.EnsureDefaultAPIKeys(context.Background(), 7)
	require.NoError(t, err)
	old := *keys[0].APIKey
	groupID := int64(1)
	repo.candidate = &APIKey{ID: 99, UserID: 7, GroupID: &groupID, Key: "sk-existing-new-key", Status: StatusActive, Quota: 20, QuotaUsed: 3}
	updated, err := svc.UpdateDefaultAPIKey(context.Background(), 7, "text", 99)
	require.NoError(t, err)
	require.Equal(t, "text", updated.Purpose)
	require.Equal(t, repo.candidate, updated.APIKey)
	current, err := svc.GetDefaultAPIKeys(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "sk-existing-new-key", current[0].APIKey.Key)
	require.NotEqual(t, old.Key, current[0].APIKey.Key)
	for i := 1; i < 4; i++ {
		require.Equal(t, defaultAPIKeyGroups[i].Purpose, current[i].Purpose)
	}
	current, err = svc.EnsureDefaultAPIKeys(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, int64(99), current[0].APIKey.ID)
	require.Equal(t, 2, repo.writes)
}
