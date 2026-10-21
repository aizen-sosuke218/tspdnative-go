package securemsg

import (
	"crypto/subtle"
	"encoding/binary"
)

// Seal encrypts and authenticates payload, then wraps it in a securemsg
// envelope for the given scope and 4-byte context.
func Seal(payload []byte, scope byte, context [4]byte, key Key) ([]byte, error) {
	ciphertext, err := Encrypt(key, payload)
	if err != nil {
		return nil, err
	}

	signatureLength := 8 + len(ciphertext)
	if signatureLength >= 4096 {
		return nil, ErrPayloadTooLong
	}

	message := make([]byte, minHeader+signatureLength)
	message[0] = minHeader
	copy(message[1:5], context[:])
	message[5] = scope
	binary.LittleEndian.PutUint16(message[6:8], uint16(signatureLength))

	mac := MAC(key[:], concat(ciphertext, context[:], []byte{scope}))
	copy(message[8:16], mac[:])
	copy(message[16:], ciphertext)
	return message, nil
}

// Unseal validates and decrypts a securemsg envelope.
//
// When shortcut is true, the function mirrors the sensor's "AB" path: it
// returns the message prefix (header, MAC and ciphertext) without checking
// the MAC or decrypting. Callers use this only to read the embedded context.
func Unseal(
	message []byte,
	scope byte,
	key Key,
	shortcut bool,
) (payload []byte, context [4]byte, err error) {
	if len(message) < minHeader {
		return nil, context, wrapError(
			ErrMessageTooShort, "%d < %d", len(message), minHeader,
		)
	}

	headerLength := int(message[0])
	copy(context[:], message[1:5])
	messageScope := message[5]
	signatureLength := int(binary.LittleEndian.Uint16(message[6:8]))

	if messageScope != scope {
		return nil, context, wrapError(
			ErrScopeMismatch, "%02x != %02x", messageScope, scope,
		)
	}
	if headerLength < minHeader {
		return nil, context, wrapError(
			ErrMessageTooShort, "header %d < %d", headerLength, minHeader,
		)
	}

	total := headerLength + signatureLength
	if len(message) < total {
		return nil, context, wrapError(
			ErrMessageTooShort, "%d < %d", len(message), total,
		)
	}
	if signatureLength < 8 {
		return nil, context, wrapError(
			ErrMessageTooShort, "signature %d < 8", signatureLength,
		)
	}

	if shortcut {
		payload = make([]byte, total)
		copy(payload, message[:total])
		return payload, context, nil
	}

	ciphertext := message[headerLength+8 : total]
	expectedMAC := MAC(key[:], concat(ciphertext, context[:], []byte{scope}))
	if subtle.ConstantTimeCompare(message[headerLength:headerLength+8], expectedMAC[:]) != 1 {
		return nil, context, ErrIntegrity
	}

	plaintext, err := Decrypt(key, ciphertext)
	if err != nil {
		return nil, context, err
	}
	return plaintext, context, nil
}
