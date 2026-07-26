package postgres

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"testing"
)

func TestContactCodecUsesCurrentKeyAndDecryptsHistoricalVersion(t *testing.T) {
	oldCodec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKeyVersion:  "v1",
		FingerprintKeyVersion: "v1",
		EncryptionKeys:        map[string]string{"v1": "test-encryption-key-v1"},
		FingerprintKeys:       map[string]string{"v1": "test-fingerprint-key-v1"},
	})
	if err != nil {
		t.Fatalf("new old codec: %v", err)
	}
	encoded, err := oldCodec.encode("secret@example.com", "record-1", contactFieldMethodValue)
	if err != nil {
		t.Fatalf("encode old version: %v", err)
	}

	codec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKeyVersion:  "v2",
		FingerprintKeyVersion: "v2",
		EncryptionKeys: map[string]string{
			"v1": "test-encryption-key-v1",
			"v2": "test-encryption-key-v2",
		},
		FingerprintKeys: map[string]string{
			"v1": "test-fingerprint-key-v1",
			"v2": "test-fingerprint-key-v2",
		},
	})
	if err != nil {
		t.Fatalf("new keyring codec: %v", err)
	}
	value, err := codec.decode(encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion, encoded.CipherFormat, "record-1", contactFieldMethodValue)
	if err != nil {
		t.Fatalf("decode historical version: %v", err)
	}
	if value != "secret@example.com" {
		t.Fatalf("unexpected decoded value %q", value)
	}

	current, err := codec.encode("next@example.com", "record-2", contactFieldMethodValue)
	if err != nil {
		t.Fatalf("encode current version: %v", err)
	}
	if current.EncryptionKeyVersion != "v2" || current.FingerprintKeyVersion != "v2" || current.CipherFormat != contactCipherFormatAADV1 {
		t.Fatalf("new writes must use current versions and AAD: %+v", current)
	}
}

func TestContactCodecAADRejectsRecordOrFieldSwap(t *testing.T) {
	codec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKeyVersion:  "v1",
		FingerprintKeyVersion: "v1",
		EncryptionKeys:        map[string]string{"v1": "test-encryption-key-v1"},
		FingerprintKeys:       map[string]string{"v1": "test-fingerprint-key-v1"},
	})
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}
	encoded, err := codec.encode("sensitive", "record-1", contactFieldMethodValue)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := codec.decode(encoded.Ciphertext, encoded.Nonce, "v1", contactCipherFormatAADV1, "record-2", contactFieldMethodValue); err == nil {
		t.Fatal("record ID swap must fail authentication")
	}
	if _, err := codec.decode(encoded.Ciphertext, encoded.Nonce, "v1", contactCipherFormatAADV1, "record-1", contactFieldModelAPIKey); err == nil {
		t.Fatal("field type swap must fail authentication")
	}
}

func TestContactCodecReadsExplicitLegacyFormat(t *testing.T) {
	const key = "legacy-encryption-key"
	derived := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("new GCM: %v", err)
	}
	nonce := make([]byte, aead.NonceSize())
	ciphertext := aead.Seal(nil, nonce, []byte("legacy-value"), nil)

	codec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKeyVersion:  "legacy-v1",
		FingerprintKeyVersion: "legacy-v1",
		EncryptionKeys:        map[string]string{"legacy-v1": key},
		FingerprintKeys:       map[string]string{"legacy-v1": "legacy-fingerprint-key"},
	})
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}
	value, err := codec.decode(ciphertext, nonce, "legacy-v1", contactCipherFormatLegacy, "record-1", contactFieldMethodValue)
	if err != nil {
		t.Fatalf("decode legacy: %v", err)
	}
	if value != "legacy-value" {
		t.Fatalf("unexpected legacy value %q", value)
	}
}

func TestContactCodecUnknownVersionIsObservable(t *testing.T) {
	codec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKeyVersion:  "v2",
		FingerprintKeyVersion: "v2",
		EncryptionKeys:        map[string]string{"v2": "test-encryption-key-v2"},
		FingerprintKeys:       map[string]string{"v2": "test-fingerprint-key-v2"},
	})
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}
	_, err = codec.decode([]byte("ciphertext"), make([]byte, 12), "retired-v1", contactCipherFormatAADV1, "record-1", contactFieldMethodValue)
	if !errors.Is(err, errUnknownContactEncryptionKey) {
		t.Fatalf("expected unknown key error, got %v", err)
	}
	stats := codec.stats()
	if stats.UnknownKeyTotal != 1 || stats.DecryptFailureTotal != 1 || stats.DecryptSuccessTotal != 0 {
		t.Fatalf("unexpected codec stats: %+v", stats)
	}
}
