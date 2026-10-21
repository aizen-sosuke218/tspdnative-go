package answer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aizen-sosuke218/tspdnative-go/internal/challenge"
	"github.com/aizen-sosuke218/tspdnative-go/internal/securemsg"
	"github.com/aizen-sosuke218/tspdnative-go/internal/testsupport"
)

type capturedFixture struct {
	Key0Hex      string `json:"key0_hex"`
	ChallengeHex string `json:"challenge_hex"`
	Expect       struct {
		ContextHex string `json:"context_hex"`
	} `json:"expect"`
}

type liveFixture struct {
	Key0Hex     string `json:"key0_hex"`
	CookieName  string `json:"cookie_name"`
	CookieValue string `json:"cookie_value"`
	Parsed      struct {
		ContextHex  string `json:"context_hex"`
		SealCtx     string `json:"seal_ctx"`
		Fingerprint struct {
			Field10 string `json:"10"`
			Field11 string `json:"11"`
		} `json:"fingerprint"`
		Tail struct {
			Field7 string `json:"f7"`
		} `json:"tail"`
	} `json:"parsed"`
}

func TestBuildFromCapturedChallenge(t *testing.T) {
	var fixture capturedFixture
	loadJSON(t, "captured_challenge.json", &fixture)

	key0 := mustHex(t, fixture.Key0Hex)
	bobcmn := testsupport.Bobcmn(
		5, 1, 0xAA03129D, 0, 2,
		"", "", "084ce4df70ab2000", "TS",
	)
	html := `<html><body><script>var bobcmn = "` + bobcmn + `";` +
		`window.fp.k = "` + fixture.ChallengeHex + `";</script>` +
		`<script src="/TSPD/x?type=10"></script></body></html>`
	script := "var k=[" + testsupport.EscapedKeyLiteral(key0) + `,""];`

	cookie, err := Build(html, []string{script}, testsupport.Language, testsupport.UserAgent)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if cookie.Name != "TSaa03129d075" {
		t.Fatalf("cookie name = %q, want TSaa03129d075", cookie.Name)
	}
	if !strings.HasPrefix(cookie.Value, "0502c101") {
		t.Fatalf("cookie value prefix = %q", cookie.Value[:8])
	}
	if len(cookie.Value) < 457 {
		t.Fatalf("cookie value length = %d, want at least 457", len(cookie.Value))
	}
	if got := cookie.Value[8:104]; got != fixture.Expect.ContextHex {
		t.Fatalf("embedded context = %s, want %s", got, fixture.Expect.ContextHex)
	}
	if cookie.Value[104] != ':' {
		t.Fatalf("separator = %q, want ':'", cookie.Value[104])
	}

	key, err := securemsg.NewKey(key0)
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	seal, err := hex.DecodeString(cookie.Value[105:457])
	if err != nil {
		t.Fatalf("decode seal: %v", err)
	}
	fingerprint, _, err := securemsg.Unseal(seal, ScopeFingerprint, key, false)
	if err != nil {
		t.Fatalf("unseal fingerprint: %v", err)
	}
	expected, err := BuildFingerprint(testsupport.Language, testsupport.UserAgent)
	if err != nil {
		t.Fatalf("BuildFingerprint: %v", err)
	}
	if !bytes.Equal(fingerprint, expected) {
		t.Fatalf("fingerprint round trip mismatch:\n got %x\nwant %x", fingerprint, expected)
	}
}

func TestLiveCookieRebuild(t *testing.T) {
	for _, name := range []string{
		"live_answer_main_out3.json",
		"live_answer_bench_4.json",
	} {
		t.Run(name, func(t *testing.T) {
			rebuildLiveCookie(t, name)
		})
	}
}

func rebuildLiveCookie(t *testing.T, fixtureName string) {
	t.Helper()

	var fixture liveFixture
	loadJSON(t, fixtureName, &fixture)

	keyBytes := mustHex(t, fixture.Key0Hex)
	key, err := securemsg.NewKey(keyBytes)
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}

	value := fixture.CookieValue
	if len(value) < 457 {
		t.Fatalf("cookie value length = %d, want at least 457", len(value))
	}
	if value[:8] != "0502c101" {
		t.Fatalf("value prefix = %q, want 0502c101", value[:8])
	}
	if got := value[8:104]; got != fixture.Parsed.ContextHex {
		t.Fatalf("embedded context = %s, want %s", got, fixture.Parsed.ContextHex)
	}
	if value[104] != ':' {
		t.Fatalf("separator = %q, want ':'", value[104])
	}

	sealHex := value[105:457]
	sealMessage := mustHex(t, sealHex)
	fingerprint, sealContext, err := securemsg.Unseal(
		sealMessage, ScopeFingerprint, key, false,
	)
	if err != nil {
		t.Fatalf("unseal fingerprint: %v", err)
	}
	if got := hex.EncodeToString(sealContext[:]); got != fixture.Parsed.SealCtx {
		t.Fatalf("seal context = %s, want %s", got, fixture.Parsed.SealCtx)
	}
	if len(fingerprint) != 159 {
		t.Fatalf("fingerprint length = %d, want 159", len(fingerprint))
	}

	resealed, err := securemsg.Seal(fingerprint, ScopeFingerprint, sealContext, key)
	if err != nil {
		t.Fatalf("reseal fingerprint: %v", err)
	}
	if got := hex.EncodeToString(resealed); got != sealHex {
		t.Fatalf("seal round trip mismatch:\n got %s\nwant %s", got, sealHex)
	}

	language := string(mustHex(t, fixture.Parsed.Fingerprint.Field10))
	userAgent := string(mustHex(t, fixture.Parsed.Fingerprint.Field11))
	builtFingerprint, err := BuildFingerprint(language, userAgent)
	if err != nil {
		t.Fatalf("BuildFingerprint: %v", err)
	}
	if !bytes.Equal(builtFingerprint, fingerprint) {
		t.Fatalf("BuildFingerprint does not reproduce the captured 159 bytes")
	}

	config, err := challenge.Parse(testsupport.Bobcmn(
		5, 1, 0xAA03129D, 0, 2,
		"", "", fixture.Parsed.Tail.Field7, "TS",
	))
	if err != nil {
		t.Fatalf("parse synthetic bobcmn: %v", err)
	}

	name, err := BuildName(config)
	if err != nil {
		t.Fatalf("BuildName: %v", err)
	}
	if name != fixture.CookieName {
		t.Fatalf("cookie name = %q, want %q", name, fixture.CookieName)
	}

	rebuiltValue, err := BuildValue(config, fixture.Parsed.ContextHex, sealHex)
	if err != nil {
		t.Fatalf("BuildValue: %v", err)
	}
	if rebuiltValue != fixture.CookieValue {
		t.Fatalf("cookie value mismatch:\n got %s\nwant %s", rebuiltValue, fixture.CookieValue)
	}
}

func TestFingerprintConstants(t *testing.T) {
	fingerprint, err := BuildFingerprint(testsupport.Language, testsupport.UserAgent)
	if err != nil {
		t.Fatalf("BuildFingerprint: %v", err)
	}

	if len(fingerprint) != 159 {
		t.Fatalf("length = %d, want 159", len(fingerprint))
	}
	if fingerprint[15] != 56 {
		t.Fatalf("field 09 = %d, want 56", fingerprint[15])
	}
	if !bytes.Contains(fingerprint, []byte(testsupport.Language)) {
		t.Fatal("fingerprint does not contain the language")
	}
	if !bytes.Contains(fingerprint, []byte(testsupport.UserAgent)) {
		t.Fatal("fingerprint does not contain the user agent")
	}
	if !strings.Contains(
		hex.EncodeToString(fingerprint),
		"86000001013058626000058fffffffff",
	) {
		t.Fatal("fingerprint does not contain the Modernizr bitmask")
	}

	other, err := BuildFingerprint("de-DE", testsupport.UserAgent)
	if err != nil {
		t.Fatalf("BuildFingerprint(de-DE): %v", err)
	}
	if bytes.Equal(fingerprint, other) {
		t.Fatal("changing the language did not change the fingerprint")
	}
}

func TestBuildFailureModes(t *testing.T) {
	challengeHex := strings.Repeat("a", 80)
	bobcmn := testsupport.Bobcmn(5, 1, 0x12345678, 0, 2, "", "", "f7", "TS")
	key := bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 4)
	script := "var k=[" + testsupport.EscapedKeyLiteral(key) + `,""];`
	validPage := `<script>var bobcmn = "` + bobcmn + `";window.a.b = "` + challengeHex + `";</script>`

	tests := []struct {
		name    string
		html    string
		scripts []string
		want    error
	}{
		{
			name:    "bobcmn missing",
			html:    "<html></html>",
			scripts: []string{script},
			want:    ErrBobcmnNotFound,
		},
		{
			name:    "challenge missing",
			html:    `<script>var bobcmn = "` + bobcmn + `";</script>`,
			scripts: []string{script},
			want:    ErrChallengeNotFound,
		},
		{
			name:    "key missing",
			html:    validPage,
			scripts: nil,
			want:    ErrKeyNotFound,
		},
		{
			name: "config too short",
			html: `<script>var bobcmn = "` +
				strings.Repeat("10", 14) +
				`";window.a.b = "` + challengeHex + `";</script>`,
			scripts: []string{script},
			want:    ErrConfigTooShort,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Build(test.html, test.scripts, testsupport.Language, testsupport.UserAgent)
			if !errors.Is(err, test.want) {
				t.Fatalf("Build error = %v, want %v", err, test.want)
			}
		})
	}
}

func BenchmarkBuild(b *testing.B) {
	key := securemsg.Key{
		0xb0, 0x2b, 0xed, 0x0d, 0x2d, 0xac, 0x17, 0xcf,
		0xf5, 0x54, 0x36, 0x4b, 0x62, 0x48, 0xd5, 0x77,
	}
	innerContext := [4]byte{1, 2, 3, 4}
	challengeContext := [4]byte{5, 6, 7, 8}
	innerPlain := make([]byte, 24)
	for i := range innerPlain {
		innerPlain[i] = byte(i + 1)
	}

	inner, err := securemsg.Seal(innerPlain, ScopeInner, innerContext, key)
	if err != nil {
		b.Fatal(err)
	}
	challengeMessage, err := securemsg.Seal(inner, ScopeChallenge, challengeContext, key)
	if err != nil {
		b.Fatal(err)
	}

	bobcmn := testsupport.Bobcmn(5, 1, 0xAA03129D, 0, 2, "", "", "084ce4df70ab2000", "TS")
	html := fmt.Sprintf(
		`<html><script>var bobcmn = %q;window.fp.k = %q;</script><script src="/TSPD/x?type=10"></script></html>`,
		bobcmn, hex.EncodeToString(challengeMessage),
	)
	scripts := []string{"var k=[" + testsupport.EscapedKeyLiteral(key[:]) + `,""];`}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Build(html, scripts, testsupport.Language, testsupport.UserAgent); err != nil {
			b.Fatal(err)
		}
	}
}

func loadJSON(t *testing.T, name string, target any) {
	t.Helper()
	data, err := os.ReadFile(testsupport.FixturePath(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
}

func mustHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode hex %q: %v", value, err)
	}
	return decoded
}
