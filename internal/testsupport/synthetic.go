// Package testsupport provides synthetic challenge inputs and fixture paths
// shared by the test suites.
package testsupport

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// UserAgent matches the library default profile.
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/126.0.0.0 Safari/537.36"

	// Language matches the library default locale.
	Language = "en-NZ"
)

// Bobcmn builds a 25-entry bobcmn string with the live build's field layout.
func Bobcmn(
	number7 uint32,
	number10 uint32,
	number11 uint32,
	number13 uint32,
	number14 uint32,
	text15 string,
	text16 string,
	text21 string,
	text22 string,
) string {
	var builder strings.Builder
	builder.Grow(400)

	for index := 0; index < 25; index++ {
		switch index {
		case 7:
			appendNumber(&builder, number7)
		case 10:
			appendNumber(&builder, number10)
		case 11:
			appendNumber(&builder, number11)
		case 13:
			appendNumber(&builder, number13)
		case 14:
			appendNumber(&builder, number14)
		case 15:
			appendString(&builder, text15)
		case 16:
			appendString(&builder, text16)
		case 21:
			appendString(&builder, text21)
		case 22:
			appendString(&builder, text22)
		default:
			builder.WriteString("10") // bool false
		}
	}
	return builder.String()
}

func appendNumber(builder *strings.Builder, value uint32) {
	fmt.Fprintf(builder, "2%08x", value)
}

func appendString(builder *strings.Builder, value string) {
	fmt.Fprintf(builder, "3%08x%s", len(value), value)
}

// EscapedKeyLiteral renders key bytes as a JavaScript double-quoted literal:
// printable bytes verbatim, everything else as \xNN.
func EscapedKeyLiteral(key []byte) string {
	var builder strings.Builder
	builder.Grow(len(key)*4 + 2)
	builder.WriteByte('"')
	for _, value := range key {
		if value >= 0x20 && value <= 0x7E && value != '"' && value != '\\' {
			builder.WriteByte(value)
			continue
		}
		fmt.Fprintf(&builder, "\\x%02x", value)
	}
	builder.WriteByte('"')
	return builder.String()
}

// FixturePath returns the absolute path of a file in the repository's
// testdata directory.
func FixturePath(name string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("testdata", name)
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}
