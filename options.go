package tspdnative

import (
	"log/slog"
	"time"
)

// Options configures a Solver. The zero value is valid and uses the dedicated
// immigration target with the built-in Chrome profile.
type Options struct {
	// Target is the challenged page. Defaults to DefaultTarget.
	Target string

	// UserAgent is used for HTTP requests and fingerprint field 11.
	// Defaults to DefaultUserAgent.
	UserAgent string

	// Language is the locale written to fingerprint field 10.
	// Defaults to DefaultLanguage.
	Language string

	// AcceptLanguage overrides the Accept-Language header. When empty it is
	// derived from Language.
	AcceptLanguage string

	// Proxy accepts a URL (http, https, socks5, socks5h) or one of the common
	// provider shapes:
	//
	//	host:port:user:pass
	//	user:pass@host:port
	//
	// An empty value means a direct connection. Call ProxyFromEnv to opt into
	// the standard environment variables.
	Proxy string

	// Timeout bounds the complete solve. Defaults to 25s.
	Timeout time.Duration

	// Debug enables debug logging to stderr when Logger is nil.
	Debug bool

	// Logger receives debug pipeline events. When nil and Debug is true, a
	// text logger writing to stderr is created.
	Logger *slog.Logger
}

func (o Options) withDefaults() Options {
	if o.Target == "" {
		o.Target = DefaultTarget
	}
	if o.UserAgent == "" {
		o.UserAgent = DefaultUserAgent
	}
	if o.Language == "" {
		o.Language = DefaultLanguage
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultSolveTimeout
	}
	return o
}
