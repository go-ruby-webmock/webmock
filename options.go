// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

package webmock

import (
	"net/url"
	"regexp"
	"strings"
)

// Option is a .with(...) constraint applied to a stub or an assertion. Options
// mirror webmock's with(headers:, body:, query:) and block matchers.
type Option func(*matcher)

// Header constrains a request header to an exact value (case-insensitive name).
func Header(name, value string) Option {
	return func(m *matcher) {
		m.headers[strings.ToLower(name)] = valueMatcher{exact: value}
	}
}

// HeaderRe constrains a request header to match a regexp.
func HeaderRe(name string, re *regexp.Regexp) Option {
	return func(m *matcher) {
		m.headers[strings.ToLower(name)] = valueMatcher{re: re}
	}
}

// Headers constrains several request headers to exact values at once.
func Headers(h map[string]string) Option {
	return func(m *matcher) {
		for k, v := range h {
			m.headers[strings.ToLower(k)] = valueMatcher{exact: v}
		}
	}
}

// Body constrains the request body to an exact string.
func Body(s string) Option {
	return func(m *matcher) {
		m.body = bodyMatcher{set: true, exact: &s}
	}
}

// BodyRe constrains the request body to match a regexp.
func BodyRe(re *regexp.Regexp) Option {
	return func(m *matcher) {
		m.body = bodyMatcher{set: true, re: re}
	}
}

// BodyForm constrains the request body to a hash of fields, decoded from the
// body as URL-encoded form data or JSON per the request's Content-Type.
func BodyForm(fields map[string]string) Option {
	return func(m *matcher) {
		m.body = bodyMatcher{set: true, form: fields}
	}
}

// Query constrains the request query string to the given parameters,
// order-insensitively.
func Query(q map[string]string) Option {
	v := url.Values{}
	for k, val := range q {
		v.Set(k, val)
	}
	return func(m *matcher) {
		m.query = queryMatcher{set: true, values: v}
	}
}

// QueryValues constrains the request query string to the given url.Values,
// order-insensitively (allowing repeated keys).
func QueryValues(v url.Values) Option {
	return func(m *matcher) {
		m.query = queryMatcher{set: true, values: v}
	}
}

// Block constrains the request with an arbitrary predicate, mirroring
// webmock's with { |req| ... } block matcher.
func Block(fn func(Request) bool) Option {
	return func(m *matcher) {
		m.block = fn
	}
}
