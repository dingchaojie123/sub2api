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
	{"text", "OC--ChatGPT【文本模型，官key】"},
	{"image", "OC--ChatGPT【生图】"},
	{"video", "视频【聚合】"},
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

type AvailableGroupAPIKeyProvisioner interface {
	EnsureAPIKeysForGroups(ctx context.Context, userID int64, keys []APIKey) error
}

var (
	ErrDefaultAPIKeyPurpose       = infraerrors.BadRequest("INVALID_DEFAULT_API_KEY_PURPOSE", "purpose must be text, image, video or audio")
	ErrDefaultAPIKeyGroupMismatch = infraerrors.BadRequest("DEFAULT_API_KEY_GROUP_MISMATCH", "API key must belong to the group configured for this default purpose")
	ErrDefaultAPIKeyUnavailable   = infraerrors.BadRequest("DEFAULT_API_KEY_UNAVAILABLE", "default API key must be active, unexpired and have available quota")
	ErrDefaultAPIKeyBillingSource = infraerrors.BadRequest("DEFAULT_API_KEY_BILLING_SOURCE_UNAVAILABLE", "default API key billing source is not active")
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
	keys := []APIKey{*key}
	if err := s.fillBillingSources(ctx, keys); err != nil {
		return nil, err
	}
	if keys[0].BillingSource.Type == APIKeyBillingSourceOrganization && keys[0].BillingSource.Status != APIKeyBillingSourceActive {
		return nil, ErrDefaultAPIKeyBillingSource
	}
	key, err = repo.SetDefaultAPIKey(ctx, userID, purpose, apiKeyID, group.ID)
	if err != nil {
		return nil, err
	}
	key.BillingSource = keys[0].BillingSource
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
		// Jimeng serves video through its own gateway, separate from PP video.
		return group.Platform == PlatformJimeng || IsPPVideoPlatform(group.Platform)
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
	items, err := repo.ListDefaultAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	keys := make([]APIKey, 0, len(items))
	indexes := make([]int, 0, len(items))
	for i := range items {
		if items[i].APIKey != nil {
			keys = append(keys, *items[i].APIKey)
			indexes = append(indexes, i)
		}
	}
	if err := s.fillBillingSources(ctx, keys); err != nil {
		return nil, err
	}
	for i := range keys {
		items[indexes[i]].APIKey.BillingSource = keys[i].BillingSource
	}
	return items, nil
}

// EnsureDefaultAPIKeys only fills slots that have never been provisioned.
// Disabled, expired or deleted credentials are never reactivated by a retry.
func (s *APIKeyService) EnsureDefaultAPIKeys(ctx context.Context, userID int64) ([]DefaultAPIKey, error) {
	existing, err := s.GetDefaultAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(existing))
	usedGroups := make(map[int64]bool, len(existing))
	for _, item := range existing {
		seen[item.Purpose] = true
		if item.APIKey != nil && item.APIKey.GroupID != nil {
			usedGroups[*item.APIKey.GroupID] = true
		}
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
	availableGroups := make([]Group, 0, len(groups))
	for i := range groups {
		if s.canUserBindGroup(ctx, user, &groups[i]) {
			availableGroups = append(availableGroups, groups[i])
		}
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
		group, ok := selectDefaultAPIKeyGroup(ctx, s, user, groups, byName, spec, usedGroups)
		if !ok {
			logger.LegacyPrintf("service.api_key", "[APIKey] Skipping unavailable default group: user_id=%d purpose=%s preferred_group=%s", userID, spec.Purpose, spec.Name)
			continue
		}
		key, err := s.GenerateKey()
		if err != nil {
			return nil, err
		}
		keys = append(keys, DefaultAPIKey{Purpose: spec.Purpose, APIKey: &APIKey{
			UserID: userID, Key: key, Name: group.Name, GroupID: &group.ID, Status: StatusActive,
		}})
		usedGroups[group.ID] = true
	}
	if len(keys) > 0 {
		if err := s.apiKeyRepo.(DefaultAPIKeyRepository).CreateDefaultAPIKeys(ctx, userID, keys); err != nil {
			return nil, err
		}
	}
	if err := s.ensureAvailableGroupAPIKeys(ctx, userID, availableGroups); err != nil {
		return nil, err
	}
	if len(keys) == 0 && len(seen) < len(defaultAPIKeyGroups) {
		return nil, infraerrors.ServiceUnavailable("DEFAULT_API_KEY_GROUP_UNAVAILABLE", fmt.Sprintf("no default API key groups are available for user: %d", userID))
	}
	return s.GetDefaultAPIKeys(ctx, userID)
}

func (s *APIKeyService) ensureAvailableGroupAPIKeys(ctx context.Context, userID int64, groups []Group) error {
	provisioner, ok := s.apiKeyRepo.(AvailableGroupAPIKeyProvisioner)
	if !ok {
		return nil
	}
	keys := make([]APIKey, 0, len(groups))
	for _, group := range groups {
		key, err := s.GenerateKey()
		if err != nil {
			return err
		}
		groupID := group.ID
		keys = append(keys, APIKey{
			UserID: userID,
			Key:    key,
			Name:   group.Name,
			GroupID: &groupID,
			Status: StatusActive,
		})
	}
	return provisioner.EnsureAPIKeysForGroups(ctx, userID, keys)
}

func selectDefaultAPIKeyGroup(
	ctx context.Context,
	s *APIKeyService,
	user *User,
	groups []Group,
	byName map[string][]Group,
	spec struct {
		Purpose string
		Name    string
	},
	usedGroups map[int64]bool,
) (Group, bool) {
	// Keep the configured names as the primary mapping for stable deployments.
	if matches := byName[spec.Name]; len(matches) > 1 {
		// Duplicate reserved names are ambiguous; require an admin to fix them.
		return Group{}, false
	} else if len(matches) == 1 {
		group := matches[0]
		if !usedGroups[group.ID] && s.canUserBindGroup(ctx, user, &group) {
			return group, true
		}
	}
	// Fall back to a group with the requested capability, while keeping one
	// group per default slot. The configured group remains the first choice.
	for _, group := range groups {
		if usedGroups[group.ID] || !defaultAPIKeyGroupMatchesPurpose(&group, spec) {
			continue
		}
		if s.canUserBindGroup(ctx, user, &group) {
			return group, true
		}
	}
	return Group{}, false
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
