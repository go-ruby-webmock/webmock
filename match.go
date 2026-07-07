// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

package webmock

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// schemeRe detects an explicit URI scheme (e.g. "https://").
var schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// matcher is the request-matching half of a stub: method, URI, and the
// optional .with(...) constraints. A nil/zero constraint means "don't care".
type matcher struct {
	method  string // upper-case; "" or "ANY" matches any method
	uri     *uriPattern
	headers map[string]valueMatcher // lower-case name -> value matcher
	body    bodyMatcher
	query   queryMatcher
	block   func(Request) bool
}

func newMatcher(method string, uri *uriPattern, opts []Option) *matcher {
	m := &matcher{
		method:  strings.ToUpper(method),
		uri:     uri,
		headers: map[string]valueMatcher{},
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// matches reports whether req satisfies every constraint in the matcher.
func (m *matcher) matches(req Request) bool {
	if !methodMatches(m.method, req.Method) {
		return false
	}
	if !m.uri.match(req.URI) {
		return false
	}
	if m.query.set && !m.query.match(req.URI) {
		return false
	}
	for name, vm := range m.headers {
		if !headerMatches(req, name, vm) {
			return false
		}
	}
	if m.body.set && !m.body.match(req) {
		return false
	}
	if m.block != nil && !m.block(req) {
		return false
	}
	return true
}

// methodMatches reports whether a request method satisfies the stub method.
// An empty or "ANY" stub method matches any request method (webmock :any).
func methodMatches(stub, req string) bool {
	if stub == "" || stub == "ANY" {
		return true
	}
	return strings.EqualFold(stub, req)
}

// headerMatches reports whether the named request header is present and at
// least one of its values satisfies the matcher.
func headerMatches(req Request, name string, vm valueMatcher) bool {
	vals, ok := headerValues(req, name)
	if !ok {
		return false
	}
	for _, v := range vals {
		if vm.match(v) {
			return true
		}
	}
	return false
}

// valueMatcher matches a single scalar string, either exactly or by regexp.
type valueMatcher struct {
	exact string
	re    *regexp.Regexp
}

func (v valueMatcher) match(s string) bool {
	if v.re != nil {
		return v.re.MatchString(s)
	}
	return v.exact == s
}

func (v valueMatcher) describe() string {
	if v.re != nil {
		return "/" + v.re.String() + "/"
	}
	return fmt.Sprintf("%q", v.exact)
}

// bodyMatcher matches a request body: exact string, regexp, or a hash (form- or
// JSON-encoded) of expected fields.
type bodyMatcher struct {
	set   bool
	exact *string
	re    *regexp.Regexp
	form  map[string]string
}

func (b bodyMatcher) match(req Request) bool {
	switch {
	case b.exact != nil:
		return *b.exact == req.Body
	case b.re != nil:
		return b.re.MatchString(req.Body)
	case b.form != nil:
		return matchFormBody(b.form, req)
	default:
		return false
	}
}

func (b bodyMatcher) describe() string {
	switch {
	case b.exact != nil:
		return fmt.Sprintf("%q", *b.exact)
	case b.re != nil:
		return "/" + b.re.String() + "/"
	case b.form != nil:
		return describeStringMap(b.form)
	default:
		return "nil"
	}
}

// matchFormBody decodes the request body per its Content-Type (JSON if the
// header says so, otherwise URL-encoded form) and compares its flat fields to
// the expected hash.
func matchFormBody(want map[string]string, req Request) bool {
	got := map[string]string{}
	if strings.Contains(strings.ToLower(headerFirst(req, "Content-Type")), "json") {
		var raw map[string]any
		if json.Unmarshal([]byte(req.Body), &raw) != nil {
			return false
		}
		for k, v := range raw {
			got[k] = fmt.Sprint(v)
		}
	} else {
		vals, err := url.ParseQuery(req.Body)
		if err != nil {
			return false
		}
		for k := range vals {
			got[k] = vals.Get(k)
		}
	}
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// queryMatcher matches request query parameters order-insensitively.
type queryMatcher struct {
	set    bool
	values url.Values
}

func (q queryMatcher) match(reqURI string) bool {
	u, err := url.Parse(reqURI)
	if err != nil {
		return false
	}
	return sameValues(q.values, u.Query())
}

// uriPattern is a parsed stub URI: either a regexp over the whole request URI,
// or a structured scheme/host/port/path(/query) comparison. A scheme is matched
// only when the stub gave one explicitly (webmock is scheme-agnostic otherwise).
type uriPattern struct {
	raw         string
	parseErr    error
	re          *regexp.Regexp
	schemeGiven bool
	scheme      string
	host        string
	port        string
	path        string
	query       url.Values
}

// newStringURI parses a string stub URI. A missing scheme is tolerated (and not
// required to match). A malformed URI yields a pattern that matches nothing.
func newStringURI(s string) *uriPattern {
	p := &uriPattern{raw: s}
	target := s
	if !schemeRe.MatchString(s) {
		target = "http://" + s
	} else {
		p.schemeGiven = true
	}
	u, err := url.Parse(target)
	if err != nil {
		p.parseErr = err
		return p
	}
	p.scheme = u.Scheme
	p.host = strings.ToLower(u.Hostname())
	p.port = u.Port()
	p.path = u.EscapedPath()
	if p.path == "" {
		p.path = "/"
	}
	if u.RawQuery != "" {
		p.query = u.Query()
	}
	return p
}

func (p *uriPattern) match(reqURI string) bool {
	if p.parseErr != nil {
		return false
	}
	if p.re != nil {
		return p.re.MatchString(reqURI)
	}
	u, err := url.Parse(reqURI)
	if err != nil {
		return false
	}
	if p.schemeGiven && !strings.EqualFold(p.scheme, u.Scheme) {
		return false
	}
	if p.host != strings.ToLower(u.Hostname()) {
		return false
	}
	patScheme := u.Scheme
	if p.schemeGiven {
		patScheme = p.scheme
	}
	if defaultPort(patScheme, p.port) != defaultPort(u.Scheme, u.Port()) {
		return false
	}
	reqPath := u.EscapedPath()
	if reqPath == "" {
		reqPath = "/"
	}
	if p.path != reqPath {
		return false
	}
	if p.query != nil && !sameValues(p.query, u.Query()) {
		return false
	}
	return true
}

func (p *uriPattern) describe() string {
	if p.re != nil {
		return "/" + p.re.String() + "/"
	}
	return fmt.Sprintf("%q", p.raw)
}

// defaultPort resolves an empty port to the scheme's default, so
// "http://h" and "http://h:80" compare equal.
func defaultPort(scheme, port string) string {
	if port != "" {
		return port
	}
	switch strings.ToLower(scheme) {
	case "https", "wss":
		return "443"
	case "http", "ws":
		return "80"
	default:
		return "0"
	}
}

// sameValues reports whether two url.Values hold the same keys with the same
// multiset of values, regardless of order.
func sameValues(a, b url.Values) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || len(av) != len(bv) {
			return false
		}
		as := append([]string(nil), av...)
		bs := append([]string(nil), bv...)
		sort.Strings(as)
		sort.Strings(bs)
		for i := range as {
			if as[i] != bs[i] {
				return false
			}
		}
	}
	return true
}

// describe renders a matcher as a webmock-style stub_request(...) snippet.
func (m *matcher) describe() string {
	var b strings.Builder
	method := strings.ToLower(m.method)
	if method == "" || method == "any" {
		method = "any"
	}
	b.WriteString("stub_request(:")
	b.WriteString(method)
	b.WriteString(", ")
	b.WriteString(m.uri.describe())
	b.WriteString(")")

	var parts []string
	if len(m.headers) > 0 {
		parts = append(parts, "headers: "+describeHeaderMatchers(m.headers))
	}
	if m.body.set {
		parts = append(parts, "body: "+m.body.describe())
	}
	if m.query.set {
		parts = append(parts, "query: "+describeValues(m.query.values))
	}
	if m.block != nil {
		parts = append(parts, "block: <func>")
	}
	if len(parts) > 0 {
		b.WriteString(".with(" + strings.Join(parts, ", ") + ")")
	}
	return b.String()
}

func describeHeaderMatchers(h map[string]valueMatcher) string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q=>%s", k, h[k].describe()))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func describeValues(v url.Values) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q=>%q", k, strings.Join(v[k], ", ")))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func describeStringMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q=>%q", k, m[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
