package tspdnative

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aizen-sosuke218/tspdnative-go/internal/answer"
	"github.com/aizen-sosuke218/tspdnative-go/internal/discovery"
)

// Solver performs TSPD solves over a reusable keep-alive HTTP session.
//
// A Solver is safe for concurrent use: the request pipeline keeps all mutable
// state in the cookie jar of each Solve call.
type Solver struct {
	options Options
	session *httpSession
	logger  *slog.Logger
}

// NewSolver creates a solver and validates its proxy configuration.
//
// The caller must call Close when the solver is no longer needed.
func NewSolver(options Options) (*Solver, error) {
	options = options.withDefaults()

	session, err := newHTTPSession(options.Proxy)
	if err != nil {
		return nil, solveError(KindProxy, err)
	}

	logger := options.Logger
	if logger == nil && options.Debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}
	if logger != nil {
		logger = logger.With("component", "tspdnative")
	}

	return &Solver{
		options: options,
		session: session,
		logger:  logger,
	}, nil
}

// Solve performs one solve with a temporary Solver. It is the simplest entry
// point for callers that do not need connection reuse.
func Solve(ctx context.Context, options Options) (*Result, error) {
	solver, err := NewSolver(options)
	if err != nil {
		return nil, err
	}
	defer solver.Close()
	return solver.Solve(ctx)
}

// Solve executes the challenge pipeline:
//
//	GET target -> challenge page
//	GET sensor scripts -> key
//	build answer cookie offline
//	GET target with answer -> TSPD_101
//
// When the page is not challenged, Solve returns a successful result with
// Solved and HadChallenge set to false.
func (s *Solver) Solve(ctx context.Context) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	solveCtx, cancel := context.WithTimeout(ctx, s.options.Timeout)
	defer cancel()

	started := time.Now()
	headers := buildHeaders(s.options)
	jar := newCookieJar()

	// First pass: collect the challenge page and any early session cookies.
	first, err := s.get(solveCtx, s.options.Target, headers, "", "")
	if err != nil {
		return nil, err
	}
	if first.status < http.StatusOK || first.status >= http.StatusMultipleChoices {
		return nil, &SolveError{
			Kind:       KindGetFailed,
			HTTPStatus: first.status,
			Err:        fmt.Errorf("GET %s failed", s.options.Target),
		}
	}
	jar.applySetCookies(first.setCookies)
	s.debug("challenge page fetched",
		"status", first.status,
		"bytes", len(first.body),
		"cookies", jar.Len(),
		"elapsed", time.Since(started),
	)

	bobcmn, challenged := discovery.FindBobcmn(first.body)
	if !challenged {
		s.debug("page is not challenged", "elapsed", time.Since(started))
		return &Result{
			CookieString: jar.Header(),
			HTMLLength:   len(first.body),
			HTML:         first.body,
			Cookies:      jar.Snapshot(),
		}, nil
	}
	s.debug("bobcmn located", "chars", len(bobcmn))

	// The sensor script contains the per-build key used by the crypto layer.
	scripts := discovery.ExtractScripts(first.body, first.finalURL)
	externalURLs := make([]string, 0, len(scripts))
	for _, script := range scripts {
		if script.IsExternal() {
			externalURLs = append(externalURLs, script.URL)
		}
	}
	scriptBodies := s.fetchScripts(solveCtx, externalURLs, headers, jar.Header(), s.options.Target)
	s.debug("sensor scripts fetched",
		"total", len(scripts),
		"external", len(externalURLs),
		"fetched", len(scriptBodies),
		"elapsed", time.Since(started),
	)
	if err := solveCtx.Err(); err != nil {
		return nil, solveError(KindNetwork, fmt.Errorf("solve aborted: %w", err))
	}

	// Native answer: unseal challenge -> read context -> seal fingerprint.
	answerCookie, err := answer.Build(
		first.body,
		scriptBodies,
		s.options.Language,
		s.options.UserAgent,
	)
	if err != nil {
		return nil, solveError(KindNoAnswer, err)
	}
	jar.Set(answerCookie.Name, answerCookie.Value)
	s.debug("answer cookie built",
		"name", answerCookie.Name,
		"chars", len(answerCookie.Value),
		"elapsed", time.Since(started),
	)

	// Second pass: the edge clears the challenge with TSPD_101.
	second, err := s.get(solveCtx, s.options.Target, headers, jar.Header(), "")
	if err != nil {
		return nil, err
	}
	jar.applySetCookies(second.setCookies)

	tspd101, present := jar.Get("TSPD_101")
	if !present {
		return nil, &SolveError{
			Kind:       KindNoTSPD,
			HTTPStatus: second.status,
			Err:        fmt.Errorf("no TSPD_101 in second response (%d bytes)", len(second.body)),
		}
	}
	s.debug("challenge cleared",
		"status", second.status,
		"tspd101", len(tspd101),
		"elapsed", time.Since(started),
	)

	return &Result{
		Solved:           true,
		HadChallenge:     true,
		CookieString:     jar.Header(),
		TSPD101:          tspd101,
		AnswerCookieName: answerCookie.Name,
		HTMLLength:       len(second.body),
		HTML:             second.body,
		Cookies:          jar.Snapshot(),
	}, nil
}

// Close releases pooled connections. It is safe to call more than once.
func (s *Solver) Close() error {
	s.session.close()
	return nil
}

func (s *Solver) get(
	ctx context.Context,
	target string,
	headers http.Header,
	cookie string,
	referer string,
) (*httpResponse, error) {
	response, err := s.session.get(ctx, httpRequest{
		url:     target,
		headers: headers,
		cookie:  cookie,
		referer: referer,
	})
	if err == nil {
		return response, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, solveError(KindNetwork, fmt.Errorf("GET %s: %w", target, ctxErr))
	}
	return nil, solveError(KindNetwork, fmt.Errorf("GET %s: %w", target, err))
}

// fetchScripts downloads sensor scripts concurrently, preserving URL order.
// Individual failures are tolerated because the key locator can use any
// script that carries it.
func (s *Solver) fetchScripts(
	ctx context.Context,
	urls []string,
	headers http.Header,
	cookie string,
	referer string,
) []string {
	if len(urls) == 0 {
		return nil
	}

	results := make([]string, len(urls))
	semaphore := make(chan struct{}, maxParallelScripts)
	var waitGroup sync.WaitGroup

	for index, scriptURL := range urls {
		waitGroup.Add(1)
		go func(index int, scriptURL string) {
			defer waitGroup.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			scriptCtx, cancel := context.WithTimeout(ctx, defaultScriptTimeout)
			defer cancel()

			response, err := s.get(scriptCtx, scriptURL, headers, cookie, referer)
			if err != nil {
				s.debug("sensor script failed", "url", scriptURL, "error", err)
				return
			}
			if response.status < http.StatusOK || response.status >= http.StatusMultipleChoices {
				s.debug("sensor script rejected", "url", scriptURL, "status", response.status)
				return
			}
			s.debug("sensor script fetched",
				"url", scriptURL,
				"status", response.status,
				"bytes", len(response.body),
			)
			results[index] = response.body
		}(index, scriptURL)
	}

	waitGroup.Wait()

	bodies := make([]string, 0, len(results))
	for _, body := range results {
		if body != "" {
			bodies = append(bodies, body)
		}
	}
	return bodies
}

func (s *Solver) debug(message string, args ...any) {
	if s.logger != nil {
		s.logger.Debug(message, args...)
	}
}
