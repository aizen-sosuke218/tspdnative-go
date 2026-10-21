package tspdnative

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ProxyFromEnv returns the first non-empty standard proxy variable in the
// order HTTPS_PROXY, https_proxy, HTTP_PROXY, http_proxy. It returns an empty
// string when no proxy is configured.
//
// The library never reads the environment implicitly; pass this value to
// Options.Proxy when environment-based configuration is desired.
func ProxyFromEnv() string {
	for _, name := range [...]string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

// NormalizeProxy converts the common provider shapes into a proxy URL:
//
//	host:port:user:pass  -> http://user:pass@host:port
//	user:pass@host:port  -> http://user:pass@host:port
//	host:port            -> http://host:port
//
// Values that already contain a scheme are returned unchanged. An empty input
// yields an empty string. The function does not validate the scheme; the
// solver accepts http, https, socks5 and socks5h.
func NormalizeProxy(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		return trimmed
	}
	if strings.Contains(trimmed, "@") {
		return "http://" + trimmed
	}

	parts := strings.SplitN(trimmed, ":", 4)
	switch {
	case len(parts) == 4 && parts[1] != "" && parts[2] != "":
		return fmt.Sprintf("http://%s:%s@%s:%s", parts[2], parts[3], parts[0], parts[1])
	case len(parts) == 3 && parts[1] != "" && parts[2] != "":
		return fmt.Sprintf("http://%s@%s:%s", parts[2], parts[0], parts[1])
	case len(parts) == 2 && parts[1] != "":
		return fmt.Sprintf("http://%s:%s", parts[0], parts[1])
	default:
		return trimmed
	}
}

// parseProxyURL normalizes and validates a proxy specification.
func parseProxyURL(raw string) (*url.URL, error) {
	normalized := NormalizeProxy(raw)
	if normalized == "" {
		return nil, nil
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL %q: %w", normalized, err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf(
			"proxy scheme not supported (use http:// https:// socks5:// socks5h://): %s",
			normalized,
		)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid proxy URL %q: missing host", normalized)
	}
	return parsed, nil
}
