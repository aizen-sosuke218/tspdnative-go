package securemsg

// jzMask XOR-masks the second key half in the chained hash.
var jzMask = [8]byte{0xB7, 0xD9, 0x20, 0x0D, 0x3D, 0xC6, 0x6C, 0x49}

// jzInitialState is the ASCII seed "poiuytre".
var jzInitialState = [8]byte{'p', 'o', 'i', 'u', 'y', 't', 'r', 'e'}

// JZ computes the 8-byte chained hash used by the MAC. Input is padded with
// 'y' bytes and each block is folded into the running state.
func JZ(data []byte) Block {
	count := BlockSize - (len(data) % BlockSize) - 1
	padded := make([]byte, len(data)+count+1)
	copy(padded, data)
	for i := 0; i < count; i++ {
		padded[len(data)+i] = 0x79
	}
	padded[len(padded)-1] = byte(count)

	state := Block(jzInitialState)
	var key Key
	for offset := 0; offset < len(padded); offset += BlockSize {
		copy(key[0:8], padded[offset:offset+BlockSize])
		for i := 0; i < BlockSize; i++ {
			key[8+i] = padded[offset+i] ^ jzMask[i]
		}

		encrypted := EncryptBlock(key, state, false)
		for i := 0; i < BlockSize; i++ {
			state[i] ^= encrypted[i]
		}
	}
	return state
}

// MAC computes the 8-byte message authentication code over data. Keys longer
// than 16 bytes are first folded through JZ.
func MAC(keyMaterial []byte, data []byte) Block {
	var material Key
	if len(keyMaterial) <= KeySize {
		copy(material[:], keyMaterial)
	} else {
		digest := JZ(keyMaterial)
		copy(material[:], digest[:])
	}

	var innerKey, outerKey Key
	for i := 0; i < KeySize; i++ {
		innerKey[i] = material[i] ^ 0x36
		outerKey[i] = material[i] ^ 0x5C
	}

	inner := JZ(concat(innerKey[:], data))
	return JZ(concat(outerKey[:], inner[:]))
}

func concat(parts ...[]byte) []byte {
	size := 0
	for _, part := range parts {
		size += len(part)
	}
	out := make([]byte, 0, size)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}
