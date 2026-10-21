package securemsg

// CBCDecrypt decrypts block-aligned ciphertext using CBC mode with a zero IV.
func CBCDecrypt(key Key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext)%BlockSize != 0 {
		return nil, wrapError(ErrCBCLength, "got %d bytes", len(ciphertext))
	}

	plaintext := make([]byte, len(ciphertext))
	var previous Block
	for offset := 0; offset < len(ciphertext); offset += BlockSize {
		var current Block
		copy(current[:], ciphertext[offset:offset+BlockSize])

		decrypted := EncryptBlock(key, current, true)
		for i := 0; i < BlockSize; i++ {
			plaintext[offset+i] = decrypted[i] ^ previous[i]
		}
		previous = current
	}
	return plaintext, nil
}

// CBCEncrypt encrypts block-aligned plaintext using CBC mode with a zero IV.
func CBCEncrypt(key Key, plaintext []byte) ([]byte, error) {
	if len(plaintext)%BlockSize != 0 {
		return nil, wrapError(ErrCBCLength, "got %d bytes", len(plaintext))
	}

	ciphertext := make([]byte, len(plaintext))
	var previous Block
	for offset := 0; offset < len(plaintext); offset += BlockSize {
		var mixed Block
		for i := 0; i < BlockSize; i++ {
			mixed[i] = plaintext[offset+i] ^ previous[i]
		}
		encrypted := EncryptBlock(key, mixed, false)
		copy(ciphertext[offset:offset+BlockSize], encrypted[:])
		previous = encrypted
	}
	return ciphertext, nil
}

// Pad appends count bytes followed by the count itself. The result is always a
// multiple of blockSize.
func Pad(data []byte, blockSize int, padByte byte) ([]byte, error) {
	if blockSize <= 0 {
		return nil, wrapError(ErrBlockSize, "got %d", blockSize)
	}
	count := blockSize - (len(data) % blockSize) - 1
	padded := make([]byte, len(data)+count+1)
	copy(padded, data)
	for i := 0; i < count; i++ {
		padded[len(data)+i] = padByte
	}
	padded[len(padded)-1] = byte(count)
	return padded, nil
}

// Unpad strips the padding appended by Pad.
func Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrPadding
	}
	count := int(data[len(data)-1])
	if count >= len(data) {
		return nil, wrapError(ErrPadding, "count %d for length %d", count, len(data))
	}
	plaintext := make([]byte, len(data)-count-1)
	copy(plaintext, data[:len(data)-count-1])
	return plaintext, nil
}

// Encrypt pads plaintext with 0xFF bytes and applies CBC encryption.
func Encrypt(key Key, plaintext []byte) ([]byte, error) {
	padded, err := Pad(plaintext, BlockSize, 0xFF)
	if err != nil {
		return nil, err
	}
	return CBCEncrypt(key, padded)
}

// Decrypt applies CBC decryption and strips 0xFF padding.
func Decrypt(key Key, ciphertext []byte) ([]byte, error) {
	plaintext, err := CBCDecrypt(key, ciphertext)
	if err != nil {
		return nil, err
	}
	return Unpad(plaintext)
}
