package answer

import (
	"encoding/binary"
	"fmt"
)

// featureBitmask is the Modernizr feature-result bitmask ([12]) observed in
// accepted runs. The sensor packs the same table for this fingerprint profile.
var featureBitmask = [...]byte{
	0x86, 0x00, 0x00, 0x01, 0x01, 0x30, 0x58, 0x62,
	0x60, 0x00, 0x05, 0x8F, 0xFF, 0xFF, 0xFF, 0xFF,
}

// BuildFingerprint serializes the 15 fingerprint fields using the live build's
// schema: 1 byte for fields [00..06], 4 little-endian bytes for [07..09] and
// [13], and 1-byte length-prefixed strings for the remaining fields.
func BuildFingerprint(language, userAgent string) ([]byte, error) {
	fingerprint := make([]byte, 0, 160)

	// [00..06] module results: headless combo, callPhantom, history,
	// referrer, Flash version, WebGL and UA marker.
	fingerprint = append(fingerprint, 0, 0, 0, 0, 0, 1, 0)

	fingerprint = binary.LittleEndian.AppendUint32(fingerprint, 0)  // [07] webdriver/selenium probe
	fingerprint = binary.LittleEndian.AppendUint32(fingerprint, 0)  // [08] device-motion collector
	fingerprint = binary.LittleEndian.AppendUint32(fingerprint, 56) // [09] non-native/missing API count

	var err error
	if fingerprint, err = appendLengthPrefixed(fingerprint, latin1Bytes(language)); err != nil {
		return nil, fmt.Errorf("fingerprint field 10: %w", err)
	}
	if fingerprint, err = appendLengthPrefixed(fingerprint, latin1Bytes(userAgent)); err != nil {
		return nil, fmt.Errorf("fingerprint field 11: %w", err)
	}
	if fingerprint, err = appendLengthPrefixed(fingerprint, featureBitmask[:]); err != nil {
		return nil, fmt.Errorf("fingerprint field 12: %w", err)
	}

	fingerprint = binary.LittleEndian.AppendUint32(fingerprint, 0) // [13] extension bitmask

	if fingerprint, err = appendLengthPrefixed(fingerprint, nil); err != nil { // [14] proxy/sniffer probes
		return nil, fmt.Errorf("fingerprint field 14: %w", err)
	}
	return fingerprint, nil
}

func appendLengthPrefixed(target, data []byte) ([]byte, error) {
	if len(data) > 0xFF {
		return nil, fmt.Errorf("%w: %d bytes (max 255)", ErrFieldTooLong, len(data))
	}
	target = append(target, byte(len(data)))
	return append(target, data...), nil
}

// latin1Bytes mirrors Encoding.Latin1: one byte per rune and '?' for runes
// outside the Latin-1 range.
func latin1Bytes(value string) []byte {
	out := make([]byte, 0, len(value))
	for _, character := range value {
		if character > 0xFF {
			out = append(out, '?')
			continue
		}
		out = append(out, byte(character))
	}
	return out
}
