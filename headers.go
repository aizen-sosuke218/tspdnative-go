package tspdnative

import (
	"net/http"
	"strings"
)

// buildHeaders returns the browser-shaped request headers used for both page
// and sensor-script requests.
func buildHeaders(options Options) http.Header {
	chrome := chromeMajor(options.UserAgent)
	acceptLanguage := options.AcceptLanguage
	if acceptLanguage == "" {
		acceptLanguage = deriveAcceptLanguage(options.Language)
	}

	headers := make(http.Header, 11)
	headers.Set("User-Agent", options.UserAgent)
	headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,"+
		"image/avif,image/webp,image/apng,*/*;q=0.8")
	headers.Set("Accept-Language", acceptLanguage)
	headers.Set("Upgrade-Insecure-Requests", "1")
	headers.Set("Sec-Fetch-Dest", "document")
	headers.Set("Sec-Fetch-Mode", "navigate")
	headers.Set("Sec-Fetch-Site", "none")
	headers.Set("Sec-Fetch-User", "?1")
	headers.Set("Sec-CH-UA", `"Chromium";v="`+chrome+`", "Google Chrome";v="`+chrome+`", "Not-A.Brand";v="99"`)
	headers.Set("Sec-CH-UA-Platform", platformOf(options.UserAgent))
	return headers
}

// deriveAcceptLanguage expands a locale into a browser-like header. The
// default locale uses the profile captured from the reference browser.
func deriveAcceptLanguage(language string) string {
	if strings.EqualFold(language, DefaultLanguage) {
		return DefaultAcceptLanguage
	}
	if !strings.Contains(language, "-") {
		return language
	}
	return language + ",en;q=0.9"
}

// chromeMajor extracts the Chrome major version from a user agent.
func chromeMajor(userAgent string) string {
	const marker = "Chrome/"
	start := strings.Index(userAgent, marker)
	if start < 0 {
		return "126"
	}
	start += len(marker)
	end := start
	for end < len(userAgent) && userAgent[end] >= '0' && userAgent[end] <= '9' {
		end++
	}
	if end == start {
		return "126"
	}
	return userAgent[start:end]
}

// platformOf maps the user-agent platform to a Sec-CH-UA-Platform value.
func platformOf(userAgent string) string {
	switch {
	case strings.Contains(userAgent, "Windows"):
		return `"Windows"`
	case strings.Contains(userAgent, "Macintosh"):
		return `"macOS"`
	case strings.Contains(userAgent, "Android"):
		return `"Android"`
	default:
		return `"Linux"`
	}
}
