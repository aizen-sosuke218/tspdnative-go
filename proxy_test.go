package tspdnative

import "testing"

func TestNormalizeProxy(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"host:8080:user:pass", "http://user:pass@host:8080"},
		{"user:pass@host:8080", "http://user:pass@host:8080"},
		{"host:8080", "http://host:8080"},
		{"host:8080:user", "http://user@host:8080"},
		{"  http://host:8080  ", "http://host:8080"},
		{"socks5://1.2.3.4:1080", "socks5://1.2.3.4:1080"},
		{"", ""},
	}
	for _, test := range tests {
		if got := NormalizeProxy(test.raw); got != test.want {
			t.Fatalf("NormalizeProxy(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestProxyFromEnvOrder(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://https-proxy:8080")
	t.Setenv("https_proxy", "http://lower-proxy:8080")
	t.Setenv("HTTP_PROXY", "http://http-proxy:8080")
	t.Setenv("http_proxy", "http://lower-http-proxy:8080")

	if got := ProxyFromEnv(); got != "http://https-proxy:8080" {
		t.Fatalf("ProxyFromEnv() = %q", got)
	}

	t.Setenv("HTTPS_PROXY", "")
	if got := ProxyFromEnv(); got != "http://lower-proxy:8080" {
		t.Fatalf("ProxyFromEnv() after clearing HTTPS_PROXY = %q", got)
	}
}

func TestParseProxyURL(t *testing.T) {
	if parsed, err := parseProxyURL(""); err != nil || parsed != nil {
		t.Fatalf("parseProxyURL(empty) = %v, %v", parsed, err)
	}
	if _, err := parseProxyURL("ftp://1.2.3.4:21"); err == nil {
		t.Fatal("parseProxyURL accepted an unsupported scheme")
	}
	if _, err := parseProxyURL("http://"); err == nil {
		t.Fatal("parseProxyURL accepted a URL without a host")
	}
}
