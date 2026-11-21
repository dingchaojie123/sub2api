package service

import (
	"context"
	"net/http/httptrace"
	"sync/atomic"
	"time"
)

// ppVideoSubmitMaxAttempts is how many times a submission is sent when, and only when, every
// earlier attempt is known not to have reached the upstream (see ppVideoSendProbe).
const ppVideoSubmitMaxAttempts = 3

// ppVideoSubmitConnectTimeout bounds the connection phase of one submission attempt: dial, proxy
// tunnel and TLS handshake. The shared upstream transport has no handshake timeout, so a stalled
// handshake used to hold a submission until the peer reset it (2m19s in production, miaowu behind
// Cloudflare). Nothing has been sent to the upstream while connecting, so cutting the attempt off
// and trying again cannot create a second task. It is a variable only so tests can shorten it.
var ppVideoSubmitConnectTimeout = 20 * time.Second

// ppVideoSubmitRetryDelay is the pause before a submission is sent again.
var ppVideoSubmitRetryDelay = 500 * time.Millisecond

// ppVideoSendProbe records how far one upstream attempt got, using net/http's client trace hooks.
// A submission creates (and bills) a task upstream, so it may only be repeated when it is certain
// that the previous attempt never delivered the request. The probe errs on the side of "delivered":
// the transport fires WroteHeaders before any request byte can leave the process (HTTP/1 writes the
// headers into its buffer first, HTTP/2 hooks the same point), so "WroteHeaders never fired" proves
// that no byte of the request was sent, while a fired hook only means it might have been.
type ppVideoSendProbe struct {
	attempted       atomic.Bool // the transport started to obtain a connection
	connected       atomic.Bool // a connection (including TLS and proxy tunnel) was obtained
	written         atomic.Bool // the request may have been written to the connection
	connectTimedOut atomic.Bool // the attempt was cut off because connecting took too long
}

// newPPVideoSendProbe derives the attempt context. The returned cancel function must always be
// called. When connectTimeout elapses before a connection is obtained and before anything is
// written, the attempt context is cancelled and connectTimedOut is set.
func newPPVideoSendProbe(parent context.Context, connectTimeout time.Duration) (*ppVideoSendProbe, context.Context, context.CancelFunc) {
	probe := &ppVideoSendProbe{}
	ctx, cancel := context.WithCancel(parent)
	var timer *time.Timer
	if connectTimeout > 0 {
		timer = time.AfterFunc(connectTimeout, func() {
			if probe.connected.Load() || probe.written.Load() {
				return
			}
			probe.connectTimedOut.Store(true)
			cancel()
		})
	}
	stopTimer := func() {
		if timer != nil {
			timer.Stop()
		}
	}
	trace := &httptrace.ClientTrace{
		GetConn: func(string) { probe.attempted.Store(true) },
		GotConn: func(httptrace.GotConnInfo) {
			probe.connected.Store(true)
			stopTimer()
		},
		WroteHeaders: func() { probe.written.Store(true) },
		WroteRequest: func(httptrace.WroteRequestInfo) { probe.written.Store(true) },
	}
	ctx = httptrace.WithClientTrace(ctx, trace)
	return probe, ctx, func() {
		stopTimer()
		cancel()
	}
}

// provablyNotDelivered reports whether the transport tried to connect and never wrote any part of
// the request. Requiring that the transport actually started (GetConn fired) keeps errors raised
// before the transport was reached, which would repeat identically, from being retried.
func (p *ppVideoSendProbe) provablyNotDelivered() bool {
	return p != nil && p.attempted.Load() && !p.written.Load()
}
