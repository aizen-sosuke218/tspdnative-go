// Package securemsg is a Go port of the F5 TSPD "securemsg" module.
//
// It implements the TEA-variant block cipher, CBC mode, the chained hash, the
// MAC and the seal/unseal message envelope used by the TSPD sensor build.
//
// The wire format of a sealed message is:
//
//	[1B header=8][4B context][1B scope][2B signature length LE][8B MAC][cipher]
//	MAC    = MAC(key, cipher || context || scope)
//	cipher = Encrypt(key, payload)   // CBC with 0xFF padding
//
// This is protocol-specific cryptography, not a general-purpose replacement
// for AES or HMAC.
package securemsg

const (
	// KeySize is the block cipher key size in bytes.
	KeySize = 16

	// BlockSize is the block cipher block size in bytes.
	BlockSize = 8

	rounds = 16

	// delta is the TEA round constant.
	delta uint32 = 0x9E3779B9

	// minHeader is the fixed envelope header size.
	minHeader = 8
)

// Key is a 16-byte securemsg key.
type Key [KeySize]byte

// Block is one 8-byte cipher block.
type Block [BlockSize]byte

// NewKey validates and copies raw key material.
func NewKey(material []byte) (Key, error) {
	var key Key
	if len(material) != KeySize {
		return key, wrapError(ErrKeySize, "got %d bytes", len(material))
	}
	copy(key[:], material)
	return key, nil
}
