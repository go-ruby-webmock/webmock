// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

package webmock

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ErrTimeout is returned by [Registry.Match] when the matched stub is a
// to_timeout stub. It mirrors the gem raising a timeout on the bound transport.
var ErrTimeout = errors.New("webmock: stubbed request timed out")

// ErrNetConnectAllowed is returned by [Registry.Match] when no stub matches but
// real net connections have been enabled with [Registry.AllowNetConnect]. The
// host should perform the real request itself; this engine opens no socket.
var ErrNetConnectAllowed = errors.New("webmock: no stub registered; real net connection allowed")

// RaiseError is returned by [Registry.Match] when the matched stub is a
// to_raise stub. It wraps the exception the stub was told to raise, so
// errors.Is / errors.Unwrap reach the underlying error.
type RaiseError struct {
	Err error
}

func (e *RaiseError) Error() string {
	if e.Err == nil {
		return "webmock: stubbed request raised"
	}
	return "webmock: stubbed request raised: " + e.Err.Error()
}

// Unwrap exposes the raised exception to errors.Is / errors.As.
func (e *RaiseError) Unwrap() error { return e.Err }

// AssertionError reports a request-count assertion mismatch, mirroring the
// gem's assert_requested failure message.
type AssertionError struct {
	Desc     string
	Expected int
	Got      int
}

func (e *AssertionError) Error() string {
	return fmt.Sprintf("webmock: expected %s to be requested %d time(s) but it was requested %d time(s)",
		e.Desc, e.Expected, e.Got)
}

// NoStubError is returned by [Registry.Match] when no registered stub matches a
// request and net connections are disabled. Its message reproduces webmock's
// "unregistered request" diff: the offending request, a suggested stub snippet,
// and the list of registered stubs.
type NoStubError struct {
	Request    Request
	Registered []*Stub
}

func (e *NoStubError) Error() string {
	var b strings.Builder
	b.WriteString("webmock: real HTTP connections are disabled. Unregistered request: ")
	b.WriteString(strings.ToUpper(e.Request.Method))
	b.WriteString(" ")
	b.WriteString(e.Request.URI)
	if len(e.Request.Headers) > 0 {
		b.WriteString(" with headers ")
		b.WriteString(describeRequestHeaders(e.Request.Headers))
	}
	if e.Request.Body != "" {
		b.WriteString(" with body ")
		b.WriteString(strconv.Quote(e.Request.Body))
	}
	b.WriteString("\n\nYou can stub this request with the following snippet:\n\n")
	b.WriteString(suggestSnippet(e.Request))
	if len(e.Registered) > 0 {
		b.WriteString("\n\nregistered request stubs:\n")
		for _, s := range e.Registered {
			b.WriteString("  ")
			b.WriteString(s.m.describe())
			b.WriteString("\n")
		}
	} else {
		b.WriteString("\n\nNo stubs are registered.")
	}
	return b.String()
}

// suggestSnippet renders the "you can stub this" snippet for a request.
func suggestSnippet(req Request) string {
	method := strings.ToLower(req.Method)
	if method == "" {
		method = "any"
	}
	return fmt.Sprintf("stub_request(:%s, %q).\n  to_return(status: 200, body: \"\", headers: {})",
		method, req.URI)
}

// describeRequestHeaders renders request headers deterministically for the diff.
func describeRequestHeaders(h map[string][]string) string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q=>%q", k, strings.Join(h[k], ", ")))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
