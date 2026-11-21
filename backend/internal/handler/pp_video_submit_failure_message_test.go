package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPPVideoSubmitFailureMessage(t *testing.T) {
	t.Run("transport failure uses the safe cause", func(t *testing.T) {
		err := &service.UpstreamFailoverError{StatusCode: 502, TransportCause: "connection reset by upstream"}
		require.Equal(t, "upstream connection failed: connection reset by upstream", ppVideoSubmitFailureMessage(err))
	})

	t.Run("wrapped failover error is still recognised", func(t *testing.T) {
		err := fmt.Errorf("forward: %w", &service.UpstreamFailoverError{StatusCode: 502, TransportCause: "upstream request timed out"})
		require.Equal(t, "upstream connection failed: upstream request timed out", ppVideoSubmitFailureMessage(err))
	})

	t.Run("failover error without a cause keeps the old text", func(t *testing.T) {
		err := &service.UpstreamFailoverError{StatusCode: 502}
		require.Equal(t, "upstream error: 502 (failover)", ppVideoSubmitFailureMessage(err))
	})

	t.Run("other errors keep their own text", func(t *testing.T) {
		err := errors.New("PP video request body is empty")
		require.Equal(t, "PP video request body is empty", ppVideoSubmitFailureMessage(err))
	})

	t.Run("nil", func(t *testing.T) {
		require.Equal(t, "", ppVideoSubmitFailureMessage(nil))
	})
}
