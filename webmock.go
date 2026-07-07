// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

package webmock

import "regexp"

// Default is the process-wide registry backing the package-level helpers, so a
// host can call [StubRequest] / [Match] / [AssertRequested] the way Ruby uses
// the global WebMock module. Tests that want isolation should use their own
// [NewRegistry] instead.
var Default = NewRegistry()

// StubRequest builds and registers a stub on [Default].
func StubRequest(method, uri string, opts ...Option) *Stub {
	return Default.StubRequest(method, uri, opts...)
}

// StubRequestRe registers a regexp-URI stub on [Default].
func StubRequestRe(method string, re *regexp.Regexp, opts ...Option) *Stub {
	return Default.StubRequestRe(method, re, opts...)
}

// Match resolves a request against [Default].
func Match(req Request) (StubResponse, error) {
	return Default.Match(req)
}

// Requested counts matching recorded requests on [Default].
func Requested(method, uri string, opts ...Option) int {
	return Default.Requested(method, uri, opts...)
}

// AssertRequested asserts a request count on [Default].
func AssertRequested(method, uri string, times int, opts ...Option) error {
	return Default.AssertRequested(method, uri, times, opts...)
}

// AllowNetConnect enables passthrough of unstubbed requests on [Default].
func AllowNetConnect() { Default.AllowNetConnect() }

// DisableNetConnect forbids unstubbed requests on [Default].
func DisableNetConnect() { Default.DisableNetConnect() }

// Reset clears stubs and history on [Default].
func Reset() { Default.Reset() }
