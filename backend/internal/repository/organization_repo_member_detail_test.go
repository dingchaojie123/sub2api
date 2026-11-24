//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestMemberImageBatchStatusGroup(t *testing.T) {
	for status, want := range map[string]string{
		"completed":      service.PPVideoTaskStatusSucceeded,
		"output_deleted": service.PPVideoTaskStatusSucceeded,
		"failed":         service.PPVideoTaskStatusFailed,
		"cancelled":      service.PPVideoTaskStatusFailed,
		" Canceled ":     service.PPVideoTaskStatusFailed,
		"created":        service.PPVideoTaskStatusProcessing,
		"submitted":      service.PPVideoTaskStatusProcessing,
		"running":        service.PPVideoTaskStatusProcessing,
		"":               service.PPVideoTaskStatusProcessing,
	} {
		require.Equal(t, want, memberImageBatchStatusGroup(status), status)
	}
}
