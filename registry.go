// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

package webmock

import (
	"regexp"
	"sync"
)

// Registry holds registered stubs, the recorded request history, and the
// net-connect policy. It is the deterministic engine a host drives: every
// intercepted request goes through [Registry.Match]. A Registry is safe for
// concurrent use.
type Registry struct {
	mu              sync.Mutex
	stubs           []*Stub
	history         []Request
	allowNetConnect bool
}

// NewRegistry returns an empty registry with net connections disabled, matching
// webmock's default of refusing unstubbed requests.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a pre-built stub and returns it for chaining.
func (r *Registry) Register(s *Stub) *Stub {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stubs = append(r.stubs, s)
	return s
}

// StubRequest builds and registers a stub, mirroring stub_request(method, uri).
func (r *Registry) StubRequest(method, uri string, opts ...Option) *Stub {
	return r.Register(NewStub(method, uri, opts...))
}

// StubRequestRe builds and registers a stub whose URI is matched by regexp.
func (r *Registry) StubRequestRe(method string, re *regexp.Regexp, opts ...Option) *Stub {
	return r.Register(NewStubRe(method, re, opts...))
}

// AllowNetConnect re-enables passthrough of unstubbed requests, mirroring
// WebMock.allow_net_connect!.
func (r *Registry) AllowNetConnect() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.allowNetConnect = true
}

// DisableNetConnect forbids unstubbed requests, mirroring
// WebMock.disable_net_connect! (the default).
func (r *Registry) DisableNetConnect() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.allowNetConnect = false
}

// Reset clears stubs and history, mirroring WebMock.reset!.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stubs = nil
	r.history = nil
}

// Requests returns a copy of the recorded request history.
func (r *Registry) Requests() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Request(nil), r.history...)
}

// Match resolves a request against the registered stubs. On a match it records
// the request and returns the next stubbed outcome: a [StubResponse], or one of
// [ErrTimeout] / a [*RaiseError] for to_timeout / to_raise stubs. When several
// stubs match, the most recently registered one wins (webmock semantics). With
// no match it returns [ErrNetConnectAllowed] if net connections are enabled, or
// a [*NoStubError] diff otherwise.
func (r *Registry) Match(req Request) (StubResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.stubs) - 1; i >= 0; i-- {
		if r.stubs[i].m.matches(req) {
			r.history = append(r.history, req)
			return r.stubs[i].respond()
		}
	}
	if r.allowNetConnect {
		r.history = append(r.history, req)
		return StubResponse{}, ErrNetConnectAllowed
	}
	return StubResponse{}, &NoStubError{Request: req, Registered: append([]*Stub(nil), r.stubs...)}
}

// Requested counts recorded requests matching method/uri and the given
// constraints, mirroring a_request(...).should have_been_made / the count
// behind assert_requested.
func (r *Registry) Requested(method, uri string, opts ...Option) int {
	m := newMatcher(method, newStringURI(uri), opts)
	return r.countMatching(m)
}

// RequestedRe is like [Registry.Requested] but matches the URI by regexp.
func (r *Registry) RequestedRe(method string, re *regexp.Regexp, opts ...Option) int {
	m := newMatcher(method, &uriPattern{re: re}, opts)
	return r.countMatching(m)
}

func (r *Registry) countMatching(m *matcher) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, req := range r.history {
		if m.matches(req) {
			n++
		}
	}
	return n
}

// AssertRequested returns nil if method/uri was requested exactly times, or an
// [*AssertionError] otherwise, mirroring assert_requested(..., times:).
func (r *Registry) AssertRequested(method, uri string, times int, opts ...Option) error {
	m := newMatcher(method, newStringURI(uri), opts)
	got := r.countMatching(m)
	if got != times {
		return &AssertionError{Desc: m.describe(), Expected: times, Got: got}
	}
	return nil
}
