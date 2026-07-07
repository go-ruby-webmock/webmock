// Copyright (c) the go-ruby-webmock/webmock authors
//
// SPDX-License-Identifier: BSD-3-Clause

package webmock

import "regexp"

// respKind is the behaviour of a single element in a stub's response sequence.
type respKind int

const (
	respReturn  respKind = iota // return a StubResponse
	respRaise                   // raise an error
	respTimeout                 // time out
)

// responder is one behaviour in a stub's (possibly sequential) response list.
type responder struct {
	kind respKind
	resp StubResponse
	err  error
}

// Stub is a registered request matcher paired with an ordered list of
// behaviours. Successive matches walk the list; once exhausted the last
// behaviour repeats, mirroring webmock's sequential to_return semantics.
type Stub struct {
	m         *matcher
	responses []responder
	pos       int
}

// NewStub builds an unregistered stub for method (":any" via "" or "ANY") and a
// string URI pattern, with optional .with(...) constraints. Register it on a
// [Registry] with [Registry.Register], or use [Registry.StubRequest] to do both.
func NewStub(method, uri string, opts ...Option) *Stub {
	return &Stub{m: newMatcher(method, newStringURI(uri), opts)}
}

// NewStubRe is like [NewStub] but matches the whole request URI against a
// regexp, mirroring stub_request(:any, /regex/).
func NewStubRe(method string, re *regexp.Regexp, opts ...Option) *Stub {
	m := newMatcher(method, &uriPattern{re: re}, opts)
	return &Stub{m: m}
}

// With appends further .with(...) constraints to the stub's matcher.
func (s *Stub) With(opts ...Option) *Stub {
	for _, o := range opts {
		o(s.m)
	}
	return s
}

// ToReturn appends one or more responses. Called with several responses (or
// chained) it builds a sequence returned on successive matches; called with
// none it appends a default empty 200. A zero Status defaults to 200.
func (s *Stub) ToReturn(resps ...StubResponse) *Stub {
	if len(resps) == 0 {
		s.responses = append(s.responses, responder{kind: respReturn, resp: StubResponse{Status: 200}})
		return s
	}
	for _, r := range resps {
		if r.Status == 0 {
			r.Status = 200
		}
		s.responses = append(s.responses, responder{kind: respReturn, resp: r})
	}
	return s
}

// ToRaise appends a behaviour that makes [Registry.Match] return a
// [*RaiseError] wrapping err, mirroring to_raise.
func (s *Stub) ToRaise(err error) *Stub {
	s.responses = append(s.responses, responder{kind: respRaise, err: err})
	return s
}

// ToTimeout appends a behaviour that makes [Registry.Match] return
// [ErrTimeout], mirroring to_timeout.
func (s *Stub) ToTimeout() *Stub {
	s.responses = append(s.responses, responder{kind: respTimeout})
	return s
}

// respond produces the next behaviour's outcome, advancing through the sequence
// and repeating the final element once exhausted.
func (s *Stub) respond() (StubResponse, error) {
	if len(s.responses) == 0 {
		return StubResponse{Status: 200}, nil
	}
	i := s.pos
	if s.pos < len(s.responses)-1 {
		s.pos++
	}
	r := s.responses[i]
	switch r.kind {
	case respRaise:
		return StubResponse{}, &RaiseError{Err: r.err}
	case respTimeout:
		return StubResponse{}, ErrTimeout
	default:
		return r.resp, nil
	}
}
