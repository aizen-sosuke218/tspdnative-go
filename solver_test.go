package tspdnative

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aizen-sosuke218/tspdnative-go/internal/answer"
	"github.com/aizen-sosuke218/tspdnative-go/internal/securemsg"
	"github.com/aizen-sosuke218/tspdnative-go/internal/testsupport"
)

type recordedRequest struct {
	Method string
	Path   string
	Cookie string
}

type mockConfig struct {
	omitKey        bool
	forceChallenge bool
	forcedBody     string
}

// challengeMock is a local F5 edge. It validates the answer cookie
// cryptographically before issuing TSPD_101.
type challengeMock struct {
	server       *httptest.Server
	key          securemsg.Key
	context      []byte
	challengeHex string
	bobcmn       string
	expectedName string
	config       mockConfig

	mu              sync.Mutex
	requests        []recordedRequest
	answerValidated bool
	lastFailure     string
}

func newChallengeMock(t *testing.T, config mockConfig) *challengeMock {
	t.Helper()

	keyMaterial := make([]byte, securemsg.KeySize)
	if _, err := rand.Read(keyMaterial); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyMaterial[0] = 0x0B
	keyMaterial[1] = 0x1F // guarantee non-printable bytes for the key literal
	key, err := securemsg.NewKey(keyMaterial)
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}

	challengeContext := randomContext(t)
	innerContext := randomContext(t)
	innerPlain := make([]byte, 24)
	if _, err := rand.Read(innerPlain); err != nil {
		t.Fatalf("generate context: %v", err)
	}

	// The embedded context is the 48-byte sealed inner message. The
	// challenge then seals that context under scope 01.
	innerMessage, err := securemsg.Seal(innerPlain, answer.ScopeInner, innerContext, key)
	if err != nil {
		t.Fatalf("seal inner message: %v", err)
	}
	challengeMessage, err := securemsg.Seal(innerMessage, answer.ScopeChallenge, challengeContext, key)
	if err != nil {
		t.Fatalf("seal challenge: %v", err)
	}

	var sessionBytes [4]byte
	if _, err := rand.Read(sessionBytes[:]); err != nil {
		t.Fatalf("generate session id: %v", err)
	}
	session := binary.BigEndian.Uint32(sessionBytes[:])

	mock := &challengeMock{
		key:          key,
		context:      innerMessage,
		challengeHex: hex.EncodeToString(challengeMessage),
		expectedName: fmt.Sprintf("TS%08x075", session),
		bobcmn: testsupport.Bobcmn(
			5, 1, session, 0, 2,
			"", "", "084ce4df70ab2000", "TS",
		),
		config: config,
	}
	mock.server = httptest.NewServer(http.HandlerFunc(mock.handle))
	t.Cleanup(mock.server.Close)
	return mock
}

func (m *challengeMock) handle(writer http.ResponseWriter, request *http.Request) {
	m.mu.Lock()
	m.requests = append(m.requests, recordedRequest{
		Method: request.Method,
		Path:   request.URL.Path,
		Cookie: request.Header.Get("Cookie"),
	})
	m.mu.Unlock()

	if strings.HasPrefix(request.URL.Path, "/TSPD/") {
		writer.Header().Set("Content-Type", "application/javascript")
		if m.config.omitKey {
			io.WriteString(writer, "var nothing=1;")
			return
		}
		fmt.Fprintf(writer, "var k=[%s,\"\"];", testsupport.EscapedKeyLiteral(m.key[:]))
		return
	}

	if m.config.forcedBody != "" {
		io.WriteString(writer, m.config.forcedBody)
		return
	}

	if !m.config.forceChallenge {
		valid, failure := m.validateAnswer(request)
		if valid {
			m.mu.Lock()
			m.answerValidated = true
			m.mu.Unlock()
			writer.Header().Set("Set-Cookie", "TSPD_101=abcdef0123456789; Path=/")
			io.WriteString(writer, "SOLVED-OK")
			return
		}
		if failure != "" {
			m.mu.Lock()
			m.lastFailure = failure
			m.mu.Unlock()
		}
	}

	writer.Header().Set("Set-Cookie", "TSmock028=1; Path=/")
	fmt.Fprintf(
		writer,
		`<html><head><title>Challenge</title></head><body>`+
			`<script>var bobcmn = "%s";window.fp.k = "%s";</script>`+
			`<script src="/TSPD/%s?type=10"></script></body></html>`,
		m.bobcmn,
		m.challengeHex,
		m.challengeHex[:8],
	)
}

func (m *challengeMock) validateAnswer(request *http.Request) (bool, string) {
	cookie, err := request.Cookie(m.expectedName)
	if err != nil {
		return false, ""
	}

	value := cookie.Value
	if len(value) < 457 || !strings.HasPrefix(value, "0502c101") || value[104] != ':' {
		return false, fmt.Sprintf("bad value shape (len %d)", len(value))
	}
	if got := value[8:104]; got != hex.EncodeToString(m.context) {
		return false, fmt.Sprintf("context mismatch: got %s", got)
	}

	sealBytes, err := hex.DecodeString(value[105:457])
	if err != nil {
		return false, "seal is not hex: " + err.Error()
	}
	fingerprint, _, err := securemsg.Unseal(
		sealBytes, answer.ScopeFingerprint, m.key, false,
	)
	if err != nil {
		return false, "unseal fingerprint: " + err.Error()
	}
	expected, err := answer.BuildFingerprint(testsupport.Language, testsupport.UserAgent)
	if err != nil {
		return false, "build expected fingerprint: " + err.Error()
	}
	if !bytes.Equal(fingerprint, expected) {
		return false, "fingerprint mismatch"
	}
	return true, ""
}

func (m *challengeMock) Requests() []recordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]recordedRequest(nil), m.requests...)
}

func (m *challengeMock) AnswerValidated() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.answerValidated
}

func (m *challengeMock) LastFailure() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastFailure
}

func TestSolveFullPipeline(t *testing.T) {
	mock := newChallengeMock(t, mockConfig{})

	result, err := Solve(context.Background(), Options{
		Target:  mock.server.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Solve: %v (mock: %s)", err, mock.LastFailure())
	}

	if !result.Solved {
		t.Fatal("Solved = false")
	}
	if !result.HadChallenge {
		t.Fatal("HadChallenge = false")
	}
	if result.TSPD101 != "abcdef0123456789" {
		t.Fatalf("TSPD101 = %q", result.TSPD101)
	}
	if result.AnswerCookieName != mock.expectedName {
		t.Fatalf("AnswerCookieName = %q, want %q", result.AnswerCookieName, mock.expectedName)
	}
	if !mock.AnswerValidated() {
		t.Fatal("the mock did not validate the answer")
	}
	if !strings.Contains(result.CookieString, mock.expectedName+"=") {
		t.Fatalf("answer missing from CookieString: %q", result.CookieString)
	}
	if !strings.Contains(result.CookieString, "TSPD_101=abcdef0123456789") {
		t.Fatalf("TSPD_101 missing from CookieString: %q", result.CookieString)
	}
	if !strings.Contains(result.CookieString, "TSmock028=1") {
		t.Fatalf("early session cookie missing: %q", result.CookieString)
	}
	if !strings.Contains(result.HTML, "SOLVED-OK") {
		t.Fatalf("second response body = %q", result.HTML)
	}

	requests := mock.Requests()
	if len(requests) != 3 {
		t.Fatalf("request count = %d, want 3", len(requests))
	}
	if !strings.HasPrefix(requests[1].Path, "/TSPD/") {
		t.Fatalf("request 1 path = %q", requests[1].Path)
	}
	if !strings.Contains(requests[2].Cookie, mock.expectedName+"=") {
		t.Fatalf("answer not sent on request 2: %q", requests[2].Cookie)
	}
}

func TestSolveWithoutChallenge(t *testing.T) {
	mock := newChallengeMock(t, mockConfig{forcedBody: "<html>real page</html>"})

	result, err := Solve(context.Background(), Options{
		Target:  mock.server.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if result.Solved {
		t.Fatal("Solved = true for an unchallenged page")
	}
	if result.HadChallenge {
		t.Fatal("HadChallenge = true for an unchallenged page")
	}
	if result.CookieString != "" {
		t.Fatalf("CookieString = %q, want empty", result.CookieString)
	}
	if got := len(mock.Requests()); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
}

func TestSolveMissingKey(t *testing.T) {
	mock := newChallengeMock(t, mockConfig{omitKey: true})

	_, err := Solve(context.Background(), Options{
		Target:  mock.server.URL,
		Timeout: 5 * time.Second,
	})
	var solveErr *SolveError
	if !errors.As(err, &solveErr) || solveErr.Kind != KindNoAnswer {
		t.Fatalf("Solve error = %v, want KindNoAnswer", err)
	}
}

func TestSolveRejectedAnswer(t *testing.T) {
	mock := newChallengeMock(t, mockConfig{forceChallenge: true})

	_, err := Solve(context.Background(), Options{
		Target:  mock.server.URL,
		Timeout: 5 * time.Second,
	})
	var solveErr *SolveError
	if !errors.As(err, &solveErr) || solveErr.Kind != KindNoTSPD {
		t.Fatalf("Solve error = %v, want KindNoTSPD", err)
	}
	if got := len(mock.Requests()); got != 3 {
		t.Fatalf("request count = %d, want 3", got)
	}
}

func TestNewSolverRejectsUnsupportedProxy(t *testing.T) {
	_, err := NewSolver(Options{Proxy: "ftp://1.2.3.4:21"})
	var solveErr *SolveError
	if !errors.As(err, &solveErr) || solveErr.Kind != KindProxy {
		t.Fatalf("NewSolver error = %v, want KindProxy", err)
	}
}

func TestSolveHonoursContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Solve(ctx, Options{
		Target:  "http://127.0.0.1:1",
		Timeout: time.Second,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Solve error = %v, want context.Canceled", err)
	}
}

func TestOptionsDefaults(t *testing.T) {
	options := Options{}.withDefaults()

	if options.Target != DefaultTarget {
		t.Fatalf("Target = %q", options.Target)
	}
	if options.UserAgent != DefaultUserAgent {
		t.Fatalf("UserAgent = %q", options.UserAgent)
	}
	if options.Language != DefaultLanguage {
		t.Fatalf("Language = %q", options.Language)
	}
	if options.Timeout != 25*time.Second {
		t.Fatalf("Timeout = %s", options.Timeout)
	}
}

func randomContext(t *testing.T) [4]byte {
	t.Helper()
	var context [4]byte
	if _, err := rand.Read(context[:]); err != nil {
		t.Fatalf("generate context: %v", err)
	}
	return context
}
