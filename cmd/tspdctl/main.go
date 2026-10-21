// Command tspdctl solves the TSPD challenge and prints the session cookie.
//
//	stdout  cookie string ("k=v; k2=v2; ...") or JSON with --json
//	stderr  status, diagnostics and errors
//	exit    0 only when the edge issued TSPD_101, 1 on failure, 2 on bad usage
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aizen-sosuke218/tspdnative-go"
	"github.com/aizen-sosuke218/tspdnative-go/internal/answer"
	"github.com/aizen-sosuke218/tspdnative-go/internal/securemsg"
)

const usageText = `tspdctl - engine-free TSPD solver for onlineservices.immigration.govt.nz

Usage:
  tspdctl [solve] [flags]
  tspdctl <proxy>
  tspdctl bench-answer [--iters <n>]

Commands:
  solve, cookie   Solve the challenge (default).
  bench-answer    CPU-only benchmark of the answer builder. No network.

Flags:
  --proxy <value>   Proxy URL or host:port:user:pass / user:pass@host:port.
                    Falls back to HTTPS_PROXY/HTTP_PROXY, then direct.
  --url <value>     Target page (default: the immigration host).
  --ua <value>      User agent (default: Chrome 126 / Windows).
  --lang <value>    Fingerprint locale (default: en-NZ).
  --json            Print a JSON object instead of the cookie string.
  --repeat <n>      Solve n times in one process with a warm connection.
  --iters <n>       Iterations for bench-answer (default: 20000).
  --debug           Log the solve pipeline to stderr.
  -h, --help        Show this help.

Output:
  stdout is the Cookie header value, for example "TSPD_101=...; TS...=...".
  Exit code 0 only when the edge issued TSPD_101.
`

type cliConfig struct {
	command    string
	proxy      string
	target     string
	userAgent  string
	language   string
	jsonOutput bool
	debug      bool
	repeat     int
	iterations int
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	config := cliConfig{
		command:    "solve",
		repeat:     1,
		iterations: 20_000,
	}

	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch argument {
		case "solve", "cookie", "bench-answer":
			config.command = argument

		case "--proxy":
			value, ok := nextValue(args, &index, argument)
			if !ok {
				return 2
			}
			config.proxy = value

		case "--url":
			value, ok := nextValue(args, &index, argument)
			if !ok {
				return 2
			}
			config.target = value

		case "--ua":
			value, ok := nextValue(args, &index, argument)
			if !ok {
				return 2
			}
			config.userAgent = value

		case "--lang":
			value, ok := nextValue(args, &index, argument)
			if !ok {
				return 2
			}
			config.language = value

		case "--json":
			config.jsonOutput = true

		case "--debug":
			config.debug = true

		case "--repeat":
			value, ok := nextValue(args, &index, argument)
			if !ok {
				return 2
			}
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 {
				fmt.Fprintln(os.Stderr, "error: --repeat must be an integer >= 1")
				return 2
			}
			config.repeat = parsed

		case "--iters":
			value, ok := nextValue(args, &index, argument)
			if !ok {
				return 2
			}
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 {
				fmt.Fprintln(os.Stderr, "error: --iters must be an integer >= 1")
				return 2
			}
			config.iterations = parsed

		case "-h", "--help":
			fmt.Print(usageText)
			return 0

		default:
			if strings.HasPrefix(argument, "-") {
				fmt.Fprintf(os.Stderr, "error: unknown flag %s (see --help)\n", argument)
				return 2
			}
			if config.command == "solve" && config.proxy == "" {
				config.proxy = argument
				continue
			}
			fmt.Fprintf(os.Stderr, "error: unexpected argument %s (see --help)\n", argument)
			return 2
		}
	}

	if config.command == "bench-answer" {
		return runAnswerBenchmark(config.iterations)
	}

	if config.proxy == "" {
		config.proxy = tspdnative.ProxyFromEnv()
	}

	options := tspdnative.Options{
		Target:    config.target,
		UserAgent: config.userAgent,
		Language:  config.language,
		Proxy:     config.proxy,
		Debug:     config.debug,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, elapsed, err := execute(ctx, options, config.repeat)
	if err != nil {
		var solveErr *tspdnative.SolveError
		if errors.As(err, &solveErr) {
			fmt.Fprintf(os.Stderr, "error [%s]: %v\n", solveErr.Kind, solveErr.Err)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return 1
	}

	if config.jsonOutput {
		if err := writeJSON(result, elapsed); err != nil {
			fmt.Fprintf(os.Stderr, "error: encode JSON: %v\n", err)
			return 1
		}
	} else {
		fmt.Println(result.CookieString)
	}

	if result.Solved {
		fmt.Fprintf(
			os.Stderr,
			"solved in %dms - %d cookies, TSPD_101 %d chars, answer %s\n",
			elapsed.Milliseconds(),
			len(result.Cookies),
			len(result.TSPD101),
			result.AnswerCookieName,
		)
	} else {
		fmt.Fprintf(
			os.Stderr,
			"warning: no challenge served (real page, %d bytes) in %dms - "+
				"%d cookies, no TSPD_101; the cookie string is still valid for this session\n",
			result.HTMLLength,
			elapsed.Milliseconds(),
			len(result.Cookies),
		)
	}

	if result.Solved {
		return 0
	}
	return 1
}

func execute(
	ctx context.Context,
	options tspdnative.Options,
	repeat int,
) (*tspdnative.Result, time.Duration, error) {
	if repeat == 1 {
		started := time.Now()
		result, err := tspdnative.Solve(ctx, options)
		return result, time.Since(started), err
	}

	solver, err := tspdnative.NewSolver(options)
	if err != nil {
		return nil, 0, err
	}
	defer solver.Close()

	var result *tspdnative.Result
	var lastElapsed time.Duration
	var total time.Duration
	var minDuration time.Duration
	var maxDuration time.Duration

	for run := 1; run <= repeat; run++ {
		started := time.Now()
		current, err := solver.Solve(ctx)
		elapsed := time.Since(started)
		if err != nil {
			fmt.Fprintf(os.Stderr, "run %d/%d failed: %v\n", run, repeat, err)
			return nil, 0, err
		}
		result = current
		lastElapsed = elapsed
		total += elapsed
		if run == 1 || elapsed < minDuration {
			minDuration = elapsed
		}
		if elapsed > maxDuration {
			maxDuration = elapsed
		}
		fmt.Fprintf(
			os.Stderr,
			"run %d/%d: solved=%t %dms (TSPD_101 %d chars)\n",
			run, repeat, current.Solved, elapsed.Milliseconds(), len(current.TSPD101),
		)
	}

	fmt.Fprintf(
		os.Stderr,
		"avg %dms over %d runs (min %dms, max %dms)\n",
		(total / time.Duration(repeat)).Milliseconds(),
		repeat,
		minDuration.Milliseconds(),
		maxDuration.Milliseconds(),
	)
	return result, lastElapsed, nil
}

type jsonResult struct {
	Solved           bool    `json:"solved"`
	Cookie           string  `json:"cookie"`
	TSPD101          *string `json:"tspd101"`
	AnswerCookieName *string `json:"answerCookieName"`
	HadChallenge     bool    `json:"hadChallenge"`
	Cookies          int     `json:"cookies"`
	ElapsedMS        int64   `json:"elapsedMs"`
}

func writeJSON(result *tspdnative.Result, elapsed time.Duration) error {
	output := jsonResult{
		Solved:       result.Solved,
		Cookie:       result.CookieString,
		HadChallenge: result.HadChallenge,
		Cookies:      len(result.Cookies),
		ElapsedMS:    elapsed.Milliseconds(),
	}
	if result.Solved {
		output.TSPD101 = &result.TSPD101
	}
	if result.AnswerCookieName != "" {
		output.AnswerCookieName = &result.AnswerCookieName
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// runAnswerBenchmark measures the full native answer builder on inputs shaped
// like the live page. It performs no network I/O.
func runAnswerBenchmark(iterations int) int {
	random := rand.New(rand.NewSource(12345))
	keyMaterial := make([]byte, securemsg.KeySize)
	random.Read(keyMaterial)
	keyMaterial[0] = 0x0B
	keyMaterial[1] = 0x1F
	key, err := securemsg.NewKey(keyMaterial)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bench: %v\n", err)
		return 1
	}

	innerPlain := make([]byte, 24)
	random.Read(innerPlain)
	var innerContext, challengeContext [4]byte
	random.Read(innerContext[:])
	random.Read(challengeContext[:])

	innerMessage, err := securemsg.Seal(innerPlain, answer.ScopeInner, innerContext, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bench: seal inner: %v\n", err)
		return 1
	}
	challengeMessage, err := securemsg.Seal(
		innerMessage, answer.ScopeChallenge, challengeContext, key,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bench: seal challenge: %v\n", err)
		return 1
	}

	bobcmn := syntheticBobcmn()
	html := fmt.Sprintf(
		`<html><head><title>Challenge</title></head><body>`+
			`<script>var bobcmn = %q;window.fp.k = %q;</script>`+
			`<script src="/TSPD/x?type=10"></script></body></html>`,
		bobcmn,
		hex.EncodeToString(challengeMessage),
	)
	scripts := []string{"var k=[" + escapedKeyLiteral(key[:]) + `,""];`}

	cookie, err := answer.Build(html, scripts, tspdnative.DefaultLanguage, tspdnative.DefaultUserAgent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bench: builder failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "bench: synthetic answer %s (%d chars)\n", cookie.Name, len(cookie.Value))

	started := time.Now()
	for i := 0; i < iterations; i++ {
		if _, err := answer.Build(
			html, scripts, tspdnative.DefaultLanguage, tspdnative.DefaultUserAgent,
		); err != nil {
			fmt.Fprintf(os.Stderr, "bench: iteration %d failed: %v\n", i, err)
			return 1
		}
	}
	elapsed := time.Since(started)
	perOperation := elapsed.Seconds() * 1e6 / float64(iterations)
	fmt.Fprintf(
		os.Stderr,
		"bench-answer: %d iters in %dms -> %.2f us/op\n",
		iterations, elapsed.Milliseconds(), perOperation,
	)
	return 0
}

// syntheticBobcmn builds a 25-entry configuration with the live field layout.
func syntheticBobcmn() string {
	var builder strings.Builder
	builder.Grow(400)
	for index := 0; index < 25; index++ {
		switch index {
		case 7:
			builder.WriteString("200000005")
		case 10:
			builder.WriteString("200000001")
		case 11:
			builder.WriteString("2aa03129d")
		case 13:
			builder.WriteString("200000000")
		case 14:
			builder.WriteString("200000002")
		case 21:
			builder.WriteString("300000010084ce4df70ab2000")
		case 22:
			builder.WriteString("300000002TS")
		default:
			builder.WriteString("10")
		}
	}
	return builder.String()
}

// escapedKeyLiteral renders key bytes as a JavaScript string literal.
func escapedKeyLiteral(key []byte) string {
	var builder strings.Builder
	builder.Grow(len(key)*4 + 2)
	builder.WriteByte('"')
	for _, value := range key {
		if value >= 0x20 && value <= 0x7E && value != '"' && value != '\\' {
			builder.WriteByte(value)
			continue
		}
		fmt.Fprintf(&builder, "\\x%02x", value)
	}
	builder.WriteByte('"')
	return builder.String()
}

func nextValue(args []string, index *int, flag string) (string, bool) {
	if *index+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "error: %s requires a value\n", flag)
		return "", false
	}
	*index = *index + 1
	return args[*index], true
}
