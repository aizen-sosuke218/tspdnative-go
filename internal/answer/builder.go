package answer

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/aizen-sosuke218/tspdnative-go/internal/challenge"
	"github.com/aizen-sosuke218/tspdnative-go/internal/discovery"
	"github.com/aizen-sosuke218/tspdnative-go/internal/securemsg"
)

// Build constructs the answer cookie from the challenge page and the fetched
// external script bodies.
func Build(html string, externalScripts []string, language, userAgent string) (Cookie, error) {
	bobcmnRaw, ok := discovery.FindBobcmn(html)
	if !ok {
		return Cookie{}, ErrBobcmnNotFound
	}

	challengeHex, ok := discovery.FindChallenge(html)
	if !ok {
		return Cookie{}, ErrChallengeNotFound
	}

	key, ok := discovery.FindKey0(externalScripts)
	if !ok {
		return Cookie{}, ErrKeyNotFound
	}

	config, err := challenge.Parse(bobcmnRaw)
	if err != nil {
		return Cookie{}, fmt.Errorf("answer: parse bobcmn: %w", err)
	}
	if config.Len() < requiredEntries {
		return Cookie{}, fmt.Errorf("%w: %d entries", ErrConfigTooShort, config.Len())
	}

	// Challenge (scope 01, full path): unseal and keep the cached context.
	challengeMessage, err := hex.DecodeString(challengeHex)
	if err != nil {
		return Cookie{}, fmt.Errorf("answer: decode challenge hex: %w", err)
	}
	challengePlain, challengeContext, err := securemsg.Unseal(
		challengeMessage, ScopeChallenge, key, false,
	)
	if err != nil {
		return Cookie{}, fmt.Errorf("answer: unseal challenge: %w", err)
	}

	// Inner message (scope AB, shortcut): only the sealed prefix is needed.
	innerMessage, _, err := securemsg.Unseal(challengePlain, ScopeInner, key, true)
	if err != nil {
		return Cookie{}, fmt.Errorf("answer: read challenge context: %w", err)
	}
	contextHex := hex.EncodeToString(innerMessage)

	// Fingerprint seal (scope 04) bound to the challenge context.
	fingerprint, err := BuildFingerprint(language, userAgent)
	if err != nil {
		return Cookie{}, err
	}
	seal, err := securemsg.Seal(fingerprint, ScopeFingerprint, challengeContext, key)
	if err != nil {
		return Cookie{}, fmt.Errorf("answer: seal fingerprint: %w", err)
	}

	name, err := BuildName(config)
	if err != nil {
		return Cookie{}, err
	}
	value, err := BuildValue(config, contextHex, hex.EncodeToString(seal))
	if err != nil {
		return Cookie{}, err
	}

	return Cookie{Name: name, Value: value}, nil
}

// BuildName returns the cookie name:
//
//	bobcmn[22] + zero-padded 8-hex(bobcmn[11]) + zero-padded 3(75 + bobcmn[13])
//
// The current build yields names such as "TSaa03129d075".
func BuildName(config *challenge.Config) (string, error) {
	if config.Len() < requiredEntries {
		return "", fmt.Errorf("%w: %d entries", ErrConfigTooShort, config.Len())
	}

	identifier := strconv.FormatUint(uint64(config.Number(11)), 16)
	suffix := strconv.FormatUint(uint64(nameSuffixBase)+uint64(config.Number(13)), 10)

	return config.Text(22) + padLeftZeros(identifier, 8) + padLeftZeros(suffix, 3), nil
}

// BuildValue serializes the answer object. Numbers are little-endian hex,
// string lengths are 2-byte little-endian hex and string content is raw. The
// result starts with the fixed "0502c101" prefix for the live build.
func BuildValue(config *challenge.Config, contextHex, sealHex string) (string, error) {
	if config.Len() < requiredEntries {
		return "", fmt.Errorf("%w: %d entries", ErrConfigTooShort, config.Len())
	}

	var builder strings.Builder
	builder.Grow(1024)

	appendNumberHex(&builder, config.Number(7), 1)
	appendNumberHex(&builder, config.Number(14), 1)

	if err := appendSizedHex(&builder, contextHex+":"+sealHex); err != nil {
		return "", err
	}
	if err := appendSizedHex(&builder, ""); err != nil {
		return "", err
	}

	field5 := uint32(0)
	if config.Number(10) == field5Compare {
		field5 = 1
	}
	appendNumberHex(&builder, field5, 1)

	if err := appendSizedHex(&builder, config.TextOr(15, "0")); err != nil {
		return "", err
	}
	if err := appendSizedHex(&builder, config.Text(21)); err != nil {
		return "", err
	}
	if err := appendSizedHex(&builder, config.TextOr(16, "0")); err != nil {
		return "", err
	}

	return builder.String(), nil
}

const hexDigits = "0123456789abcdef"

// appendNumberHex writes value as little-endian hex with a fixed byte width.
func appendNumberHex(builder *strings.Builder, value uint32, size int) {
	for i := 0; i < size; i++ {
		current := byte(value >> (8 * i))
		builder.WriteByte(hexDigits[current>>4])
		builder.WriteByte(hexDigits[current&0x0F])
	}
}

// appendSizedHex writes the 2-byte little-endian length as hex, then content.
func appendSizedHex(builder *strings.Builder, content string) error {
	if len(content) > 0xFFFF {
		return fmt.Errorf("%w: %d chars (max 65535)", ErrFieldTooLong, len(content))
	}
	length := uint16(len(content))
	builder.WriteByte(hexDigits[byte(length)>>4])
	builder.WriteByte(hexDigits[byte(length)&0x0F])
	builder.WriteByte(hexDigits[byte(length>>8)>>4])
	builder.WriteByte(hexDigits[byte(length>>8)&0x0F])
	builder.WriteString(content)
	return nil
}

func padLeftZeros(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return strings.Repeat("0", width-len(value)) + value
}
