package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaiTokenSeedancePlatformRegistration(t *testing.T) {
	t.Parallel()

	require.Equal(t, "mai-token-seedance", PlatformMaiTokenSeedance)
	require.Equal(t, "https://api.mai-token.com", MaiTokenSeedanceDefaultBaseURL)
	require.Equal(t, "sdtj-2.5-480p", MaiTokenSeedanceModel)
	require.True(t, IsPPVideoPlatform(PlatformMaiTokenSeedance))
	require.True(t, IsAllowedQuotaPlatform(PlatformMaiTokenSeedance))
}

func TestMaiTokenSeedanceFiltersExactModel(t *testing.T) {
	t.Parallel()

	models := []string{
		"sdtj-2.5-480p",
		"SDTJ-2.5-480P",
		"seedance-2.5",
		"gpt-image-2",
	}
	want := []string{"sdtj-2.5-480p"}
	require.Equal(t, want, filterUpstreamModelsForPlatform(PlatformMaiTokenSeedance, models))
	require.Equal(t, want, filterPPVideoPublicModelsForPlatform(PlatformMaiTokenSeedance, models))
}
