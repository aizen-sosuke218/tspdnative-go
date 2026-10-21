// Package challenge parses the TSPD bobcmn session configuration embedded in
// the challenge page.
//
// bobcmn is a compact TLV stream:
//
//	'1' + 1 digit                 -> bool
//	'2' + 8 hex digits (BE)       -> number
//	'3' + 8 hex digits + content  -> string
//
// Current builds contain 25 entries. The semantic index map is randomised per
// build, but the indices themselves are stable.
//
// Payloads in current builds are ASCII, so the parser indexes the raw string
// by byte. A deployment using non-ASCII values would need UTF-16 length
// handling.
package challenge

import (
	"errors"
	"fmt"
	"strconv"
)

// Kind is the bobcmn entry type.
type Kind uint8

const (
	// KindBool is a boolean entry encoded as "1" plus one digit.
	KindBool Kind = 1

	// KindNumber is an unsigned 32-bit entry encoded as "2" plus 8 hex digits.
	KindNumber Kind = 2

	// KindString is a length-prefixed string entry encoded as "3".
	KindString Kind = 3
)

// String returns the entry kind name.
func (k Kind) String() string {
	switch k {
	case KindBool:
		return "bool"
	case KindNumber:
		return "number"
	case KindString:
		return "string"
	default:
		return "unknown"
	}
}

// Entry is one decoded bobcmn value. Only the field matching Kind is set.
type Entry struct {
	Kind   Kind
	Bool   bool
	Number uint32
	Text   string
}

// Config is an ordered bobcmn entry list.
type Config struct {
	entries []Entry
}

var (
	// ErrTooShort reports a bobcmn string below the minimum length.
	ErrTooShort = errors.New("challenge: bobcmn too short")

	// ErrTruncated reports an entry that extends past the input.
	ErrTruncated = errors.New("challenge: truncated entry")

	// ErrUnknownType reports an unsupported entry marker.
	ErrUnknownType = errors.New("challenge: unknown entry type")
)

// Parse decodes a raw bobcmn string.
func Parse(raw string) (*Config, error) {
	if len(raw) < 14 {
		return nil, fmt.Errorf("%w: %d chars", ErrTooShort, len(raw))
	}

	entries := make([]Entry, 0, 25)
	for offset := 0; offset < len(raw); {
		switch raw[offset] {
		case '1':
			if offset+2 > len(raw) {
				return nil, fmt.Errorf("%w: bool at offset %d", ErrTruncated, offset)
			}
			entries = append(entries, Entry{
				Kind: KindBool,
				Bool: raw[offset+1] == '1',
			})
			offset += 2

		case '2':
			if offset+9 > len(raw) {
				return nil, fmt.Errorf("%w: number at offset %d", ErrTruncated, offset)
			}
			value, err := strconv.ParseUint(raw[offset+1:offset+9], 16, 32)
			if err != nil {
				return nil, fmt.Errorf("challenge: invalid number at offset %d: %w", offset, err)
			}
			entries = append(entries, Entry{
				Kind:   KindNumber,
				Number: uint32(value),
			})
			offset += 9

		case '3':
			if offset+9 > len(raw) {
				return nil, fmt.Errorf("%w: string length at offset %d", ErrTruncated, offset)
			}
			length, err := strconv.ParseUint(raw[offset+1:offset+9], 16, 32)
			if err != nil {
				return nil, fmt.Errorf("challenge: invalid string length at offset %d: %w", offset, err)
			}
			contentStart := offset + 9
			if uint64(length) > uint64(len(raw)-contentStart) {
				return nil, fmt.Errorf(
					"%w: string at offset %d declares %d bytes, %d remain",
					ErrTruncated, offset, length, len(raw)-contentStart,
				)
			}
			contentEnd := contentStart + int(length)
			entries = append(entries, Entry{
				Kind: KindString,
				Text: raw[contentStart:contentEnd],
			})
			offset = contentEnd

		default:
			return nil, fmt.Errorf("%w: %q at offset %d", ErrUnknownType, raw[offset], offset)
		}
	}

	return &Config{entries: entries}, nil
}

// Len returns the number of decoded entries.
func (c *Config) Len() int { return len(c.entries) }

// Entry returns the entry at index.
func (c *Config) Entry(index int) Entry { return c.entries[index] }

// Type returns the entry kind at index.
func (c *Config) Type(index int) Kind { return c.entries[index].Kind }

// Bool returns the value of a boolean entry.
func (c *Config) Bool(index int) bool { return c.entries[index].Bool }

// Number returns the value of a numeric entry.
func (c *Config) Number(index int) uint32 { return c.entries[index].Number }

// Text returns the value of a string entry.
func (c *Config) Text(index int) string { return c.entries[index].Text }

// TextOr returns the string entry at index, or fallback when the entry is not
// a non-empty string. It mirrors the answer builder's defaulting rule.
func (c *Config) TextOr(index int, fallback string) string {
	entry := c.entries[index]
	if entry.Kind != KindString || entry.Text == "" {
		return fallback
	}
	return entry.Text
}
