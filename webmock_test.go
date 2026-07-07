// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

package webmock

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func req(method, uri string) Request {
	return Request{Method: method, URI: uri}
}

func TestBasicMatchAndDefaultResponse(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "www.example.com")

	resp, err := r.Match(req("GET", "http://www.example.com/"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Status != 200 || resp.Body != "" {
		t.Fatalf("want default 200 empty, got %+v", resp)
	}
	// https also matches a scheme-less stub.
	if _, err := r.Match(req("GET", "https://www.example.com/")); err != nil {
		t.Fatalf("https should match scheme-less stub: %v", err)
	}
	if got := r.Requested("GET", "www.example.com"); got != 2 {
		t.Fatalf("want 2 recorded, got %d", got)
	}
}

func TestToReturnAndZeroStatusDefault(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "http://a.test/x").
		ToReturn(StubResponse{Body: "hi", Headers: map[string][]string{"X": {"1"}}})

	resp, err := r.Match(req("GET", "http://a.test/x"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 || resp.Body != "hi" || resp.Headers["X"][0] != "1" {
		t.Fatalf("got %+v", resp)
	}
}

func TestToReturnNoArgsAppendsDefault(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "a.test").ToReturn()
	resp, err := r.Match(req("GET", "http://a.test/"))
	if err != nil || resp.Status != 200 {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestSequentialResponsesAndExhaustionRepeatsLast(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "seq.test").
		ToReturn(
			StubResponse{Status: 201, Body: "one"},
			StubResponse{Status: 202, Body: "two"},
		)
	want := []string{"one", "two", "two", "two"}
	for i, w := range want {
		resp, err := r.Match(req("GET", "http://seq.test/"))
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if resp.Body != w {
			t.Fatalf("call %d: want %q got %q", i, w, resp.Body)
		}
	}
}

func TestChainedSequences(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "chain.test").
		ToReturn(StubResponse{Body: "a"}).
		ToTimeout().
		ToRaise(errors.New("boom"))

	if resp, err := r.Match(req("GET", "http://chain.test/")); err != nil || resp.Body != "a" {
		t.Fatalf("first: resp=%+v err=%v", resp, err)
	}
	if _, err := r.Match(req("GET", "http://chain.test/")); !errors.Is(err, ErrTimeout) {
		t.Fatalf("second: want timeout, got %v", err)
	}
	_, err := r.Match(req("GET", "http://chain.test/"))
	var re *RaiseError
	if !errors.As(err, &re) || re.Err.Error() != "boom" {
		t.Fatalf("third: want raise boom, got %v", err)
	}
}

func TestToRaiseAndToTimeout(t *testing.T) {
	r := NewRegistry()
	sentinel := errors.New("kaboom")
	r.StubRequest("POST", "raise.test").ToRaise(sentinel)
	r.StubRequest("GET", "timeout.test").ToTimeout()

	_, err := r.Match(req("POST", "http://raise.test/"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("want wrapped sentinel, got %v", err)
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("raise message: %v", err)
	}
	_, err = r.Match(req("GET", "http://timeout.test/"))
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("want timeout, got %v", err)
	}
}

func TestRaiseErrorNilAndTimeoutErrorString(t *testing.T) {
	e := &RaiseError{}
	if e.Error() != "webmock: stubbed request raised" {
		t.Fatalf("nil raise: %q", e.Error())
	}
	if e.Unwrap() != nil {
		t.Fatal("nil unwrap")
	}
	if ErrTimeout.Error() == "" {
		t.Fatal("timeout msg empty")
	}
}

func TestMostRecentStubWins(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "dup.test").ToReturn(StubResponse{Body: "old"})
	r.StubRequest("GET", "dup.test").ToReturn(StubResponse{Body: "new"})
	resp, _ := r.Match(req("GET", "http://dup.test/"))
	if resp.Body != "new" {
		t.Fatalf("want newest stub, got %q", resp.Body)
	}
}

func TestAnyMethod(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("", "any.test").ToReturn(StubResponse{Body: "e"})
	r.StubRequestRe("ANY", regexp.MustCompile(`re\.test`)).ToReturn(StubResponse{Body: "r"})

	if resp, err := r.Match(req("DELETE", "http://any.test/")); err != nil || resp.Body != "e" {
		t.Fatalf("empty method any: resp=%+v err=%v", resp, err)
	}
	if resp, err := r.Match(req("PUT", "http://re.test/path")); err != nil || resp.Body != "r" {
		t.Fatalf("ANY regex: resp=%+v err=%v", resp, err)
	}
}

func TestMethodMismatch(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect() // so no-match returns pass-through, not a diff
	r.StubRequest("GET", "m.test")
	if _, err := r.Match(req("POST", "http://m.test/")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("method mismatch should not match: %v", err)
	}
}

func TestHeaderMatchers(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("GET", "h.test").With(
		Header("Accept", "application/json"),
		HeaderRe("X-Trace", regexp.MustCompile(`^[0-9]+$`)),
	)
	good := Request{Method: "GET", URI: "http://h.test/", Headers: map[string][]string{
		"accept":  {"application/json"},
		"x-trace": {"12345"},
	}}
	if _, err := r.Match(good); err != nil {
		t.Fatalf("headers should match case-insensitively: %v", err)
	}
	// missing header
	miss := Request{Method: "GET", URI: "http://h.test/", Headers: map[string][]string{
		"accept": {"application/json"},
	}}
	if _, err := r.Match(miss); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("missing header should not match: %v", err)
	}
	// wrong value
	bad := Request{Method: "GET", URI: "http://h.test/", Headers: map[string][]string{
		"accept":  {"text/html"},
		"x-trace": {"12"},
	}}
	if _, err := r.Match(bad); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("wrong header value should not match: %v", err)
	}
}

func TestHeadersOptionMultiValue(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "hv.test").With(Headers(map[string]string{"A": "1", "B": "2"}))
	good := Request{Method: "GET", URI: "http://hv.test/", Headers: map[string][]string{
		"a": {"x", "1"}, // second value matches
		"b": {"2"},
	}}
	if _, err := r.Match(good); err != nil {
		t.Fatalf("multi-value header: %v", err)
	}
}

func TestBodyMatchers(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("POST", "b.test").With(Body("exact-body"))
	if _, err := r.Match(Request{Method: "POST", URI: "http://b.test/", Body: "exact-body"}); err != nil {
		t.Fatalf("exact body: %v", err)
	}
	if _, err := r.Match(Request{Method: "POST", URI: "http://b.test/", Body: "nope"}); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("wrong exact body should miss: %v", err)
	}

	r2 := NewRegistry()
	r2.AllowNetConnect()
	r2.StubRequest("POST", "b2.test").With(BodyRe(regexp.MustCompile(`id=\d+`)))
	if _, err := r2.Match(Request{Method: "POST", URI: "http://b2.test/", Body: "id=42"}); err != nil {
		t.Fatalf("regex body: %v", err)
	}
	if _, err := r2.Match(Request{Method: "POST", URI: "http://b2.test/", Body: "id=x"}); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("regex body miss: %v", err)
	}
}

func TestBodyFormAndJSON(t *testing.T) {
	// form-encoded
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("POST", "f.test").With(BodyForm(map[string]string{"a": "1", "b": "2"}))
	if _, err := r.Match(Request{Method: "POST", URI: "http://f.test/", Body: "a=1&b=2"}); err != nil {
		t.Fatalf("form body: %v", err)
	}
	// wrong value
	if _, err := r.Match(Request{Method: "POST", URI: "http://f.test/", Body: "a=1&b=9"}); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("form value mismatch: %v", err)
	}
	// wrong length
	if _, err := r.Match(Request{Method: "POST", URI: "http://f.test/", Body: "a=1"}); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("form length mismatch: %v", err)
	}
	// bad form encoding
	if _, err := r.Match(Request{Method: "POST", URI: "http://f.test/", Body: "%zz"}); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("bad form: %v", err)
	}

	// JSON body via content-type
	rj := NewRegistry()
	rj.AllowNetConnect()
	rj.StubRequest("POST", "j.test").With(BodyForm(map[string]string{"n": "5", "s": "hi"}))
	jgood := Request{Method: "POST", URI: "http://j.test/", Body: `{"n":5,"s":"hi"}`,
		Headers: map[string][]string{"Content-Type": {"application/json"}}}
	if _, err := rj.Match(jgood); err != nil {
		t.Fatalf("json body: %v", err)
	}
	jbad := Request{Method: "POST", URI: "http://j.test/", Body: `{not json`,
		Headers: map[string][]string{"Content-Type": {"application/json"}}}
	if _, err := rj.Match(jbad); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("bad json: %v", err)
	}
}

func TestBodyMatcherDefaultFalse(t *testing.T) {
	// A body matcher marked set but with no shape never matches.
	bm := bodyMatcher{set: true}
	if bm.match(Request{Body: "anything"}) {
		t.Fatal("empty body matcher should not match")
	}
	if bm.describe() != "nil" {
		t.Fatalf("describe: %q", bm.describe())
	}
}

func TestQueryMatchers(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("GET", "q.test").With(Query(map[string]string{"a": "1", "b": "2"}))
	if _, err := r.Match(req("GET", "http://q.test/?b=2&a=1")); err != nil {
		t.Fatalf("order-insensitive query: %v", err)
	}
	if _, err := r.Match(req("GET", "http://q.test/?a=1")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("query mismatch: %v", err)
	}
	// bad request URI in query matcher
	if _, err := r.Match(req("GET", "http://q.test/?a=1&b=2&c=3")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("extra query key: %v", err)
	}
}

func TestQueryValuesRepeatedKeys(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("GET", "qv.test").With(QueryValues(url.Values{"x": {"1", "2"}}))
	if _, err := r.Match(req("GET", "http://qv.test/?x=2&x=1")); err != nil {
		t.Fatalf("repeated query keys: %v", err)
	}
	// differing multiplicity
	if _, err := r.Match(req("GET", "http://qv.test/?x=1")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("query multiplicity: %v", err)
	}
	// value differs
	if _, err := r.Match(req("GET", "http://qv.test/?x=1&x=3")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("query value: %v", err)
	}
}

func TestQueryMatcherBadURI(t *testing.T) {
	qm := queryMatcher{set: true, values: url.Values{"a": {"1"}}}
	if qm.match("http://[::1") {
		t.Fatal("bad uri should not match")
	}
}

func TestURIEmbeddedQuery(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("GET", "http://eq.test/?a=1&b=2")
	if _, err := r.Match(req("GET", "http://eq.test/?b=2&a=1")); err != nil {
		t.Fatalf("embedded query order-insensitive: %v", err)
	}
	if _, err := r.Match(req("GET", "http://eq.test/?a=1")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("embedded query mismatch: %v", err)
	}
}

func TestURIPathAndPortAndScheme(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	// explicit scheme + path
	r.StubRequest("GET", "https://p.test/a/b")
	if _, err := r.Match(req("GET", "https://p.test/a/b")); err != nil {
		t.Fatalf("explicit scheme+path: %v", err)
	}
	// scheme mismatch
	if _, err := r.Match(req("GET", "http://p.test/a/b")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("scheme mismatch should miss: %v", err)
	}
	// path mismatch
	if _, err := r.Match(req("GET", "https://p.test/a/c")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("path mismatch: %v", err)
	}
	// host mismatch
	if _, err := r.Match(req("GET", "https://other.test/a/b")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("host mismatch: %v", err)
	}
}

func TestURIPortNormalization(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("GET", "http://port.test:80/")
	if _, err := r.Match(req("GET", "http://port.test/")); err != nil {
		t.Fatalf(":80 == default: %v", err)
	}
	// explicit non-default port required
	r2 := NewRegistry()
	r2.AllowNetConnect()
	r2.StubRequest("GET", "http://port.test:8080/")
	if _, err := r2.Match(req("GET", "http://port.test/")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("port mismatch: %v", err)
	}
	// ws/wss default ports through defaultPort
	if defaultPort("ws", "") != "80" || defaultPort("wss", "") != "443" || defaultPort("ftp", "") != "0" {
		t.Fatal("defaultPort scheme table")
	}
}

func TestURIRootPathDefaulting(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("GET", "http://root.test") // no path -> "/"
	// request with empty path (rare) still normalizes to "/"
	if _, err := r.Match(req("GET", "http://root.test")); err != nil {
		t.Fatalf("root path default: %v", err)
	}
}

func TestURIMalformedPatternMatchesNothing(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("GET", "http://%zz") // parse error
	if _, err := r.Match(req("GET", "http://whatever/")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("malformed pattern must not match: %v", err)
	}
	p := newStringURI("http://%zz")
	if p.parseErr == nil {
		t.Fatal("expected parse error")
	}
	if p.describe() != `"http://%zz"` {
		t.Fatalf("describe malformed: %q", p.describe())
	}
}

func TestURIMatchBadRequestURI(t *testing.T) {
	p := newStringURI("host.test")
	if p.match("://bad uri") {
		t.Fatal("bad request uri should not match")
	}
}

func TestRegexURIDescribe(t *testing.T) {
	s := NewStubRe("GET", regexp.MustCompile(`ex.*mple`))
	if got := s.m.uri.describe(); got != "/ex.*mple/" {
		t.Fatalf("regex describe: %q", got)
	}
}

func TestBlockMatcher(t *testing.T) {
	r := NewRegistry()
	r.AllowNetConnect()
	r.StubRequest("GET", "blk.test").With(Block(func(rq Request) bool {
		return strings.HasPrefix(rq.Body, "ok")
	}))
	if _, err := r.Match(Request{Method: "GET", URI: "http://blk.test/", Body: "ok!"}); err != nil {
		t.Fatalf("block match: %v", err)
	}
	if _, err := r.Match(Request{Method: "GET", URI: "http://blk.test/", Body: "no"}); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("block reject: %v", err)
	}
}

func TestNoStubErrorDiff(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "http://reg.test/x").With(Header("Accept", "text/plain"))
	badReq := Request{
		Method:  "POST",
		URI:     "http://miss.test/y",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    "payload",
	}
	_, err := r.Match(badReq)
	var ns *NoStubError
	if !errors.As(err, &ns) {
		t.Fatalf("want NoStubError, got %T %v", err, err)
	}
	msg := err.Error()
	for _, want := range []string{
		"real HTTP connections are disabled",
		"POST http://miss.test/y",
		`with headers {"Content-Type"=>"application/json"}`,
		`with body "payload"`,
		"You can stub this request",
		`stub_request(:post, "http://miss.test/y")`,
		"registered request stubs:",
		`stub_request(:get, "http://reg.test/x").with(headers: {"accept"=>"text/plain"})`,
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("diff missing %q in:\n%s", want, msg)
		}
	}
}

func TestNoStubErrorNoRegistered(t *testing.T) {
	r := NewRegistry()
	_, err := r.Match(req("GET", "http://x.test/"))
	if !strings.Contains(err.Error(), "No stubs are registered.") {
		t.Fatalf("want empty-registry note: %v", err)
	}
}

func TestSuggestSnippetAnyMethod(t *testing.T) {
	got := suggestSnippet(Request{Method: "", URI: "http://x/"})
	if !strings.Contains(got, "stub_request(:any,") {
		t.Fatalf("empty method snippet: %q", got)
	}
}

func TestDescribeAllConstraints(t *testing.T) {
	m := newMatcher("GET", newStringURI("http://d.test/"), []Option{
		Header("A", "1"),
		Body("bod"),
		Query(map[string]string{"q": "v"}),
		Block(func(Request) bool { return true }),
	})
	got := m.describe()
	for _, want := range []string{
		`headers: {"a"=>"1"}`,
		`body: "bod"`,
		`query: {"q"=>"v"}`,
		"block: <func>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("describe missing %q: %s", want, got)
		}
	}
	// header + body regexp describe branches
	m2 := newMatcher("GET", newStringURI("d.test"), []Option{
		HeaderRe("R", regexp.MustCompile("abc")),
		BodyRe(regexp.MustCompile("xyz")),
	})
	g2 := m2.describe()
	if !strings.Contains(g2, "/abc/") || !strings.Contains(g2, "/xyz/") {
		t.Fatalf("regex describe: %s", g2)
	}
	// body-form describe branch
	m3 := newMatcher("GET", newStringURI("d.test"), []Option{BodyForm(map[string]string{"k": "v"})})
	if !strings.Contains(m3.describe(), `body: {"k"=>"v"}`) {
		t.Fatalf("form describe: %s", m3.describe())
	}
}

func TestDescribeAnyMethod(t *testing.T) {
	m := newMatcher("", newStringURI("http://d.test/"), nil)
	if got := m.describe(); !strings.HasPrefix(got, "stub_request(:any,") {
		t.Fatalf("any-method describe: %q", got)
	}
}

func TestValueMatcherDescribe(t *testing.T) {
	if (valueMatcher{exact: "e"}).describe() != `"e"` {
		t.Fatal("exact describe")
	}
	if (valueMatcher{re: regexp.MustCompile("r")}).describe() != "/r/" {
		t.Fatal("re describe")
	}
}

func TestNetConnectToggle(t *testing.T) {
	r := NewRegistry()
	// default disabled -> diff
	if _, err := r.Match(req("GET", "http://n.test/")); err == nil {
		t.Fatal("default should be disabled")
	}
	r.AllowNetConnect()
	if _, err := r.Match(req("GET", "http://n.test/")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("allow: %v", err)
	}
	r.DisableNetConnect()
	if _, err := r.Match(req("GET", "http://n.test/")); errors.Is(err, ErrNetConnectAllowed) {
		t.Fatal("disable should stop passthrough")
	}
}

func TestAssertRequestedAndCounts(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "c.test")
	r.Match(req("GET", "http://c.test/"))
	r.Match(req("GET", "http://c.test/"))

	if err := r.AssertRequested("GET", "c.test", 2); err != nil {
		t.Fatalf("count 2: %v", err)
	}
	err := r.AssertRequested("GET", "c.test", 5)
	var ae *AssertionError
	if !errors.As(err, &ae) {
		t.Fatalf("want AssertionError, got %v", err)
	}
	if ae.Expected != 5 || ae.Got != 2 {
		t.Fatalf("assertion fields: %+v", ae)
	}
	if !strings.Contains(ae.Error(), "requested 5 time(s) but it was requested 2 time(s)") {
		t.Fatalf("assertion msg: %s", ae.Error())
	}
	// with constraints that don't match -> 0
	if got := r.Requested("GET", "c.test", Header("Missing", "x")); got != 0 {
		t.Fatalf("constrained count: %d", got)
	}
}

func TestRequestedReAndRequestsCopy(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "hist.test")
	r.Match(req("GET", "http://hist.test/"))
	if got := r.RequestedRe("GET", regexp.MustCompile(`hist`)); got != 1 {
		t.Fatalf("regex count: %d", got)
	}
	h := r.Requests()
	if len(h) != 1 || h[0].URI != "http://hist.test/" {
		t.Fatalf("history: %+v", h)
	}
	h[0].URI = "mutated" // must not affect internal state
	if r.Requests()[0].URI != "http://hist.test/" {
		t.Fatal("Requests should return a copy")
	}
}

func TestReset(t *testing.T) {
	r := NewRegistry()
	r.StubRequest("GET", "z.test")
	r.Match(req("GET", "http://z.test/"))
	r.Reset()
	if len(r.Requests()) != 0 {
		t.Fatal("history not cleared")
	}
	// after reset, previously-stubbed request no longer matches
	if _, err := r.Match(req("GET", "http://z.test/")); err == nil {
		t.Fatal("stubs not cleared")
	}
}

func TestRegisterReturnsStub(t *testing.T) {
	r := NewRegistry()
	s := NewStub("GET", "ret.test").ToReturn(StubResponse{Body: "ok"})
	if r.Register(s) != s {
		t.Fatal("Register should return the stub")
	}
	resp, err := r.Match(req("GET", "http://ret.test/"))
	if err != nil || resp.Body != "ok" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestPackageLevelHelpers(t *testing.T) {
	Reset()
	defer Reset()
	StubRequest("GET", "pkg.test").With(Header("A", "1")).ToReturn(StubResponse{Body: "pkg"})
	StubRequestRe("GET", regexp.MustCompile(`pkgre`)).ToReturn(StubResponse{Body: "pkgre"})

	good := Request{Method: "GET", URI: "http://pkg.test/", Headers: map[string][]string{"a": {"1"}}}
	resp, err := Match(good)
	if err != nil || resp.Body != "pkg" {
		t.Fatalf("pkg match: resp=%+v err=%v", resp, err)
	}
	if resp, err := Match(req("GET", "http://pkgre/x")); err != nil || resp.Body != "pkgre" {
		t.Fatalf("pkg regex: resp=%+v err=%v", resp, err)
	}
	if Requested("GET", "pkg.test", Header("A", "1")) != 1 {
		t.Fatal("pkg Requested")
	}
	if err := AssertRequested("GET", "pkg.test", 1, Header("A", "1")); err != nil {
		t.Fatalf("pkg AssertRequested: %v", err)
	}
	AllowNetConnect()
	if _, err := Match(req("GET", "http://unstubbed.test/")); !errors.Is(err, ErrNetConnectAllowed) {
		t.Fatalf("pkg allow: %v", err)
	}
	DisableNetConnect()
	if _, err := Match(req("GET", "http://unstubbed.test/")); errors.Is(err, ErrNetConnectAllowed) {
		t.Fatal("pkg disable")
	}
}
