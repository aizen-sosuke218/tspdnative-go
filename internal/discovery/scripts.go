package discovery

import (
	"net/url"
	"strings"
)

const scriptTag = "<script"

// ExtractScripts scans HTML in document order and returns every script
// element. External sources are resolved against baseURL.
func ExtractScripts(html, baseURL string) []Script {
	scripts := make([]Script, 0, 8)
	offset := 0

	for offset < len(html) {
		index := indexFold(html, scriptTag, offset)
		if index < 0 {
			break
		}

		openEnd := findTagEnd(html, index+len(scriptTag))
		if openEnd < 0 {
			break
		}

		src, hasSrc := attributeValue(html, index, openEnd, "src")
		contentStart := openEnd + 1
		closeIndex := indexFold(html, "</script", contentStart)

		contentEnd := len(html)
		if closeIndex >= 0 {
			contentEnd = closeIndex
		}

		if hasSrc {
			scripts = append(scripts, Script{URL: ResolveURL(baseURL, src)})
		} else {
			code := html[contentStart:contentEnd]
			if strings.TrimSpace(code) != "" {
				scripts = append(scripts, Script{Inline: code})
			}
		}

		if closeIndex < 0 {
			offset = len(html)
		} else {
			offset = closeIndex + len("</script")
		}
	}

	return scripts
}

// ResolveURL resolves relative against baseURL. If baseURL is not absolute,
// relative is returned unchanged.
func ResolveURL(baseURL, relative string) string {
	base, err := url.Parse(baseURL)
	if err != nil || !base.IsAbs() {
		return relative
	}
	reference, err := url.Parse(relative)
	if err != nil {
		return relative
	}
	return base.ResolveReference(reference).String()
}

// findTagEnd returns the index of the closing '>' of an open tag, skipping
// quoted attribute values. It returns -1 when the tag is not terminated.
func findTagEnd(html string, start int) int {
	var quote byte
	for i := start; i < len(html); i++ {
		character := html[i]
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '"', '\'':
			quote = character
		case '>':
			return i
		}
	}
	return -1
}

// attributeValue reads an attribute from an open tag. It supports double
// quoted, single quoted and unquoted values.
func attributeValue(html string, tagStart, tagEnd int, name string) (string, bool) {
	for i := tagStart + len(scriptTag); i < tagEnd; i++ {
		if asciiLower(html[i]) != name[0] || !matchesAttributeName(html, i, tagEnd, name) {
			continue
		}

		position := i + len(name)
		for position < tagEnd && isHTMLSpace(html[position]) {
			position++
		}
		if position >= tagEnd || html[position] != '=' {
			continue
		}
		position++
		for position < tagEnd && isHTMLSpace(html[position]) {
			position++
		}
		if position >= tagEnd {
			return "", false
		}

		if quote := html[position]; quote == '"' || quote == '\'' {
			end := strings.IndexByte(html[position+1:tagEnd], quote)
			if end < 0 {
				return "", false
			}
			return html[position+1 : position+1+end], true
		}

		valueStart := position
		for position < tagEnd && !isHTMLSpace(html[position]) && html[position] != '>' {
			position++
		}
		return html[valueStart:position], true
	}
	return "", false
}

func matchesAttributeName(source string, at, end int, name string) bool {
	if at > 0 && isNameChar(source[at-1]) {
		return false
	}
	if at+len(name) > end {
		return false
	}
	for i := 0; i < len(name); i++ {
		if asciiLower(source[at+i]) != name[i] {
			return false
		}
	}
	return at+len(name) >= end || !isNameChar(source[at+len(name)])
}

func isNameChar(character byte) bool {
	return isASCIILetterOrDigit(character) ||
		character == '-' || character == '_' || character == ':'
}

func isHTMLSpace(character byte) bool {
	switch character {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}

func isASCIILetter(character byte) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z')
}

func isASCIILetterOrDigit(character byte) bool {
	return isASCIILetter(character) || (character >= '0' && character <= '9')
}

func asciiLower(character byte) byte {
	if character >= 'A' && character <= 'Z' {
		return character + ('a' - 'A')
	}
	return character
}

// indexFold is an ASCII case-insensitive strings.Index.
func indexFold(haystack, needle string, start int) int {
	if start < 0 {
		start = 0
	}
	last := len(haystack) - len(needle)
	for i := start; i <= last; i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if asciiLower(haystack[i+j]) != asciiLower(needle[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
