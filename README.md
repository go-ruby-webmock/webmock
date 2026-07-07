<p align="center"><img src="https://go-ruby-webmock.github.io/logo.png" alt="go-ruby-webmock/webmock" width="720"></p>

# webmock — go-ruby-webmock

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-webmock.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the deterministic core of Ruby's
[`webmock`](https://github.com/bblimke/webmock) gem** — an HTTP **request
matcher**, **stub registry**, and **recorded-request history** — **without any
Ruby runtime**.

It is the webmock engine for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a **standalone,
reusable** module — a sibling of
[go-ruby-openbao](https://github.com/go-ruby-openbao/openbao) and
[go-ruby-net-http](https://github.com/go-ruby-net-http/net-http).

> **What it is — and isn't.** Everything the `webmock` gem does *around* the wire
> is deterministic and needs **no interpreter**, so it lives here as pure Go:
> deciding whether a request matches a stub (method, URI incl. query, headers,
> body — exact / regexp / hash / proc), picking the stubbed response (including
> sequential responses, `to_raise`, `to_timeout`), counting requests for
> `assert_requested` / `have_requested`, and refusing unstubbed traffic when the
> network is disabled. **Intercepting the real transport is the host's job**: rbgo
> swaps in its `Net::HTTP` and feeds every outgoing request through
> [`Registry.Match`]. This engine opens no socket of its own.

## Features

Faithful port of the `webmock` gem's matching core:

- **Stub builder** — `StubRequest(method, uri)` (`:any` via `""`/`"ANY"`) and
  `StubRequestRe(method, *regexp.Regexp)` for a regexp URI, chained with
  `.With(...)`, `.ToReturn(...)`, `.ToRaise(err)`, `.ToTimeout()`.
- **`.with(...)` constraints** — `Header`/`HeaderRe`/`Headers`,
  `Body`/`BodyRe`/`BodyForm` (hash body decoded from form or JSON per
  `Content-Type`), `Query`/`QueryValues` (order-insensitive), and `Block` (a
  `func(Request) bool` predicate, the gem's `with { |req| … }`).
- **Faithful URI semantics** — scheme-agnostic unless the stub gives a scheme,
  default-port normalization (`:80`/`:443`), root-path defaulting, and
  order-insensitive query matching. Header names match case-insensitively.
- **Responses** — `to_return(status:, body:, headers:)` with **sequential
  responses** (the last repeats once exhausted), plus `to_raise` (`*RaiseError`,
  `errors.Unwrap`-able) and `to_timeout` (`ErrTimeout`). Zero status defaults to
  200; a stub with no response answers an empty `200`.
- **Registry & history** — `Register`/`StubRequest`, `Match(request)` (the most
  recently registered matching stub wins), a recorded `Requests()` history, and
  `Reset()`.
- **Assertions** — `Requested(method, uri, …)` / `RequestedRe(…)` counts and
  `AssertRequested(method, uri, times, …)` returning an `*AssertionError`,
  mirroring `assert_requested` / `have_requested`.
- **Net-connect policy** — `DisableNetConnect()` (the default) makes an
  unmatched request return a webmock-style diff (`*NoStubError`);
  `AllowNetConnect()` returns `ErrNetConnectAllowed` so the host performs the
  real request.
- **Package-level `Default`** — `webmock.StubRequest` / `Match` /
  `AssertRequested` / `DisableNetConnect` mirror the global `WebMock` module.

CGO-free, dependency-free (stdlib only), **100% test coverage**, `gofmt` +
`go vet` clean, and green across the six 64-bit Go targets (amd64, arm64,
riscv64, loong64, ppc64le, **s390x** — big-endian) plus `js/wasm` and
`wasip1/wasm`.

## Install

```sh
go get github.com/go-ruby-webmock/webmock
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-webmock/webmock"
)

func main() {
	reg := webmock.NewRegistry() // net connections disabled by default

	reg.StubRequest("GET", "www.example.com").
		With(webmock.Header("Accept", "application/json")).
		ToReturn(webmock.StubResponse{Status: 200, Body: `{"ok":true}`})

	resp, err := reg.Match(webmock.Request{
		Method:  "GET",
		URI:     "http://www.example.com/",
		Headers: map[string][]string{"Accept": {"application/json"}},
	})
	if err != nil {
		// *webmock.NoStubError (diff), webmock.ErrTimeout,
		// *webmock.RaiseError, or webmock.ErrNetConnectAllowed.
		return
	}
	fmt.Println(resp.Status, resp.Body) // 200 {"ok":true}

	// Assert how many times it was hit.
	_ = reg.AssertRequested("GET", "www.example.com", 1,
		webmock.Header("Accept", "application/json"))
}
```

### Sequential responses, raise, timeout

```go
reg.StubRequest("GET", "flaky.test").
	ToReturn(webmock.StubResponse{Body: "first"}, webmock.StubResponse{Body: "second"}).
	ToRaise(errors.New("boom")) // then this; the last behaviour repeats
```

## Value model

| gem                                                | this package                                        |
| -------------------------------------------------- | --------------------------------------------------- |
| `stub_request(:get, "host")`                       | `reg.StubRequest("GET", "host")`                    |
| `stub_request(:any, /re/)`                          | `reg.StubRequestRe("", regexp.MustCompile("re"))`   |
| `.with(headers:, body:, query:) { \|r\| … }`        | `.With(Header/Body/Query/Block(…))`                 |
| `.to_return(status:, body:, headers:)`             | `.ToReturn(StubResponse{Status, Body, Headers})`    |
| `.to_return(a, b, …)` (sequential)                 | `.ToReturn(a, b, …)` (last repeats)                 |
| `.to_raise(Err)` / `.to_timeout`                   | `.ToRaise(err)` → `*RaiseError` / `.ToTimeout()` → `ErrTimeout` |
| `assert_requested(:get, uri, times: n)`            | `reg.AssertRequested("GET", uri, n, …)`             |
| `WebMock.disable_net_connect!` / `allow_net_connect!` | `reg.DisableNetConnect()` / `reg.AllowNetConnect()` |
| unregistered-request error                         | `*NoStubError` (webmock-style diff)                 |
| the intercepted `Net::HTTP`                         | the host feeds `Request` into `Registry.Match`      |

## Tests & coverage

The suite is deterministic and needs no network: it constructs `Request` values
and asserts the `Match` outcome and recorded history directly. Every branch —
no-match diff, sequential-response exhaustion, `raise` / `timeout` stubs,
assertion-count mismatch, and the net-connect-disabled path — is exercised, so
every lane (three host OSes, six arches under qemu, both wasm targets) holds
coverage at **100%**.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-webmock/webmock authors.
