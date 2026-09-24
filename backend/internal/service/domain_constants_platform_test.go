package service

import "testing"

func TestProviderPlatformsAreOpenAICompatibleAndQuotaEnabled(t *testing.T) {
	platforms := []string{
		PlatformDoubao,
		PlatformQwen,
		PlatformKimi,
		PlatformDeepSeek,
		PlatformMidjourney,
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
		PlatformByteDance,
		PlatformWan3,
		PlatformMiniMaxH3,
		PlatformPixverseV6,
		PlatformGrokImagineVideo,
		PlatformKuaishou,
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

func TestAudioPlatformsAreAllowedQuotaPlatforms(t *testing.T) {
	for _, platform := range []string{PlatformMiniMaxSpeech, PlatformQwenTTS} {
		if !IsAllowedQuotaPlatform(platform) {
			t.Fatalf("audio platform %q must be allowed for user quotas", platform)
		}
		if IsOpenAICompatiblePlatform(platform) {
			t.Fatalf("audio platform %q must not be classified as OpenAI-compatible", platform)
		}
	}
}
