package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPPVideoTaskListFilterNormalized(t *testing.T) {
	got := PPVideoTaskListFilter{TaskID: "  abc ", Model: " M ", APIKey: " k ", Status: " FAILED ", Page: 0, PageSize: 0}.Normalized()
	require.Equal(t, "abc", got.TaskID)
	require.Equal(t, "M", got.Model)
	require.Equal(t, "k", got.APIKey)
	require.Equal(t, "failed", got.Status)
	require.Equal(t, 1, got.Page)
	require.Equal(t, PPVideoTaskListDefaultPageSize, got.PageSize)

	require.Equal(t, PPVideoTaskListMaxPageSize, PPVideoTaskListFilter{PageSize: 100000}.Normalized().PageSize)
	require.Equal(t, 3, PPVideoTaskListFilter{Page: 3}.Normalized().Page)
}

func TestPPVideoTaskStatusValuesCoverEveryNormalizedStatus(t *testing.T) {
	for _, group := range []string{
		PPVideoTaskStatusProcessing, PPVideoTaskStatusSucceeded, PPVideoTaskStatusFailed, PPVideoTaskStatusRequiresAttention,
	} {
		values := PPVideoTaskStatusValues(group)
		require.NotEmpty(t, values, group)
		for _, raw := range values {
			require.Equal(t, group, PPVideoTaskStatusGroup(raw), "raw status %q must belong to %q", raw, group)
		}
	}
	require.Nil(t, PPVideoTaskStatusValues(""))
	require.Nil(t, PPVideoTaskStatusValues("nonsense"))
}

func TestPPVideoTaskStatusValuesReturnsACopy(t *testing.T) {
	values := PPVideoTaskStatusValues(PPVideoTaskStatusFailed)
	values[0] = "mutated"
	require.NotEqual(t, "mutated", PPVideoTaskStatusValues(PPVideoTaskStatusFailed)[0])
}

func TestPPVideoTaskStatusGroupTreatsSubmittingAsProcessing(t *testing.T) {
	require.Equal(t, PPVideoTaskStatusProcessing, PPVideoTaskStatusGroup(PPVideoTaskStatusSubmitting))
	require.Equal(t, PPVideoTaskStatusSucceeded, PPVideoTaskStatusGroup("Completed"))
	require.Equal(t, PPVideoTaskStatusFailed, PPVideoTaskStatusGroup("cancelled"))
}

func TestPPVideoTaskCanStreamContent(t *testing.T) {
	for _, platform := range []string{
		PlatformCTMOSeedance, PlatformXingAPISeedance, PlatformMaiTokenSeedance, PlatformOPONE, PlatformGuangying, PlatformKling,
	} {
		require.True(t, PPVideoTaskCanStreamContent(platform), platform)
	}
	require.False(t, PPVideoTaskCanStreamContent(""))
	require.False(t, PPVideoTaskCanStreamContent(PlatformOpenAI))
}

func TestPPVideoTaskEndpoint(t *testing.T) {
	require.Equal(t, "/v1/videos/generations", PPVideoTaskEndpoint(PPVideoOperationGeneric))
	require.Equal(t, "/v1/videos/text2video", PPVideoTaskEndpoint(PPVideoOperationKlingTextToVideo))
	require.Equal(t, "/v1/videos/image2video", PPVideoTaskEndpoint(PPVideoOperationKlingImageToVideo))
	require.Equal(t, "/v1/videos/generations", PPVideoTaskEndpoint(""))
}

func TestPPVideoTaskRecordDurationSeconds(t *testing.T) {
	require.InDelta(t, 15, (&PPVideoTaskRecord{GeneratedVideoDurationMilliseconds: 15000, RequestedVideoDurationSeconds: 10}).DurationSeconds(), 1e-9)
	require.InDelta(t, 5.5, (&PPVideoTaskRecord{RequestedVideoDurationMilliseconds: 5500, RequestedVideoDurationSeconds: 5}).DurationSeconds(), 1e-9)
	require.InDelta(t, 8, (&PPVideoTaskRecord{RequestedVideoDurationSeconds: 8}).DurationSeconds(), 1e-9)
	var nilRecord *PPVideoTaskRecord
	require.InDelta(t, 0, nilRecord.DurationSeconds(), 1e-9)
}

func TestPPVideoTaskRecordTotalSeconds(t *testing.T) {
	created := time.Date(2026, 11, 6, 14, 34, 8, 0, time.UTC)
	finished := created.Add(9*time.Minute + 36*time.Second)

	got := (&PPVideoTaskRecord{CreatedAt: created, FinishedAt: &finished}).TotalSeconds()
	require.NotNil(t, got)
	require.InDelta(t, 576, *got, 1e-9)

	require.Nil(t, (&PPVideoTaskRecord{CreatedAt: created}).TotalSeconds())
	before := created.Add(-time.Second)
	require.Nil(t, (&PPVideoTaskRecord{CreatedAt: created, FinishedAt: &before}).TotalSeconds())
}

func TestPPVideoTaskRecordCreditsRefunded(t *testing.T) {
	actual := func(v float64) *float64 { return &v }

	released := (&PPVideoTaskRecord{BillingStatus: PPVideoBillingStatusReleased, HoldAmount: 6.5}).CreditsRefunded()
	require.NotNil(t, released)
	require.InDelta(t, 6.5, *released, 1e-9)

	partial := (&PPVideoTaskRecord{BillingStatus: PPVideoBillingStatusCaptured, HoldAmount: 24, ActualCost: actual(12)}).CreditsRefunded()
	require.NotNil(t, partial)
	require.InDelta(t, 12, *partial, 1e-9)

	full := (&PPVideoTaskRecord{BillingStatus: PPVideoBillingStatusCaptured, HoldAmount: 24, ActualCost: actual(24)}).CreditsRefunded()
	require.NotNil(t, full)
	require.InDelta(t, 0, *full, 1e-9)

	over := (&PPVideoTaskRecord{BillingStatus: PPVideoBillingStatusCaptured, HoldAmount: 10, ActualCost: actual(12)}).CreditsRefunded()
	require.NotNil(t, over)
	require.InDelta(t, 0, *over, 1e-9)

	require.Nil(t, (&PPVideoTaskRecord{BillingStatus: PPVideoBillingStatusHeld, HoldAmount: 6.5}).CreditsRefunded())
	require.Nil(t, (&PPVideoTaskRecord{BillingStatus: PPVideoBillingStatusNone, HoldAmount: 0}).CreditsRefunded())
	require.Nil(t, (&PPVideoTaskRecord{BillingStatus: PPVideoBillingStatusCaptured, HoldAmount: 5}).CreditsRefunded())
}
