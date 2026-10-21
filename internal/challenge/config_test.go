package challenge

import (
	"errors"
	"strings"
	"testing"

	"github.com/aizen-sosuke218/tspdnative-go/internal/testsupport"
)

func TestParseSyntheticLayout(t *testing.T) {
	raw := testsupport.Bobcmn(5, 1, 0xAA03129D, 0, 2, "a", "b", "f7", "TS")

	config, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := config.Len(); got != 25 {
		t.Fatalf("entries = %d, want 25", got)
	}
	if got := config.Type(0); got != KindBool {
		t.Fatalf("type 0 = %v, want bool", got)
	}
	if config.Bool(0) {
		t.Fatal("bool 0 = true, want false")
	}
	if got := config.Type(7); got != KindNumber {
		t.Fatalf("type 7 = %v, want number", got)
	}
	if got := config.Number(7); got != 5 {
		t.Fatalf("number 7 = %d, want 5", got)
	}
	if got := config.Number(11); got != 0xAA03129D {
		t.Fatalf("number 11 = %#x, want 0xaa03129d", got)
	}
	if got := config.Text(15); got != "a" {
		t.Fatalf("text 15 = %q, want a", got)
	}
	if got := config.Text(22); got != "TS" {
		t.Fatalf("text 22 = %q, want TS", got)
	}
	if got := config.TextOr(17, "fallback"); got != "fallback" {
		t.Fatalf("TextOr(17) = %q, want fallback", got)
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{
			name: "too short",
			raw:  strings.Repeat("10", 6),
			want: ErrTooShort,
		},
		{
			name: "truncated bool",
			raw:  strings.Repeat("10", 7) + "1",
			want: ErrTruncated,
		},
		{
			name: "unknown type",
			raw:  strings.Repeat("10", 7) + "x",
			want: ErrUnknownType,
		},
		{
			name: "truncated number",
			raw:  strings.Repeat("10", 7) + "2abc",
			want: ErrTruncated,
		},
		{
			name: "truncated string",
			raw:  strings.Repeat("10", 7) + "300000010abc",
			want: ErrTruncated,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.raw); !errors.Is(err, test.want) {
				t.Fatalf("Parse error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	for _, test := range []struct {
		kind Kind
		want string
	}{
		{KindBool, "bool"},
		{KindNumber, "number"},
		{KindString, "string"},
		{Kind(99), "unknown"},
	} {
		if got := test.kind.String(); got != test.want {
			t.Fatalf("Kind(%d).String() = %q, want %q", test.kind, got, test.want)
		}
	}
}
