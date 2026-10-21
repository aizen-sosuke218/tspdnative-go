package securemsg

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/aizen-sosuke218/tspdnative-go/internal/testsupport"
)

type capturedFixture struct {
	Key0Hex      string `json:"key0_hex"`
	ChallengeHex string `json:"challenge_hex"`
	Expect       struct {
		PlainHex       string `json:"plain_hex"`
		ContextHex     string `json:"context_hex"`
		CtxHex         string `json:"ctx_hex"`
		SealScope      string `json:"seal_scope"`
		SealInputASCII string `json:"seal_input_ascii"`
		SealHex        string `json:"seal_hex"`
	} `json:"expect"`
}

func TestCapturedVectors(t *testing.T) {
	fixture := loadCapturedFixture(t)
	key := mustKey(t, fixture.Key0Hex)
	challenge := mustHex(t, fixture.ChallengeHex)

	if len(challenge) != 136 {
		t.Fatalf("challenge bytes = %d, want 136", len(challenge))
	}

	plain, context, err := Unseal(challenge, 0x01, key, false)
	if err != nil {
		t.Fatalf("Unseal(challenge): %v", err)
	}
	if got := hex.EncodeToString(plain); got != fixture.Expect.PlainHex {
		t.Fatalf("plain = %s, want %s", got, fixture.Expect.PlainHex)
	}
	if got := hex.EncodeToString(context[:]); got != fixture.Expect.CtxHex {
		t.Fatalf("context = %s, want %s", got, fixture.Expect.CtxHex)
	}

	inner, _, err := Unseal(plain, 0xAB, key, true)
	if err != nil {
		t.Fatalf("Unseal(inner, shortcut): %v", err)
	}
	if got := hex.EncodeToString(inner); got != fixture.Expect.ContextHex {
		t.Fatalf("inner context = %s, want %s", got, fixture.Expect.ContextHex)
	}

	sealScope := mustHex(t, fixture.Expect.SealScope)[0]
	seal, err := Seal([]byte(fixture.Expect.SealInputASCII), sealScope, context, key)
	if err != nil {
		t.Fatalf("Seal(sample): %v", err)
	}
	if got := hex.EncodeToString(seal); got != fixture.Expect.SealHex {
		t.Fatalf("seal = %s, want %s", got, fixture.Expect.SealHex)
	}
}

func TestUnsealRejectsTampering(t *testing.T) {
	fixture := loadCapturedFixture(t)
	key := mustKey(t, fixture.Key0Hex)
	message := mustHex(t, fixture.Expect.SealHex)
	message[len(message)-1] ^= 0xFF

	if _, _, err := Unseal(message, 0x04, key, false); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("Unseal(tampered) error = %v, want ErrIntegrity", err)
	}
}

func TestUnsealRejectsWrongScope(t *testing.T) {
	fixture := loadCapturedFixture(t)
	key := mustKey(t, fixture.Key0Hex)
	message := mustHex(t, fixture.ChallengeHex)

	if _, _, err := Unseal(message, 0x04, key, false); !errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("Unseal(wrong scope) error = %v, want ErrScopeMismatch", err)
	}
}

func TestBlockRoundTrip(t *testing.T) {
	key := mustKey(t, "b02bed0d2dac17cff554364b6248d577")
	block := Block{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}

	ciphertext := EncryptBlock(key, block, false)
	if ciphertext == block {
		t.Fatal("EncryptBlock returned the plaintext block")
	}
	if plaintext := EncryptBlock(key, ciphertext, true); plaintext != block {
		t.Fatalf("decrypt = %x, want %x", plaintext, block)
	}
}

func TestCBCRoundTrip(t *testing.T) {
	key := mustKey(t, "85d0151cd22f3ee0bad0a86b7e29f9bf")

	for _, size := range []int{0, 1, 7, 8, 9, 24, 159} {
		plaintext := make([]byte, size)
		for i := range plaintext {
			plaintext[i] = byte(i*37 + 11)
		}

		ciphertext, err := Encrypt(key, plaintext)
		if err != nil {
			t.Fatalf("Encrypt(%d bytes): %v", size, err)
		}
		if len(ciphertext)%BlockSize != 0 {
			t.Fatalf("ciphertext length = %d, not block aligned", len(ciphertext))
		}

		decrypted, err := Decrypt(key, ciphertext)
		if err != nil {
			t.Fatalf("Decrypt(%d bytes): %v", size, err)
		}
		if !bytes.Equal(decrypted, plaintext) {
			t.Fatalf("round trip mismatch for %d bytes", size)
		}
	}
}

func TestCBCRejectsUnalignedInput(t *testing.T) {
	key := mustKey(t, "85d0151cd22f3ee0bad0a86b7e29f9bf")
	if _, err := CBCEncrypt(key, make([]byte, 7)); !errors.Is(err, ErrCBCLength) {
		t.Fatalf("CBCEncrypt error = %v, want ErrCBCLength", err)
	}
}

func TestPadRoundTrip(t *testing.T) {
	for _, size := range []int{0, 1, 7, 8, 15} {
		data := bytes.Repeat([]byte{0x41}, size)
		padded, err := Pad(data, BlockSize, 0xFF)
		if err != nil {
			t.Fatalf("Pad(%d): %v", size, err)
		}
		if len(padded)%BlockSize != 0 {
			t.Fatalf("padded length = %d, not block aligned", len(padded))
		}
		unpadded, err := Unpad(padded)
		if err != nil {
			t.Fatalf("Unpad(%d): %v", size, err)
		}
		if !bytes.Equal(unpadded, data) {
			t.Fatalf("padding round trip mismatch for %d bytes", size)
		}
	}
}

func BenchmarkSeal(b *testing.B) {
	key := Key{
		0xb0, 0x2b, 0xed, 0x0d, 0x2d, 0xac, 0x17, 0xcf,
		0xf5, 0x54, 0x36, 0x4b, 0x62, 0x48, 0xd5, 0x77,
	}
	payload := make([]byte, 159)
	context := [4]byte{0xcb, 0x8a, 0xc8, 0x8f}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Seal(payload, 0x04, context, key); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncryptBlock(b *testing.B) {
	key := Key{0xb0, 0x2b, 0xed, 0x0d, 0x2d, 0xac, 0x17, 0xcf}
	block := Block{1, 2, 3, 4, 5, 6, 7, 8}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = EncryptBlock(key, block, false)
	}
}

func loadCapturedFixture(t *testing.T) capturedFixture {
	t.Helper()
	data, err := os.ReadFile(testsupport.FixturePath("captured_challenge.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture capturedFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return fixture
}

func mustKey(t *testing.T, value string) Key {
	t.Helper()
	key, err := NewKey(mustHex(t, value))
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	return key
}

func mustHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode hex %q: %v", value, err)
	}
	return decoded
}
