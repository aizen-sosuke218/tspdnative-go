package tspdnative

import "time"

const (
	// DefaultTarget is the page protected by the TSPD challenge.
	DefaultTarget = "https://onlineservices.immigration.govt.nz/"

	// DefaultUserAgent is the browser profile used for HTTP headers and
	// fingerprint field 11.
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/126.0.0.0 Safari/537.36"

	// DefaultLanguage is the locale used for fingerprint field 10.
	DefaultLanguage = "en-NZ"

	// DefaultAcceptLanguage is the Accept-Language header paired with
	// DefaultLanguage.
	DefaultAcceptLanguage = "en-NZ,en-US;q=0.9,en;q=0.8"
)

const (
	// defaultSolveTimeout bounds the complete request pipeline.
	defaultSolveTimeout = 25 * time.Second

	// defaultScriptTimeout bounds one sensor-script download.
	defaultScriptTimeout = 10 * time.Second

	// maxParallelScripts limits concurrent sensor-script requests.
	maxParallelScripts = 6
)
