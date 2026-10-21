<div align="center">

# tspdnative-go

**Solve F5 TSPD challenges in pure Go — no JavaScript engine, no cgo, no dependencies.**

[![Go Reference](https://pkg.go.dev/badge/github.com/aizen-sosuke218/tspdnative-go.svg)](https://pkg.go.dev/github.com/aizen-sosuke218/tspdnative-go)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen)](go.mod)
[![CGO](https://img.shields.io/badge/cgo-disabled-brightgreen)](https://go.dev/wiki/cgo)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) · [Tiếng Việt](README.vi.md)

</div>

---

`tspdnative-go` is a Go library and CLI that obtains a valid TSPD session from
an F5-protected site. It fetches the challenge page, reconstructs the sensor's
answer cookie natively (AES-CBC envelope + custom digest/MAC), and returns the
cleared session cookie — typically `TSPD_101`.

No headless browser, no embedded V8, no JavaScript execution. The whole
pipeline is deterministic Go code.

## Table of contents

- [Why this library](#why-this-library)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Results](#results)
- [Error handling](#error-handling)
- [Proxy support](#proxy-support)
- [CLI](#cli)
- [How it works](#how-it-works)
- [Project layout](#project-layout)
- [Testing](#testing)
- [Benchmarks](#benchmarks)
- [Design notes and limitations](#design-notes-and-limitations)
- [C# to Go mapping](#c-to-go-mapping)
- [License](#license)

## Why this library

Browser automation is heavyweight: a browser download, a driver, a display
server, and seconds of startup time just to execute a few kilobytes of
obfuscated sensor code. `tspdnative-go` replaces that with a small HTTP client
and a native implementation of the challenge crypto.

It is aimed at services and scripts that need a fresh edge cookie on demand:

- API workers that cannot ship a browser runtime;
- crawlers and monitors that need low, predictable latency;
- environments where cgo and external binaries are not allowed.

> **Responsible use.** Use this project only for systems you own or are
> authorized to access, and comply with the target site's terms of service and
> applicable law. You are responsible for how you deploy it.

## Features

- **Pure Go, standard library only** — zero third-party modules, no cgo.
- **No JavaScript engine** — the challenge is solved by native crypto and
  parsing, not by evaluating scripts.
- **Small HTTP footprint** — two page requests plus the referenced sensor
  script(s); no browser assets or subresources.
- **Reusable sessions** — `Solver` keeps HTTP/TLS connections warm across
  solves and is safe for concurrent use.
- **Flexible proxies** — `http`, `https`, `socks5`, and `socks5h`, with support
  for common provider formats such as `host:port:user:pass`.
- **Idiomatic API** — `context.Context`, `log/slog`, typed sentinel errors via
  `*SolveError`, and an `Options` zero value that already works.
- **Tested against the real protocol** — captured fixtures are rebuilt
  byte-for-byte, and a mock F5 edge validates the answer cryptographically
  before issuing `TSPD_101`.

## Requirements

- Go **1.22** or newer.
- Network access to the target site.
- No C toolchain, no Node.js, no browser.

## Installation

```bash
go get github.com/aizen-sosuke218/tspdnative-go@latest
```

Import it as:

```go
import "github.com/aizen-sosuke218/tspdnative-go"
```

The public API lives in the root package `tspdnative`. Protocol internals are
kept under `internal/` and are not part of the compatibility contract.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aizen-sosuke218/tspdnative-go"
)

func main() {
	result, err := tspdnative.Solve(context.Background(), tspdnative.Options{
		// Empty means a direct connection.
		// ProxyFromEnv() opts into HTTPS_PROXY / HTTP_PROXY.
		Proxy: tspdnative.ProxyFromEnv(),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Solved)       // true when TSPD_101 was issued
	fmt.Println(result.CookieString) // TSPD_101=...; TS...=...
}
```

The zero value of `Options` targets the default immigration page with the
built-in Chrome profile, so `tspdnative.Solve(ctx, tspdnative.Options{})` is
valid.

### Reusing a connection

Create one `Solver` and call `Solve` repeatedly. HTTP/2 and TLS sessions are
reused, which removes handshake cost from subsequent runs.

```go
solver, err := tspdnative.NewSolver(tspdnative.Options{
	Proxy:   tspdnative.ProxyFromEnv(),
	Timeout: 20 * time.Second,
})
if err != nil {
	return err
}
defer solver.Close()

for range 5 {
	result, err := solver.Solve(ctx)
	if err != nil {
		return err
	}
	use(result.CookieString)
}
```

`Solver` is safe for concurrent use. Each `Solve` call owns its cookie jar.

## Configuration

```go
type Options struct {
	Target         string
	UserAgent      string
	Language       string
	AcceptLanguage string
	Proxy          string
	Timeout        time.Duration
	Debug          bool
	Logger         *slog.Logger
}
```

| Field | Default | Description |
| --- | --- | --- |
| `Target` | `DefaultTarget` | Challenged page to solve. |
| `UserAgent` | Chrome 126 / Windows | Sent as `User-Agent` and written to fingerprint field 11. |
| `Language` | `en-NZ` | Locale written to fingerprint field 10. |
| `AcceptLanguage` | derived from `Language` | Overrides the `Accept-Language` header. |
| `Proxy` | empty (direct) | Proxy URL or provider-style string. |
| `Timeout` | `25s` | Bounds the entire solve, including script downloads. |
| `Debug` | `false` | Enables debug logging when `Logger` is nil. |
| `Logger` | `nil` | Receives structured pipeline events. |

Exported defaults are available as `DefaultTarget`, `DefaultUserAgent`,
`DefaultLanguage`, and `DefaultAcceptLanguage`.

## Results

```go
type Result struct {
	Solved           bool
	HadChallenge     bool
	CookieString     string
	TSPD101          string
	AnswerCookieName string
	HTMLLength       int
	HTML             string
	Cookies          []Cookie
}
```

| Field | Meaning |
| --- | --- |
| `Solved` | `true` when the edge issued `TSPD_101` after the answer cookie. |
| `HadChallenge` | `true` when the first response contained a TSPD challenge. |
| `CookieString` | Full `Cookie` header value in insertion order. |
| `TSPD101` | Cleared cookie value when `Solved` is true. |
| `AnswerCookieName` | Natively built answer cookie name, e.g. `TSaa03129d075`. |
| `HTMLLength` | Byte length of `HTML`. |
| `HTML` | Second response body, or the first body when not challenged. |
| `Cookies` | Snapshot of the session jar in insertion order. |

A page that is not challenged is not an error: `Solve` returns successfully
with `Solved == false` and `HadChallenge == false`, plus any session cookies it
received.

## Error handling

Every expected failure is a `*tspdnative.SolveError`:

```go
var solveErr *tspdnative.SolveError
if errors.As(err, &solveErr) {
	switch solveErr.Kind {
	case tspdnative.KindNetwork:
		// DNS, TLS, connection reset, or overall timeout.
	case tspdnative.KindProxy:
		// Invalid proxy URL or unsupported scheme.
	case tspdnative.KindGetFailed:
		// First page request returned a non-2xx status.
	case tspdnative.KindNoAnswer:
		// Challenge or sensor data could not produce an answer cookie.
	case tspdnative.KindNoTSPD:
		// Edge rejected the answer cookie; no TSPD_101 was issued.
	}
	log.Printf("status=%d cause=%v", solveErr.HTTPStatus, solveErr.Err)
}
```

| Kind | Typical cause |
| --- | --- |
| `KindNetwork` | DNS failure, TLS error, timeout, canceled context. |
| `KindProxy` | Malformed proxy string or unsupported scheme. |
| `KindGetFailed` | Target returned 403/404/5xx on the first request. |
| `KindNoAnswer` | `bobcmn` layout changed or sensor key could not be located. |
| `KindNoTSPD` | F5 build or fingerprint parameters changed. |

Use `errors.Is(err, context.DeadlineExceeded)` for timeout-specific handling;
the timeout is wrapped inside `KindNetwork`.

## Proxy support

Set `Options.Proxy` directly, or call `tspdnative.ProxyFromEnv()` to read
`HTTPS_PROXY`, `https_proxy`, `HTTP_PROXY`, or `http_proxy` (in that order).
The library never reads the environment implicitly.

Accepted formats:

| Input | Normalized to |
| --- | --- |
| `http://user:pass@host:8080` | unchanged |
| `socks5://host:1080` | unchanged |
| `socks5h://host:1080` | unchanged |
| `host:8080:user:pass` | `http://user:pass@host:8080` |
| `user:pass@host:8080` | `http://user:pass@host:8080` |
| `host:8080` | `http://host:8080` |

`NormalizeProxy` is exported if you need the same parsing elsewhere.
`socks5h` resolves DNS through the proxy; `socks5` resolves locally.

## CLI

### Install

```bash
go install github.com/aizen-sosuke218/tspdnative-go/cmd/tspdctl@latest
```

Or build from a checkout:

```bash
make build          # -> bin/tspdctl
```

### Usage

```text
tspdctl [solve] [flags]
tspdctl <proxy>
tspdctl bench-answer [--iters N]
```

```bash
# Solve with a direct connection
tspdctl

# Proxy from the environment, or explicitly
tspdctl --proxy "$HTTPS_PROXY"
tspdctl --proxy host:8080:user:pass
tspdctl user:pass@host:8080

# Machine-readable output
tspdctl --json

# Five warm solves in one process
tspdctl --repeat 5

# CPU-only benchmark of the answer builder
tspdctl bench-answer --iters 100000
```

| Flag | Default | Description |
| --- | --- | --- |
| `--proxy` | env, then direct | Proxy URL or provider-style string. |
| `--url` | immigration host | Target page. |
| `--ua` | Chrome 126 / Windows | User agent. |
| `--lang` | `en-NZ` | Fingerprint locale. |
| `--json` | `false` | Print a JSON object instead of the cookie string. |
| `--repeat` | `1` | Solve N times with a warm connection. |
| `--iters` | `20000` | Iterations for `bench-answer`. |
| `--debug` | `false` | Log pipeline stages to stderr. |

**Streams and exit codes**

- `stdout` — cookie string or JSON; nothing else.
- `stderr` — status, diagnostics, debug logs, and errors.
- `0` — `TSPD_101` was issued.
- `1` — solve failed or the page was not challenged.
- `2` — invalid usage.

Example JSON:

```json
{
  "solved": true,
  "cookie": "TSPD_101=...; TSaa03129d075=...",
  "tspd101": "...",
  "answerCookieName": "TSaa03129d075",
  "hadChallenge": true,
  "cookies": 2,
  "elapsedMs": 1987
}
```

## How it works

```text
GET /                              challenge page: bobcmn + hex challenge
  │
  ├─ parse bobcmn TLV              per-build configuration
  ├─ GET /TSPD/<id>?type=10        sensor script(s), fetched concurrently
  │    └─ locate key literal
  │
  ├─ unseal challenge              verify MAC, read 4-byte context
  ├─ read shortcut message         scope AB → 48-byte context
  ├─ seal fingerprint (159 bytes)  scope 04, challenge context
  └─ assemble answer cookie        TS<id8><075>=0502c101...
  │
GET / + answer cookie              edge verifies → Set-Cookie: TSPD_101
```

1. Fetch the target page and extract the `bobcmn` configuration and challenge
   blob.
2. Discover and download external sensor scripts, then locate the per-build
   key in the script source.
3. Rebuild the answer cookie offline: unseal the challenge, recover the
   contexts, seal the browser fingerprint, and format the cookie value.
4. Request the page again with the answer cookie. The edge responds with
   `TSPD_101` when the answer is valid.

The crypto layer implements the same envelope, digest, and MAC routines as the
original C# implementation, including the byte-level layout of the answer.
See [`internal/securemsg`](internal/securemsg) for the primitives and
[`internal/answer`](internal/answer) for the assembly logic.

## Project layout

```text
tspdnative-go/
├── doc.go                  package documentation
├── solver.go               Solver, Solve, request pipeline
├── options.go              Options and defaulting
├── result.go               Result, Cookie
├── errors.go               ErrorKind, SolveError
├── session.go              keep-alive HTTP session
├── headers.go              browser headers, UA/locale helpers
├── proxy.go                proxy normalization and env lookup
├── jar.go                  ordered cookie jar
├── internal/
│   ├── securemsg/          cipher, CBC, digest/MAC, seal/unseal
│   ├── challenge/          bobcmn TLV parser
│   ├── discovery/          HTML script scanner and locators
│   ├── answer/             fingerprint and answer cookie builder
│   └── testsupport/        synthetic fixtures for tests
├── cmd/tspdctl/            command-line interface
├── testdata/               captured and live protocol fixtures
└── Makefile
```

Only the root package is public API. Everything under `internal/` may change
without notice.

## Testing

```bash
make test     # unit tests + mock edge end-to-end
make race     # go test -race ./...
make vet      # go vet ./...
make bench    # crypto and builder benchmarks
```

Fixtures in [`testdata/`](testdata) were captured from real sessions:

- `captured_challenge.json` — challenge unseal/seal vectors;
- `live_answer_main_out3.json`, `live_answer_bench_4.json` — real answer
  cookies, rebuilt byte-for-byte by the Go code.

The end-to-end test starts a local mock F5 edge. It only returns `TSPD_101`
when the submitted answer cookie opens correctly and matches the expected
fingerprint, so the test exercises the full crypto path rather than stubbing
it. Failure branches (`KindNoAnswer`, `KindNoTSPD`, unchallenged pages, proxy
errors, cancellation) are covered as well.

## Benchmarks

The answer builder is CPU-only and independent of network latency:

```text
BenchmarkBuild-16     ~12 µs/op    ~5.8 KB/op    ~28 allocs/op
```

A live solve on the default target typically completes in about **2 seconds**,
dominated by the two page requests and one sensor script download.

## Design notes and limitations

- **Fixed protocol scope.** The parser and constants target the `bobcmn`
  layout used by `onlineservices.immigration.govt.nz`. Other TSPD deployments
  may use different parameters and are intentionally out of scope.
- **Build-sensitive.** Fingerprint fields and answer constants match the
  current sensor build. When F5 rotates the build, `Solve` fails with
  `KindNoTSPD` instead of returning a stale cookie.
- **No environment surprise.** Proxy variables are read only when you call
  `ProxyFromEnv`; the CLI applies that fallback by default.
- **Not a browser.** The fingerprint is a fixed profile matching the configured
  user agent and locale. It is not a general browser-fingerprint emulator.
- **Ethical use.** This is not a CAPTCHA bypass. It reproduces the client-side
  proof-of-work that a browser performs, for authorized automation.

## C# to Go mapping

This library is a Go port of the original `TSPDNative` C# implementation.

| C# | Go |
| --- | --- |
| `TspdCrypto` | `internal/securemsg` |
| `TspdBobcmn` | `internal/challenge.Config` |
| `TspdLocator` + `HtmlScripts` | `internal/discovery` |
| `TspdCookie` | `internal/answer` |
| `Jar` | `jar.go` (`cookieJar`) |
| `HttpSession` + `ProxyUtil` | `session.go` + `proxy.go` |
| `Solver` / `TspdSolveOptions` / `TspdSolveResult` | `Solver` / `Options` / `Result` |
| `TspdNativeApp` | `cmd/tspdctl` |

## License

Released under the [MIT License](LICENSE).

<div align="center">

**[Back to top](#tspdnative-go)**

</div>
