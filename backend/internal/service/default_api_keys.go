package service

import (
	"context"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var defaultAPIKeyGroups = [...]struct {
	Purpose string
	Name    string
}{
	{"text", "OC--ChatGPT【文本模型】"},
	{"image", "OC--ChatGPT【生图】"},
	{"video", "OC--Seedance【视频】"},
	{"audio", "OC--Qwen-TTS【音频】"},
}

// A nil APIKey preserves the default slot after its credential has been deleted.
type DefaultAPIKey struct {
	Purpose string
	APIKey  *APIKey
}

type DefaultAPIKeyRepository interface {
	ListDefaultAPIKeys(ctx context.Context, userID int64) ([]DefaultAPIKey, error)
	CreateDefaultAPIKeys(ctx context.Context, userID int64, keys []DefaultAPIKey) error
}

type DefaultAPIKeyUpdater interface {
	SetDefaultAPIKey(ctx context.Context, userID int64, purpose string, apiKeyID, groupID int64) (*APIKey, error)
}

var (
	ErrDefaultAPIKeyPurpose       = infraerrors.BadRequest("INVALID_DEFAULT_API_KEY_PURPOSE", "purpose must be text, image, video or audio")
	ErrDefaultAPIKeyGroupMismatch = infraerrors.BadRequest("DEFAULT_API_KEY_GROUP_MISMATCH", "API key must belong to the group configured for this default purpose")
	ErrDefaultAPIKeyUnavailable   = infraerrors.BadRequest("DEFAULT_API_KEY_UNAVAILABLE", "default API key must be active, unexpired and have available quota")
)

// UpdateDefaultAPIKey changes the default association, leaving both credentials intact.
func (s *APIKeyService) UpdateDefaultAPIKey(ctx context.Context, userID int64, purpose string, apiKeyID int64) (*DefaultAPIKey, error) {
	groupSpec, ok := defaultAPIKeyGroupSpec(purpose)
	if !ok {
		return nil, ErrDefaultAPIKeyPurpose
	}
	if apiKeyID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_API_KEY_ID", "api_key_id must be positive")
	}
	repo, ok := s.apiKeyRepo.(DefaultAPIKeyUpdater)
	if !ok {
		return nil, ErrServiceUnavailable
	}
	key, err := s.apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil, err
	}
	if key.UserID != userID {
		return nil, ErrAPIKeyNotFound
	}
	if key.GroupID == nil {
		return nil, ErrDefaultAPIKeyGroupMismatch
	}
	group, err := s.groupRepo.GetByID(ctx, *key.GroupID)
	if err != nil {
		return nil, err
	}
	if !defaultAPIKeyGroupMatchesPurpose(group, groupSpec) {
		return nil, ErrDefaultAPIKeyGroupMismatch
	}
	if !group.IsActive() {
		return nil, infraerrors.ServiceUnavailable("DEFAULT_API_KEY_GROUP_UNAVAILABLE", "default API key group is not active")
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive() {
		return nil, ErrUserNotActive
	}
	if !s.canUserBindGroup(ctx, user, group) {
		return nil, ErrGroupNotAllowed
	}
	if !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		return nil, ErrDefaultAPIKeyUnavailable
	}
	key, err = repo.SetDefaultAPIKey(ctx, userID, purpose, apiKeyID, group.ID)
	if err != nil {
		return nil, err
	}
	return &DefaultAPIKey{Purpose: purpose, APIKey: key}, nil
}

func (s *APIKeyService) GetDefaultAPIKey(ctx context.Context, userID int64, purpose string) (*DefaultAPIKey, error) {
	if _, ok := defaultAPIKeyGroupName(purpose); !ok {
		return nil, ErrDefaultAPIKeyPurpose
	}
	keys, err := s.EnsureDefaultAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, item := range keys {
		if item.Purpose == purpose {
			return &DefaultAPIKey{Purpose: item.Purpose, APIKey: item.APIKey}, nil
		}
	}
	return nil, ErrDefaultAPIKeyPurpose
}

func defaultAPIKeyGroupName(purpose string) (string, bool) {
	spec, ok := defaultAPIKeyGroupSpec(purpose)
	return spec.Name, ok
}

func defaultAPIKeyGroupSpec(purpose string) (struct {
	Purpose string
	Name    string
}, bool) {
	for _, spec := range defaultAPIKeyGroups {
		if spec.Purpose == purpose {
			return spec, true
		}
	}
	return struct {
		Purpose string
		Name    string
	}{}, false
}

func defaultAPIKeyGroupMatchesPurpose(group *Group, spec struct {
	Purpose string
	Name    string
}) bool {
	if group == nil {
		return false
	}
	for _, known := range defaultAPIKeyGroups {
		if group.Name == known.Name {
			return known.Purpose == spec.Purpose
		}
	}
	switch spec.Purpose {
	case "text":
		return isDefaultTextAPIKeyGroup(group)
	case "image":
		return group.AllowImageGeneration
	case "video":
		return IsPPVideoPlatform(group.Platform)
	case "audio":
		return IsAudioPlatform(group.Platform)
	default:
		return false
	}
}

func isDefaultTextAPIKeyGroup(group *Group) bool {
	if group.AllowImageGeneration || IsPPVideoPlatform(group.Platform) || IsAudioPlatform(group.Platform) {
		return false
	}
	switch group.Platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformJimeng, PlatformDoubao, PlatformQwen, PlatformKimi, PlatformDeepSeek:
		return true
	default:
		return false
	}
}

type signupDefaultAPIKeyProvisioner interface {
	EnsureDefaultAPIKeys(ctx context.Context, userID int64) ([]DefaultAPIKey, error)
}

func (s *APIKeyService) GetDefaultAPIKeys(ctx context.Context, userID int64) ([]DefaultAPIKey, error) {
	repo, ok := s.apiKeyRepo.(DefaultAPIKeyRepository)
	if !ok {
		return nil, ErrServiceUnavailable
	}
	return repo.ListDefaultAPIKeys(ctx, userID)
}

// EnsureDefaultAPIKeys only fills slots that have never been provisioned.
// Disabled, expired or deleted credentials are never reactivated by a retry.
func (s *APIKeyService) EnsureDefaultAPIKeys(ctx context.Context, userID int64) ([]DefaultAPIKey, error) {
	existing, err := s.GetDefaultAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(existing))
	for _, item := range existing {
		seen[item.Purpose] = true
	}
	if len(seen) == len(defaultAPIKeyGroups) {
		return existing, nil
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive() {
		return nil, ErrUserNotActive
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string][]Group, len(groups))
	for _, group := range groups {
		byName[group.Name] = append(byName[group.Name], group)
	}
	keys := make([]DefaultAPIKey, 0, len(defaultAPIKeyGroups))
	for _, spec := range defaultAPIKeyGroups {
		if seen[spec.Purpose] {
			continue
		}
		matches := byName[spec.Name]
		if len(matches) != 1 {
			return nil, infraerrors.ServiceUnavailable("DEFAULT_API_KEY_GROUP_UNAVAILABLE", fmt.Sprintf("default API key group must exist and be active: %s", spec.Name))
		}
		group := matches[0]
		if !s.canUserBindGroup(ctx, user, &group) {
			return nil, infraerrors.Forbidden("DEFAULT_API_KEY_GROUP_NOT_ALLOWED", fmt.Sprintf("user cannot bind default API key group: %s", spec.Name))
		}
		key, err := s.GenerateKey()
		if err != nil {
			return nil, err
		}
		keys = append(keys, DefaultAPIKey{Purpose: spec.Purpose, APIKey: &APIKey{
			UserID: userID, Key: key, Name: spec.Name, GroupID: &group.ID, Status: StatusActive,
		}})
	}
	if err := s.apiKeyRepo.(DefaultAPIKeyRepository).CreateDefaultAPIKeys(ctx, userID, keys); err != nil {
		return nil, err
	}
	return s.GetDefaultAPIKeys(ctx, userID)
}

func (s *AuthService) provisionSignupAPIKeys(ctx context.Context, userID int64) {
	if s.defaultAPIKeys == nil {
		return
	}
	if _, err := s.defaultAPIKeys.EnsureDefaultAPIKeys(ctx, userID); err != nil {
		// The account already exists; preserve login and allow an explicit retry.
		logger.LegacyPrintf("service.auth", "[Auth] Failed to provision default API keys: user_id=%d err=%v", userID, err)
	}
}
