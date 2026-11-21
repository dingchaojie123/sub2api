//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOpenAITransportErrorLabel pins the short, customer-safe label of a transport-level
// failure. The label is written into the failure reason of a video task, which customers can
// read, so it must never contain the upstream host, IP address or port.
func TestOpenAITransportErrorLabel(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		// The exact error seen in production for miaowu-seedance submissions.
		{"production connection reset", errors.New(`Post "https://api.miaowuai.store/v1/videos": read tcp 172.30.0.4:43992->172.67.212.49:443: read: connection reset by peer`), "connection reset by upstream"},
		{"ECONNRESET typed", &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}, "connection reset by upstream"},
		{"connection refused", errors.New(`dial tcp 1.2.3.4:443: connect: connection refused`), "connection refused by upstream"},
		{"dns not found", &net.DNSError{Err: "no such host", Name: "api.example.com", IsNotFound: true}, "upstream host lookup failed"},
		{"unexpected eof", io.ErrUnexpectedEOF, "connection closed by upstream (EOF)"},
		{"plain eof", errors.New(`Post "https://api.example.com/v1/videos": EOF`), "connection closed by upstream (EOF)"},
		{"broken pipe", errors.New(`write tcp 10.0.0.1:5->2.2.2.2:443: write: broken pipe`), "connection closed by upstream"},
		{"client timeout", errors.New(`Post "https://api.example.com/v1/videos": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`), "upstream request timed out"},
		{"deadline exceeded typed", context.DeadlineExceeded, "upstream request timed out"},
		{"tls handshake", errors.New(`Post "https://api.example.com/v1/videos": remote error: tls: handshake failure`), "upstream TLS error"},
		{"proxy", errors.New(`proxyconnect tcp: dial tcp 1.2.3.4:1080: i/o timeout`), "proxy connection failed"},
		{"unknown", errors.New("something else entirely"), "upstream transport error"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, openAITransportErrorLabel(tc.err))
		})
	}
}

func TestOpenAITransportErrorLabel_NeverLeaksUpstreamAddress(t *testing.T) {
	raw := errors.New(`Post "https://api.miaowuai.store/v1/videos": read tcp 172.30.0.4:43992->172.67.212.49:443: read: connection reset by peer`)
	label := openAITransportErrorLabel(raw)
	for _, leaked := range []string{"miaowuai", "https://", "172.", ":443", "/v1/videos"} {
		require.NotContains(t, label, leaked)
	}
}

// The failover error built for a transport failure carries the label, and its Error() text
// is unchanged so every other caller keeps seeing the same message.
func TestHandleOpenAIUpstreamTransportError_CarriesTransportCause(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 89, Name: "miaowu", Platform: PlatformMiaowuSeedance}
	c, _ := newOpenAITransportErrTestContext()

	retErr := svc.handleOpenAIUpstreamTransportError(context.Background(), c, account,
		errors.New(`Post "https://api.miaowuai.store/v1/videos": read tcp 172.30.0.4:43992->172.67.212.49:443: read: connection reset by peer`), false)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, retErr, &failoverErr)
	require.Equal(t, "connection reset by upstream", failoverErr.TransportCause)
	require.Equal(t, "upstream error: 502 (failover)", failoverErr.Error())
}
