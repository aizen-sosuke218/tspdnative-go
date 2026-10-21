package discovery

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/aizen-sosuke218/tspdnative-go/internal/testsupport"
)

func TestFindBobcmn(t *testing.T) {
	raw := testsupport.Bobcmn(5, 1, 0x12345678, 0, 2, "", "", "f7", "TS")
	html := `<html><script>var bobcmn = "` + raw + `";</script></html>`

	found, ok := FindBobcmn(html)
	if !ok {
		t.Fatal("FindBobcmn did not find the configuration")
	}
	if found != raw {
		t.Fatalf("bobcmn = %q, want %q", found, raw)
	}

	if _, ok := FindBobcmn("<html>plain</html>"); ok {
		t.Fatal("FindBobcmn accepted a page without bobcmn")
	}
}

func TestFindChallenge(t *testing.T) {
	challenge := strings.Repeat("a", 80)
	html := `<html><script>window.zz.aa = "` + challenge + `";</script></html>`

	found, ok := FindChallenge(html)
	if !ok {
		t.Fatal("FindChallenge did not find the challenge")
	}
	if found != challenge {
		t.Fatalf("challenge = %q, want %q", found, challenge)
	}

	if _, ok := FindChallenge(`<script>window.x = "` + challenge + `";</script>`); ok {
		t.Fatal("FindChallenge accepted a single identifier")
	}
	if _, ok := FindChallenge("<html>no challenge</html>"); ok {
		t.Fatal("FindChallenge accepted a page without a challenge")
	}
	if _, ok := FindChallenge(`<script>window.a.b = "abc";</script>`); ok {
		t.Fatal("FindChallenge accepted a short challenge")
	}
}

func TestFindKey0(t *testing.T) {
	keyBytes := []byte{0x01, 0x1F, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD,
		0xEF, 0x00, 0x11, 0x22, 0x33, 0x44, 0xAA, 0xBB}
	script := "var k=[" + testsupport.EscapedKeyLiteral(keyBytes) + `,""];`

	key, ok := FindKey0([]string{script})
	if !ok {
		t.Fatal("FindKey0 did not find the key")
	}
	if got := hex.EncodeToString(key[:]); got != hex.EncodeToString(keyBytes) {
		t.Fatalf("key = %s, want %s", got, hex.EncodeToString(keyBytes))
	}
}

func TestFindKey0RejectsInvalidLiterals(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{
			name:   "printable only",
			script: `var k=["abcdefghijklmnop",""];`,
		},
		{
			name:   "wrong length",
			script: `var k=["\x01abcdefghijklm",""];`,
		},
		{
			name:   "no literal",
			script: `var nothing=1;`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := FindKey0([]string{test.script}); ok {
				t.Fatal("FindKey0 accepted an invalid literal")
			}
		})
	}
}

func TestFindKey0FallsBackWithoutArrayMarker(t *testing.T) {
	keyBytes := []byte{0x10, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0x00, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE}
	script := "var k=" + testsupport.EscapedKeyLiteral(keyBytes) + ";"

	key, ok := FindKey0([]string{script})
	if !ok {
		t.Fatal("FindKey0 did not accept the fallback literal")
	}
	if got := hex.EncodeToString(key[:]); got != hex.EncodeToString(keyBytes) {
		t.Fatalf("key = %s, want %s", got, hex.EncodeToString(keyBytes))
	}
}

func TestExtractScripts(t *testing.T) {
	const html = "<html><head>" +
		`<script src="/a.js"></script>` +
		`<script>inline1()</script>` +
		`<script src="https://x.test/b.js" async></script>` +
		"</head></html>"

	scripts := ExtractScripts(html, "https://site.test/dir/page.html")
	if len(scripts) != 3 {
		t.Fatalf("script count = %d, want 3", len(scripts))
	}
	if scripts[0].URL != "https://site.test/a.js" {
		t.Fatalf("script 0 url = %q", scripts[0].URL)
	}
	if scripts[0].Inline != "" {
		t.Fatalf("external script has inline code %q", scripts[0].Inline)
	}
	if scripts[1].IsExternal() {
		t.Fatal("inline script reported as external")
	}
	if !strings.Contains(scripts[1].Inline, "inline1()") {
		t.Fatalf("inline code = %q", scripts[1].Inline)
	}
	if scripts[2].URL != "https://x.test/b.js" {
		t.Fatalf("script 2 url = %q", scripts[2].URL)
	}
}

func TestScriptAttributes(t *testing.T) {
	single := `<script type='text/javascript' src = '/c.js' defer></script>`
	scripts := ExtractScripts(single, "https://s.test/p")
	if len(scripts) != 1 || scripts[0].URL != "https://s.test/c.js" {
		t.Fatalf("single-quoted src = %+v", scripts)
	}

	dataSrc := `<script data-src="/no.js">x()</script>`
	scripts = ExtractScripts(dataSrc, "https://s.test/p")
	if len(scripts) != 1 || scripts[0].IsExternal() {
		t.Fatalf("data-src parsed as src: %+v", scripts)
	}

	unquoted := `<script src=/plain.js></script>`
	scripts = ExtractScripts(unquoted, "https://s.test/p")
	if len(scripts) != 1 || scripts[0].URL != "https://s.test/plain.js" {
		t.Fatalf("unquoted src = %+v", scripts)
	}

	uppercase := `<SCRIPT SRC="/up.js"></SCRIPT>`
	scripts = ExtractScripts(uppercase, "https://s.test/p")
	if len(scripts) != 1 || scripts[0].URL != "https://s.test/up.js" {
		t.Fatalf("uppercase tag = %+v", scripts)
	}
}

func TestResolveURL(t *testing.T) {
	tests := []struct {
		base     string
		relative string
		want     string
	}{
		{"https://site.test/dir/page.html", "/a.js", "https://site.test/a.js"},
		{"https://site.test/dir/page.html", "b.js", "https://site.test/dir/b.js"},
		{"https://site.test/dir/page.html", "https://x.test/c.js", "https://x.test/c.js"},
		{"not-a-url", "/a.js", "/a.js"},
	}
	for _, test := range tests {
		if got := ResolveURL(test.base, test.relative); got != test.want {
			t.Fatalf("ResolveURL(%q, %q) = %q, want %q",
				test.base, test.relative, got, test.want)
		}
	}
}
