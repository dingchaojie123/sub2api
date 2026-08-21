package service

import "testing"

func TestProviderPlatformsAreOpenAICompatibleAndQuotaEnabled(t *testing.T) {
	platforms := []string{
		PlatformDoubao,
		PlatformQwen,
		PlatformKimi,
		PlatformDeepSeek,
	}

	for _, platform := range platforms {
		if !IsOpenAICompatiblePlatform(platform) {
			t.Errorf("platform %q is not classified as OpenAI-compatible", platform)
		}
		if !IsAllowedQuotaPlatform(platform) {
			t.Errorf("platform %q is not allowed for user quotas", platform)
		}
	}
}

func TestVideoPlatformsAreAllowedQuotaPlatforms(t *testing.T) {
	platforms := []string{
		PlatformKling,
		PlatformHappyHourse,
		PlatformSeedance,
	}

	for _, platform := range platforms {
		if !IsAllowedQuotaPlatform(platform) {
			t.Errorf("video platform %q is not allowed for user quotas", platform)
		}
		if IsOpenAICompatiblePlatform(platform) {
			t.Errorf("video platform %q must not be OpenAI-compatible", platform)
		}
	}
}
