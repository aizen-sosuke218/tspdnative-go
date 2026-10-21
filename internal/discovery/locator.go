package discovery

import (
	"strings"
	"unicode/utf8"

	"github.com/aizen-sosuke218/tspdnative-go/internal/challenge"
	"github.com/aizen-sosuke218/tspdnative-go/internal/securemsg"
)

// FindBobcmn locates the bobcmn string literal and verifies that it parses.
func FindBobcmn(html string) (string, bool) {
	search := 0
	for search < len(html) {
		index := strings.Index(html[search:], "bobcmn")
		if index < 0 {
			return "", false
		}
		index += search

		position := index + len("bobcmn")
		for position < len(html) {
			character := html[position]
			if character != '"' && character != ']' && !isHTMLSpace(character) {
				break
			}
			position++
		}

		if position < len(html) && html[position] == '=' {
			position++
			for position < len(html) && isHTMLSpace(html[position]) {
				position++
			}
			if position < len(html) && html[position] == '"' {
				if end := strings.IndexByte(html[position+1:], '"'); end >= 0 {
					candidate := html[position+1 : position+1+end]
					if config, err := challenge.Parse(candidate); err == nil && config.Len() >= 14 {
						return candidate, true
					}
				}
			}
		}

		search = index + len("bobcmn")
	}
	return "", false
}

// FindChallenge locates the hex challenge assigned through the sensor's
// window.<identifier>.<identifier> = "<hex>" pattern. At least 64 hex digits
// are required.
func FindChallenge(html string) (string, bool) {
	search := 0
	for search < len(html) {
		index := strings.Index(html[search:], "window.")
		if index < 0 {
			return "", false
		}
		index += search

		position := index + len("window.")
		if !isIdentifierStart(html, position) {
			search = position
			continue
		}

		afterFirst := skipIdentifier(html, position)
		if afterFirst >= len(html) || html[afterFirst] != '.' {
			search = afterFirst
			continue
		}

		secondStart := afterFirst + 1
		if !isIdentifierStart(html, secondStart) {
			search = secondStart
			continue
		}

		propertyEnd := skipIdentifier(html, secondStart)
		cursor := propertyEnd
		for cursor < len(html) && isHTMLSpace(html[cursor]) {
			cursor++
		}
		if cursor >= len(html) || html[cursor] != '=' {
			search = propertyEnd
			continue
		}
		cursor++
		for cursor < len(html) && isHTMLSpace(html[cursor]) {
			cursor++
		}
		if cursor >= len(html) || html[cursor] != '"' {
			search = propertyEnd
			continue
		}

		valueStart := cursor + 1
		valueEnd := valueStart
		for valueEnd < len(html) && isHexDigit(html[valueEnd]) {
			valueEnd++
		}
		if valueEnd-valueStart >= 64 {
			return html[valueStart:valueEnd], true
		}
		search = valueEnd
	}
	return "", false
}

// FindKey0 locates the per-build key in the sensor scripts: a 16-byte string
// literal with at least two non-printable bytes. The key array marker
// ("<key>","") is preferred; otherwise the first valid literal is returned.
func FindKey0(scripts []string) (securemsg.Key, bool) {
	var fallback securemsg.Key
	hasFallback := false

	for _, script := range scripts {
		offset := 0
		for offset < len(script) {
			character := script[offset]
			if character != '"' && character != '\'' {
				offset++
				continue
			}

			end, codes, ok := scanStringLiteral(script, offset)
			if !ok {
				offset++
				continue
			}

			if len(codes) == 16 {
				if key, valid := keyFromCodes(codes); valid {
					after := end
					for after < len(script) && (script[after] == ' ' || script[after] == '\t') {
						after++
					}
					if after+3 <= len(script) &&
						script[after] == ',' &&
						script[after+1] == '"' &&
						script[after+2] == '"' {
						return key, true
					}
					if !hasFallback {
						fallback = key
						hasFallback = true
					}
				}
			}

			if end <= offset {
				offset++
			} else {
				offset = end
			}
		}
	}

	if hasFallback {
		return fallback, true
	}
	return securemsg.Key{}, false
}

// scanStringLiteral decodes a JavaScript string literal starting at start.
// It returns the index just past the closing quote and the UTF-16-compatible
// character codes.
func scanStringLiteral(source string, start int) (end int, codes []int, ok bool) {
	quote := source[start]
	codes = make([]int, 0, 32)

	for i := start + 1; i < len(source); {
		character := source[i]
		if character == '\n' || character == '\r' {
			return 0, nil, false
		}

		if character == '\\' {
			i++
			if i >= len(source) {
				return 0, nil, false
			}
			switch source[i] {
			case 'x':
				value, valid := parseHex(source, i+1, 2)
				if !valid {
					return 0, nil, false
				}
				codes = append(codes, value)
				i += 3
			case 'u':
				value, valid := parseHex(source, i+1, 4)
				if !valid {
					return 0, nil, false
				}
				codes = append(codes, value)
				i += 5
			case '\n', '\r':
				i++
			default:
				codes = append(codes, simpleEscape(source[i]))
				i++
			}
			continue
		}

		if character == quote {
			return i + 1, codes, true
		}

		runeValue, size := utf8.DecodeRuneInString(source[i:])
		codes = append(codes, int(runeValue))
		i += size
	}

	return 0, nil, false
}

func keyFromCodes(codes []int) (securemsg.Key, bool) {
	var key securemsg.Key
	nonPrintable := 0
	for i, code := range codes {
		if code > 0xFF {
			return key, false
		}
		if code < 0x20 || code > 0x7E {
			nonPrintable++
		}
		key[i] = byte(code)
	}
	if nonPrintable < 2 {
		return key, false
	}
	return key, true
}

func simpleEscape(character byte) int {
	switch character {
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'v':
		return '\v'
	case '0':
		return 0
	default:
		return int(character)
	}
}

func parseHex(source string, offset, digits int) (int, bool) {
	if offset+digits > len(source) {
		return 0, false
	}
	value := 0
	for i := 0; i < digits; i++ {
		digit := hexValue(source[offset+i])
		if digit < 0 {
			return 0, false
		}
		value = value<<4 | digit
	}
	return value, true
}

func isHexDigit(character byte) bool { return hexValue(character) >= 0 }

func hexValue(character byte) int {
	switch {
	case character >= '0' && character <= '9':
		return int(character - '0')
	case character >= 'a' && character <= 'f':
		return int(character-'a') + 10
	case character >= 'A' && character <= 'F':
		return int(character-'A') + 10
	default:
		return -1
	}
}

func isIdentifierStart(source string, index int) bool {
	if index >= len(source) {
		return false
	}
	character := source[index]
	return isASCIILetter(character) || character == '_' || character == '$'
}

func skipIdentifier(source string, index int) int {
	for index < len(source) {
		character := source[index]
		if !isASCIILetterOrDigit(character) &&
			character != '_' && character != '$' {
			break
		}
		index++
	}
	return index
}
