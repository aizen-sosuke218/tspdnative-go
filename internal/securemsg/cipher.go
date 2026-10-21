package securemsg

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	// ErrKeySize reports invalid key material.
	ErrKeySize = errors.New("securemsg: key must be 16 bytes")

	// ErrBlockSize reports an invalid padding block size.
	ErrBlockSize = errors.New("securemsg: block size must be positive")

	// ErrCBCLength reports ciphertext that is not block-aligned.
	ErrCBCLength = errors.New("securemsg: CBC input length must be a multiple of 8")

	// ErrPadding reports malformed padding.
	ErrPadding = errors.New("securemsg: invalid padding")

	// ErrScopeMismatch reports an envelope scope mismatch.
	ErrScopeMismatch = errors.New("securemsg: scope mismatch")

	// ErrIntegrity reports a failed MAC check.
	ErrIntegrity = errors.New("securemsg: integrity check failed")

	// ErrMessageTooShort reports a truncated envelope.
	ErrMessageTooShort = errors.New("securemsg: message too short")

	// ErrPayloadTooLong reports a payload that does not fit the envelope.
	ErrPayloadTooLong = errors.New("securemsg: payload too long")
)

func wrapError(base error, format string, args ...any) error {
	return fmt.Errorf("%w (%s)", base, fmt.Sprintf(format, args...))
}

// EncryptBlock applies the 16-round TEA variant to one block. When decrypt is
// true the inverse permutation is applied.
func EncryptBlock(key Key, block Block, decrypt bool) Block {
	k := [4]uint32{
		binary.LittleEndian.Uint32(key[0:4]),
		binary.LittleEndian.Uint32(key[4:8]),
		binary.LittleEndian.Uint32(key[8:12]),
		binary.LittleEndian.Uint32(key[12:16]),
	}
	v0 := binary.LittleEndian.Uint32(block[0:4])
	v1 := binary.LittleEndian.Uint32(block[4:8])

	if decrypt {
		sum := delta
		sum *= rounds // wraps to delta*16 mod 2^32
		for i := 0; i < rounds; i++ {
			v1 -= round(v0) ^ (sum + k[(sum>>11)&3])
			sum -= delta
			v0 -= round(v1) ^ (sum + k[sum&3])
		}
	} else {
		var sum uint32
		for i := 0; i < rounds; i++ {
			v0 += round(v1) ^ (sum + k[sum&3])
			sum += delta
			v1 += round(v0) ^ (sum + k[(sum>>11)&3])
		}
	}

	var out Block
	binary.LittleEndian.PutUint32(out[0:4], v0)
	binary.LittleEndian.PutUint32(out[4:8], v1)
	return out
}

// round is the TEA-variant round function: ((x << 4) ^ (x >> 5)) + x.
func round(x uint32) uint32 {
	return ((x << 4) ^ (x >> 5)) + x
}
