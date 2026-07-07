// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

package webmock

import "strings"

// Request is an outgoing HTTP request presented to the registry for matching.
// It is the value model a host (rbgo) fills in from its intercepted Net::HTTP
// request. Header names are matched case-insensitively, mirroring HTTP and the
// webmock gem.
type Request struct {
	Method  string
	URI     string
	Headers map[string][]string
	Body    string
}

// StubResponse is a canned response a stub returns when its matcher fires. It
// mirrors webmock's to_return(status:, body:, headers:).
type StubResponse struct {
	Status  int
	Headers map[string][]string
	Body    string
}

// headerValues returns the values of the named header, resolved
// case-insensitively, and whether the header is present.
func headerValues(req Request, name string) ([]string, bool) {
	for k, v := range req.Headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return nil, false
}

// headerFirst returns the first value of the named header, or "".
func headerFirst(req Request, name string) string {
	if v, ok := headerValues(req, name); ok && len(v) > 0 {
		return v[0]
	}
	return ""
}
