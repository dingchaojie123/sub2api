package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The reason reported for a finished-but-unsettled task must agree with what
// parseMaiTokenSeedanceResponse actually decides, for every body it can see.
func TestMaiTokenCompletedHeldReason(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		body       string
		wantReason string
		wantStatus string
	}{
		{"settled", `{"id":"t","status":"completed","usage":{"billing_seconds":11}}`, "", PPVideoTaskStatusSucceeded},
		{"upstream billing pending", `{"id":"t","status":"completed","billing_pending":true,"usage":{"billing_seconds":11}}`, maiTokenHeldUpstreamBillingPending, PPVideoTaskStatusProcessing},
		{"upstream billing pending wins over missing seconds", `{"id":"t","status":"completed","billing_pending":true}`, maiTokenHeldUpstreamBillingPending, PPVideoTaskStatusProcessing},
		{"usage missing", `{"id":"t","status":"completed"}`, maiTokenHeldBillingSecondsMissing, PPVideoTaskStatusProcessing},
		{"billing seconds missing", `{"id":"t","status":"completed","usage":{}}`, maiTokenHeldBillingSecondsMissing, PPVideoTaskStatusProcessing},
		{"billing seconds is a string", `{"id":"t","status":"completed","usage":{"billing_seconds":"11"}}`, maiTokenHeldBillingSecondsNotNum, PPVideoTaskStatusProcessing},
		{"billing seconds is null", `{"id":"t","status":"completed","usage":{"billing_seconds":null}}`, maiTokenHeldBillingSecondsNotNum, PPVideoTaskStatusProcessing},
		{"billing seconds is fractional", `{"id":"t","status":"completed","usage":{"billing_seconds":11.5}}`, maiTokenHeldBillingSecondsNotInt, PPVideoTaskStatusProcessing},
		{"billing seconds written with a decimal point", `{"id":"t","status":"completed","usage":{"billing_seconds":11.0}}`, maiTokenHeldBillingSecondsNotInt, PPVideoTaskStatusProcessing},
		{"billing seconds overflows int64", `{"id":"t","status":"completed","usage":{"billing_seconds":9223372036854775808}}`, maiTokenHeldBillingSecondsNotInt, PPVideoTaskStatusProcessing},
		{"billing seconds is zero", `{"id":"t","status":"completed","usage":{"billing_seconds":0}}`, maiTokenHeldBillingSecondsNotPos, PPVideoTaskStatusProcessing},
		{"billing seconds is negative", `{"id":"t","status":"completed","usage":{"billing_seconds":-1}}`, maiTokenHeldBillingSecondsNotPos, PPVideoTaskStatusProcessing},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.wantReason, maiTokenCompletedHeldReason([]byte(tc.body)))

			parsed, err := parseMaiTokenSeedanceResponse([]byte(tc.body))
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, parsed.Status)
			// A held response always has a reason and a settled one never does.
			require.Equal(t, tc.wantStatus == PPVideoTaskStatusProcessing, tc.wantReason != "")
		})
	}
}

func TestLogMaiTokenCompletedHeldDoesNotPanicOnOddBodies(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"status":"completed"}`,
		`{"status":"completed","usage":"not-an-object"}`,
		`{"status":"completed","usage":{"billing_seconds":null},"result":{"url":"https://example.com/x?sig=secret"}}`,
	} {
		require.NotPanics(t, func() { logMaiTokenCompletedHeld("task-1", []byte(body)) }, body)
	}
}
