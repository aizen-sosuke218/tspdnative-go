<div align="center">

# tspdnative-go

**Giải challenge F5 TSPD bằng Go thuần — không JavaScript engine, không cgo, không dependency.**

[![Go Reference](https://pkg.go.dev/badge/github.com/aizen-sosuke218/tspdnative-go.svg)](https://pkg.go.dev/github.com/aizen-sosuke218/tspdnative-go)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen)](go.mod)
[![CGO](https://img.shields.io/badge/cgo-disabled-brightgreen)](https://go.dev/wiki/cgo)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) · [Tiếng Việt](README.vi.md)

</div>

---

`tspdnative-go` là thư viện Go kiêm CLI lấy session TSPD hợp lệ từ một site
được F5 bảo vệ. Thư viện tải trang challenge, tái tạo answer cookie của sensor
bằng crypto native (envelope AES-CBC + digest/MAC riêng), rồi trả về cookie đã
được edge chấp nhận — thường là `TSPD_101`.

Không headless browser, không nhúng V8, không chạy JavaScript. Toàn bộ pipeline
là code Go tất định.

## Mục lục

- [Vì sao dùng thư viện này](#vì-sao-dùng-thư-viện-này)
- [Tính năng](#tính-năng)
- [Yêu cầu](#yêu-cầu)
- [Cài đặt](#cài-đặt)
- [Bắt đầu nhanh](#bắt-đầu-nhanh)
- [Cấu hình](#cấu-hình)
- [Kết quả](#kết-quả)
- [Xử lý lỗi](#xử-lý-lỗi)
- [Hỗ trợ proxy](#hỗ-trợ-proxy)
- [CLI](#cli)
- [Cách hoạt động](#cách-hoạt-động)
- [Cấu trúc project](#cấu-trúc-project)
- [Kiểm thử](#kiểm-thử)
- [Benchmark](#benchmark)
- [Ghi chú thiết kế và giới hạn](#ghi-chú-thiết-kế-và-giới-hạn)
- [Ánh xạ C# sang Go](#ánh-xạ-c-sang-go)
- [Giấy phép](#giấy-phép)

## Vì sao dùng thư viện này

Browser automation rất nặng: tải browser, driver, display server và vài giây
khởi động chỉ để chạy một đoạn sensor bị obfuscate. `tspdnative-go` thay thế
bằng một HTTP client nhỏ và phần crypto native của challenge.

Phù hợp với service và script cần cookie edge mới theo yêu cầu:

- API worker không thể mang theo browser runtime;
- crawler và monitor cần latency thấp, ổn định;
- môi trường không cho phép cgo hoặc binary ngoài.

> **Sử dụng có trách nhiệm.** Chỉ dùng cho hệ thống bạn sở hữu hoặc được phép
> truy cập; tuân thủ điều khoản của site đích và pháp luật hiện hành. Bạn chịu
> trách nhiệm về cách triển khai.

## Tính năng

- **Go thuần, chỉ stdlib** — không module bên thứ ba, không cgo.
- **Không JavaScript engine** — challenge được giải bằng crypto và parser
  native, không eval script.
- **Ít request** — hai lượt tải trang cộng các sensor script được tham chiếu;
  không tải asset hay subresource của browser.
- **Tái sử dụng session** — `Solver` giữ connection HTTP/TLS ấm giữa các lần
  solve và an toàn khi gọi đồng thời.
- **Proxy linh hoạt** — `http`, `https`, `socks5`, `socks5h`, hỗ trợ các dạng
  `host:port:user:pass` và `user:pass@host:port`.
- **API hiện đại** — `context.Context`, `log/slog`, lỗi phân loại qua
  `*SolveError`; `Options` zero value dùng được ngay.
- **Test sát protocol thật** — fixture từ session thật được rebuild
  byte-for-byte; mock F5 edge kiểm tra crypto trước khi cấp `TSPD_101`.

## Yêu cầu

- Go **1.22** trở lên.
- Truy cập mạng tới site đích.
- Không cần C toolchain, Node.js hay browser.

## Cài đặt

```bash
go get github.com/aizen-sosuke218/tspdnative-go@latest
```

Import:

```go
import "github.com/aizen-sosuke218/tspdnative-go"
```

Public API nằm ở package gốc `tspdnative`. Các tầng protocol đặt trong
`internal/` và không thuộc cam kết tương thích.

## Bắt đầu nhanh

```go
result, err := tspdnative.Solve(context.Background(), tspdnative.Options{
	// Để trống nếu kết nối trực tiếp.
	// ProxyFromEnv() đọc HTTPS_PROXY / HTTP_PROXY.
	Proxy: tspdnative.ProxyFromEnv(),
})
if err != nil {
	log.Fatal(err)
}

fmt.Println(result.Solved)       // true khi edge đã cấp TSPD_101
fmt.Println(result.CookieString) // TSPD_101=...; TS...=...
```

`Options` zero value đã nhắm tới trang immigration mặc định với profile Chrome
có sẵn, nên `tspdnative.Solve(ctx, tspdnative.Options{})` hợp lệ.

### Tái sử dụng connection

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

`Solver` an toàn khi gọi đồng thời; mỗi lần `Solve` có cookie jar riêng.

## Cấu hình

| Trường | Mặc định | Mô tả |
| --- | --- | --- |
| `Target` | `DefaultTarget` | Trang cần giải challenge. |
| `UserAgent` | Chrome 126 / Windows | Gửi qua `User-Agent` và ghi vào fingerprint field 11. |
| `Language` | `en-NZ` | Locale ghi vào fingerprint field 10. |
| `AcceptLanguage` | suy ra từ `Language` | Ghi đè header `Accept-Language`. |
| `Proxy` | rỗng (trực tiếp) | Proxy URL hoặc chuỗi dạng provider. |
| `Timeout` | `25s` | Giới hạn toàn bộ lần solve, gồm tải script. |
| `Debug` | `false` | Bật log debug khi `Logger` nil. |
| `Logger` | `nil` | Nhận sự kiện pipeline có cấu trúc. |

Các hằng mặc định được export: `DefaultTarget`, `DefaultUserAgent`,
`DefaultLanguage`, `DefaultAcceptLanguage`.

## Kết quả

| Trường | Ý nghĩa |
| --- | --- |
| `Solved` | `true` khi edge cấp `TSPD_101` sau answer cookie. |
| `HadChallenge` | `true` khi response đầu có TSPD challenge. |
| `CookieString` | Giá trị header `Cookie` đầy đủ, theo thứ tự chèn. |
| `TSPD101` | Giá trị cookie đã clear khi `Solved` là true. |
| `AnswerCookieName` | Tên answer cookie dựng native, ví dụ `TSaa03129d075`. |
| `HTMLLength` | Độ dài byte của `HTML`. |
| `HTML` | Body response thứ hai, hoặc body đầu nếu không có challenge. |
| `Cookies` | Snapshot cookie jar theo thứ tự chèn. |

Trang không có challenge không phải lỗi: `Solve` trả thành công với
`Solved == false`, `HadChallenge == false` cùng các session cookie nhận được.

## Xử lý lỗi

Mọi lỗi dự kiến đều là `*tspdnative.SolveError`:

```go
var solveErr *tspdnative.SolveError
if errors.As(err, &solveErr) {
	switch solveErr.Kind {
	case tspdnative.KindNetwork:   // DNS, TLS, timeout
	case tspdnative.KindProxy:     // proxy sai hoặc scheme không hỗ trợ
	case tspdnative.KindGetFailed: // request đầu trả non-2xx
	case tspdnative.KindNoAnswer:  // không dựng được answer cookie
	case tspdnative.KindNoTSPD:    // edge từ chối answer cookie
	}
	log.Printf("status=%d cause=%v", solveErr.HTTPStatus, solveErr.Err)
}
```

Dùng `errors.Is(err, context.DeadlineExceeded)` cho trường hợp timeout; lỗi
timeout được bọc trong `KindNetwork`.

## Hỗ trợ proxy

Truyền trực tiếp `Options.Proxy`, hoặc gọi `tspdnative.ProxyFromEnv()` để đọc
`HTTPS_PROXY`, `https_proxy`, `HTTP_PROXY`, `http_proxy` (theo thứ tự).
Thư viện không tự đọc biến môi trường.

| Input | Chuẩn hóa thành |
| --- | --- |
| `http://user:pass@host:8080` | giữ nguyên |
| `socks5://host:1080` | giữ nguyên |
| `socks5h://host:1080` | giữ nguyên |
| `host:8080:user:pass` | `http://user:pass@host:8080` |
| `user:pass@host:8080` | `http://user:pass@host:8080` |
| `host:8080` | `http://host:8080` |

`NormalizeProxy` được export nếu cần dùng lại logic parse. `socks5h` resolve
DNS qua proxy; `socks5` resolve cục bộ.

## CLI

### Cài đặt

```bash
go install github.com/aizen-sosuke218/tspdnative-go/cmd/tspdctl@latest
```

Hoặc build từ source:

```bash
make build          # -> bin/tspdctl
```

### Cách dùng

```text
tspdctl [solve] [flags]
tspdctl <proxy>
tspdctl bench-answer [--iters N]
```

```bash
tspdctl                                  # kết nối trực tiếp
tspdctl --proxy "$HTTPS_PROXY"
tspdctl --proxy host:8080:user:pass
tspdctl user:pass@host:8080
tspdctl --json                           # output cho máy đọc
tspdctl --repeat 5                       # 5 lần solve, connection ấm
tspdctl bench-answer --iters 100000      # benchmark CPU, không network
```

| Flag | Mặc định | Mô tả |
| --- | --- | --- |
| `--proxy` | env, rồi trực tiếp | Proxy URL hoặc chuỗi dạng provider. |
| `--url` | host immigration | Trang đích. |
| `--ua` | Chrome 126 / Windows | User agent. |
| `--lang` | `en-NZ` | Locale fingerprint. |
| `--json` | `false` | In JSON thay vì chuỗi cookie. |
| `--repeat` | `1` | Solve N lần với connection ấm. |
| `--iters` | `20000` | Số vòng cho `bench-answer`. |
| `--debug` | `false` | Log từng bước pipeline ra stderr. |

- `stdout` — chuỗi cookie hoặc JSON.
- `stderr` — trạng thái, log debug và lỗi.
- Exit code: `0` khi có `TSPD_101`, `1` khi thất bại hoặc trang không
  challenge, `2` khi sai cú pháp.

## Cách hoạt động

```text
GET /                              trang challenge: bobcmn + hex challenge
  │
  ├─ parse bobcmn TLV              cấu hình theo build
  ├─ GET /TSPD/<id>?type=10        sensor script, tải song song
  │    └─ tìm key literal
  │
  ├─ unseal challenge              kiểm MAC, đọc context 4 byte
  ├─ đọc shortcut message          scope AB → context 48 byte
  ├─ seal fingerprint (159 byte)   scope 04, challenge context
  └─ ghép answer cookie            TS<id8><075>=0502c101...
  │
GET / + answer cookie              edge xác thực → Set-Cookie: TSPD_101
```

1. Tải trang đích, trích `bobcmn` và challenge blob.
2. Tìm và tải sensor script bên ngoài, định vị key theo build.
3. Dựng answer cookie offline: unseal challenge, lấy context, seal
   fingerprint, format giá trị cookie.
4. Gửi lại request kèm answer cookie; edge trả `TSPD_101` nếu hợp lệ.

Tầng crypto mô phỏng đúng envelope, digest và MAC của bản C# gốc, bao gồm bố
cục byte của answer. Xem [`internal/securemsg`](internal/securemsg) và
[`internal/answer`](internal/answer).

## Cấu trúc project

```text
tspdnative-go/
├── doc.go                  package documentation
├── solver.go               Solver, Solve, request pipeline
├── options.go              Options + defaulting
├── result.go               Result, Cookie
├── errors.go               ErrorKind, SolveError
├── session.go              HTTP session keep-alive
├── headers.go              browser headers, UA/locale helpers
├── proxy.go                chuẩn hóa proxy + đọc env
├── jar.go                  cookie jar có thứ tự
├── internal/
│   ├── securemsg/          cipher, CBC, digest/MAC, seal/unseal
│   ├── challenge/          parser bobcmn TLV
│   ├── discovery/          quét <script> + locator
│   ├── answer/             fingerprint + answer builder
│   └── testsupport/        fixture tổng hợp cho test
├── cmd/tspdctl/            CLI
├── testdata/               fixture protocol
└── Makefile
```

Chỉ package gốc là public API. Mọi thứ trong `internal/` có thể thay đổi mà
không báo trước.

## Kiểm thử

```bash
make test     # unit test + mock edge end-to-end
make race     # go test -race ./...
make vet      # go vet ./...
make bench    # benchmark crypto và builder
```

Fixture trong [`testdata/`](testdata) lấy từ session thật:

- `captured_challenge.json` — vector unseal/seal;
- `live_answer_main_out3.json`, `live_answer_bench_4.json` — answer cookie
  thật, được rebuild byte-for-byte.

Test end-to-end dựng mock F5 edge cục bộ: chỉ trả `TSPD_101` khi answer cookie
mở seal đúng và khớp fingerprint, nhờ đó kiểm tra toàn bộ đường crypto thay vì
stub. Các nhánh lỗi (`KindNoAnswer`, `KindNoTSPD`, trang không challenge, proxy
sai, cancellation) cũng được phủ.

## Benchmark

Answer builder chạy CPU-only, không phụ thuộc network:

```text
BenchmarkBuild-16     ~12 µs/op    ~5.8 KB/op    ~28 allocs/op
```

Live solve trên target mặc định thường khoảng **2 giây**, chủ yếu là hai lượt
tải trang và một sensor script.

## Ghi chú thiết kế và giới hạn

- **Phạm vi protocol cố định.** Parser và hằng số nhắm tới layout `bobcmn` của
  `onlineservices.immigration.govt.nz`. Các deployment TSPD khác có thể dùng
  tham số khác và không được hỗ trợ.
- **Phụ thuộc build.** Fingerprint và hằng số answer khớp sensor build hiện
  tại. Khi F5 đổi build, `Solve` trả `KindNoTSPD` thay vì cookie cũ.
- **Không đọc env ngầm.** Biến proxy chỉ được đọc khi gọi `ProxyFromEnv`; CLI
  dùng fallback này mặc định.
- **Không phải browser.** Fingerprint là profile cố định khớp user agent và
  locale đã cấu hình, không phải trình giả lập fingerprint đa dụng.
- **Sử dụng có trách nhiệm.** Đây không phải công cụ bypass CAPTCHA; nó tái
  tạo proof-of-work phía client mà browser vẫn thực hiện, cho automation được
  phép.

## Ánh xạ C# sang Go

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

## Giấy phép

Phát hành theo [MIT License](LICENSE).

<div align="center">

**[Lên đầu trang](#tspdnative-go)**

</div>
