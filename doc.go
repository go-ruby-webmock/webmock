// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package webmock is a pure-Go (CGO-free) reimplementation of the deterministic
// core of Ruby's [webmock] gem — an HTTP request matcher, stub registry, and
// recorded-request history — without any Ruby runtime.
//
// webmock lets a test declare stubs ("when a GET goes to www.example.com with
// these headers and body, answer 200 with this body") and then matches live
// requests against them, recording every request so the test can assert how
// many times an endpoint was hit. Everything webmock does around the wire —
// deciding whether a request matches a stub, picking the stubbed response,
// counting requests, refusing unstubbed traffic when the network is disabled —
// is deterministic and needs no interpreter, so it lives here as pure Go. The
// interception of the real HTTP transport is the host's job: rbgo binds this
// engine to its Net::HTTP replacement and feeds every outgoing request through
// [Registry.Match].
//
// It is the webmock engine for
// [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a
// standalone, reusable module — a sibling of go-ruby-openbao/openbao and
// go-ruby-net-http/net-http.
//
// # Value model
//
// A [Request] is the request under test (Method, URI, Headers, Body). A
// [StubResponse] is a canned answer (Status, Headers, Body). A [Stub] pairs a
// request matcher with an ordered list of behaviours (return a response, raise
// an error, time out), built fluently:
//
//	reg := webmock.NewRegistry()
//	reg.StubRequest("GET", "www.example.com").
//		With(webmock.Header("Accept", "application/json")).
//		ToReturn(webmock.StubResponse{Status: 200, Body: `{"ok":true}`})
//
//	resp, err := reg.Match(webmock.Request{
//		Method:  "GET",
//		URI:     "http://www.example.com/",
//		Headers: map[string][]string{"Accept": {"application/json"}},
//	})
//	// resp.Status == 200; err == nil
//
// When no stub matches, [Registry.Match] returns a [*NoStubError] carrying a
// webmock-style diff (unless net connections have been re-enabled with
// [Registry.AllowNetConnect], in which case it returns [ErrNetConnectAllowed] so
// the host can perform the real request). [Registry.Requested] and
// [Registry.AssertRequested] mirror the gem's assert_requested / have_requested.
//
// [webmock]: https://github.com/bblimke/webmock
package webmock
