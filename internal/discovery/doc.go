// Package discovery locates the per-session TSPD inputs in the challenge page
// and its sensor scripts.
//
// All scanning is plain text based: no regular expressions, no JavaScript
// parser. That keeps the native fast path independent of the obfuscated build.
package discovery

// Script is one <script> element in document order. Exactly one of URL and
// Inline is populated.
type Script struct {
	// URL is the absolute source URL of an external script.
	URL string

	// Inline is the source code of an inline script.
	Inline string
}

// IsExternal reports whether the script is loaded from a URL.
func (s Script) IsExternal() bool { return s.URL != "" }
