package tspdnative

// Cookie is one name/value pair from the session jar.
type Cookie struct {
	Name  string
	Value string
}

// Result is the outcome of one solve.
type Result struct {
	// Solved is true when the edge issued TSPD_101 after the answer cookie.
	Solved bool

	// HadChallenge is true when the first response carried a TSPD challenge.
	HadChallenge bool

	// CookieString is the full Cookie request header value in insertion
	// order: "name=value; name2=value2".
	CookieString string

	// TSPD101 is the cleared cookie value when Solved is true.
	TSPD101 string

	// AnswerCookieName is the natively built answer cookie name, for example
	// "TSaa03129d075".
	AnswerCookieName string

	// HTMLLength is the byte length of HTML.
	HTMLLength int

	// HTML is the second response body, or the first body when the page was
	// not challenged.
	HTML string

	// Cookies is a snapshot of the jar in insertion order.
	Cookies []Cookie
}
