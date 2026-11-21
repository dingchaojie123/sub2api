package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const ppVideoSubmitRetryTestBody = `{"model_name":"kling-v3","prompt":"a cat by the sea","duration":"5","mode":"pro"}`

func shortenPPVideoSubmitRetryTimings(t *testing.T, connectTimeout time.Duration) {
	t.Helper()
	previousTimeout, previousDelay := ppVideoSubmitConnectTimeout, ppVideoSubmitRetryDelay
	ppVideoSubmitConnectTimeout = connectTimeout
	ppVideoSubmitRetryDelay = time.Millisecond
	t.Cleanup(func() {
		ppVideoSubmitConnectTimeout = previousTimeout
		ppVideoSubmitRetryDelay = previousDelay
	})
}

// ppVideoSubmitResetBeforeSendStep fails the way a connection that could not be established does:
// the transport asked for a connection and never wrote the request.
func ppVideoSubmitResetBeforeSendStep(req *http.Request) (*http.Response, error) {
	if trace := httptrace.ContextClientTrace(req.Context()); trace != nil && trace.GetConn != nil {
		trace.GetConn("pp.example:443")
	}
	return ppVideoStatusRetryResetStep(req)
}

// ppVideoSubmitResetAfterWriteStep fails the way production did: the request was written and the
// connection was reset while waiting for the answer, so the upstream may have created the task.
func ppVideoSubmitResetAfterWriteStep(req *http.Request) (*http.Response, error) {
	trace := httptrace.ContextClientTrace(req.Context())
	if trace != nil {
		trace.GetConn("pp.example:443")
		trace.GotConn(httptrace.GotConnInfo{})
		trace.WroteHeaders()
	}
	return ppVideoStatusRetryResetStep(req)
}

func ppVideoSubmitStallInConnectStep(req *http.Request) (*http.Response, error) {
	if trace := httptrace.ContextClientTrace(req.Context()); trace != nil && trace.GetConn != nil {
		trace.GetConn("pp.example:443")
	}
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	case <-time.After(5 * time.Second):
		return nil, errors.New("the connect timeout did not cut off the stalled attempt")
	}
}

func forwardPPVideoSubmitForRetryTest(ctx context.Context, upstream *ppVideoStatusRetryUpstream) (*OpenAIForwardResult, error) {
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	return svc.ForwardPPVideoBuffered(ctx, ppVideoStatusRetryContext(http.MethodPost),
		ppVideoStatusRetryAccount(), PPVideoOperationKlingTextToVideo, "", []byte(ppVideoSubmitRetryTestBody))
}

func TestForwardPPVideoSubmissionRetriesWhenRequestWasNeverSent(t *testing.T) {
	shortenPPVideoSubmitRetryTimings(t, time.Minute)
	var bodies []string
	recordBody := func(step func(*http.Request) (*http.Response, error)) func(*http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			bodies = append(bodies, string(raw))
			return step(req)
		}
	}
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		recordBody(ppVideoSubmitResetBeforeSendStep),
		recordBody(ppVideoSubmitResetBeforeSendStep),
		recordBody(ppVideoStatusRetrySucceededStep),
	}}

	result, err := forwardPPVideoSubmitForRetryTest(context.Background(), upstream)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 3)
	// Every attempt must carry the complete body, not what the previous attempt left unread.
	require.Equal(t, []string{ppVideoSubmitRetryTestBody, ppVideoSubmitRetryTestBody, ppVideoSubmitRetryTestBody}, bodies)
	for _, req := range upstream.requests {
		require.Equal(t, http.MethodPost, req.Method)
	}
}

func TestForwardPPVideoSubmissionGivesUpAfterMaxAttempts(t *testing.T) {
	shortenPPVideoSubmitRetryTimings(t, time.Minute)
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoSubmitResetBeforeSendStep,
		ppVideoSubmitResetBeforeSendStep,
		ppVideoSubmitResetBeforeSendStep,
	}}

	result, err := forwardPPVideoSubmitForRetryTest(context.Background(), upstream)

	require.Nil(t, result)
	require.Error(t, err)
	require.Len(t, upstream.requests, ppVideoSubmitMaxAttempts)
	// The customer-facing failure message is built from this label.
	require.Equal(t, "connection reset by upstream", ppVideoTransportCause(err))
}

func TestForwardPPVideoSubmissionIsNotRetriedOnceRequestMayHaveBeenSent(t *testing.T) {
	shortenPPVideoSubmitRetryTimings(t, time.Minute)
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoSubmitResetAfterWriteStep,
		ppVideoStatusRetrySucceededStep,
	}}

	_, err := forwardPPVideoSubmitForRetryTest(context.Background(), upstream)

	require.Error(t, err)
	// The upstream may already have created, and billed, a task; a second POST could create another.
	require.Len(t, upstream.requests, 1)
}

func TestForwardPPVideoSubmissionIsNotRetriedWhenTransportNeverStarted(t *testing.T) {
	shortenPPVideoSubmitRetryTimings(t, time.Minute)
	// No trace hook fires: nothing proves the request was not sent, so it must not be repeated.
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoStatusRetryResetStep,
		ppVideoStatusRetrySucceededStep,
	}}

	_, err := forwardPPVideoSubmitForRetryTest(context.Background(), upstream)

	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
}

func TestForwardPPVideoSubmissionRetriesAStalledConnect(t *testing.T) {
	shortenPPVideoSubmitRetryTimings(t, 50*time.Millisecond)
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoSubmitStallInConnectStep,
		ppVideoStatusRetrySucceededStep,
	}}

	started := time.Now()
	result, err := forwardPPVideoSubmitForRetryTest(context.Background(), upstream)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 2)
	require.True(t, time.Since(started) < 3*time.Second, "the stalled connect must be cut off by its own timeout")
}

func TestForwardPPVideoSubmissionStalledConnectOnEveryAttemptReportsTimeout(t *testing.T) {
	shortenPPVideoSubmitRetryTimings(t, 30*time.Millisecond)
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoSubmitStallInConnectStep,
		ppVideoSubmitStallInConnectStep,
		ppVideoSubmitStallInConnectStep,
	}}

	_, err := forwardPPVideoSubmitForRetryTest(context.Background(), upstream)

	require.Error(t, err)
	require.Len(t, upstream.requests, ppVideoSubmitMaxAttempts)
	require.Equal(t, "upstream request timed out", ppVideoTransportCause(err))
}

func TestForwardPPVideoSubmissionIsNotRetriedForDurableFaults(t *testing.T) {
	shortenPPVideoSubmitRetryTimings(t, time.Minute)
	refused := func(req *http.Request) (*http.Response, error) {
		if trace := httptrace.ContextClientTrace(req.Context()); trace != nil && trace.GetConn != nil {
			trace.GetConn("pp.example:443")
		}
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	}
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		refused,
		ppVideoStatusRetrySucceededStep,
	}}

	_, err := forwardPPVideoSubmitForRetryTest(context.Background(), upstream)

	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
}

func TestForwardPPVideoSubmissionIsNotRetriedAfterCallerDeadline(t *testing.T) {
	shortenPPVideoSubmitRetryTimings(t, time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoSubmitResetBeforeSendStep,
		ppVideoStatusRetrySucceededStep,
	}}

	_, err := forwardPPVideoSubmitForRetryTest(ctx, upstream)

	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
}
