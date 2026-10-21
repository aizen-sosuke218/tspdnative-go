package tspdnative

import (
	"testing"
)

func TestDeriveAcceptLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{DefaultLanguage, DefaultAcceptLanguage},
		{"en-nz", DefaultAcceptLanguage},
		{"de-DE", "de-DE,en;q=0.9"},
		{"en", "en"},
	}
	for _, test := range tests {
		if got := deriveAcceptLanguage(test.language); got != test.want {
			t.Fatalf("deriveAcceptLanguage(%q) = %q, want %q",
				test.language, got, test.want)
		}
	}
}

func TestChromeMajor(t *testing.T) {
	tests := []struct {
		userAgent string
		want      string
	}{
		{DefaultUserAgent, "126"},
		{"no chrome here", "126"},
		{"Mozilla/5.0 Chrome/131.0.0.0 Safari/537.36", "131"},
		{"Mozilla/5.0 Chrome/", "126"},
	}
	for _, test := range tests {
		if got := chromeMajor(test.userAgent); got != test.want {
			t.Fatalf("chromeMajor(%q) = %q, want %q",
				test.userAgent, got, test.want)
		}
	}
}

func TestPlatformOf(t *testing.T) {
	tests := []struct {
		userAgent string
		want      string
	}{
		{DefaultUserAgent, `"Windows"`},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", `"macOS"`},
		{"Mozilla/5.0 (Linux; Android 14)", `"Android"`},
		{"Mozilla/5.0 (X11; Linux x86_64)", `"Linux"`},
	}
	for _, test := range tests {
		if got := platformOf(test.userAgent); got != test.want {
			t.Fatalf("platformOf(%q) = %q, want %q",
				test.userAgent, got, test.want)
		}
	}
}

func TestBuildHeaders(t *testing.T) {
	headers := buildHeaders(Options{
		UserAgent: DefaultUserAgent,
		Language:  DefaultLanguage,
	})

	checks := map[string]string{
		"User-Agent":                DefaultUserAgent,
		"Accept-Language":           DefaultAcceptLanguage,
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Sec-CH-UA-Platform":        `"Windows"`,
		"Sec-CH-UA":                 `"Chromium";v="126", "Google Chrome";v="126", "Not-A.Brand";v="99"`,
	}
	for name, want := range checks {
		if got := headers.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}

	custom := buildHeaders(Options{
		UserAgent:      DefaultUserAgent,
		Language:       "de-DE",
		AcceptLanguage: "de-DE,de;q=0.9",
	})
	if got := custom.Get("Accept-Language"); got != "de-DE,de;q=0.9" {
		t.Fatalf("custom Accept-Language = %q", got)
	}
}
