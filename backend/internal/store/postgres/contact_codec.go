package postgres

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

type ContactCryptoConfig struct {
	EncryptionKey         string
	FingerprintKey        string
	EncryptionKeyVersion  string
	FingerprintKeyVersion string
}

type contactCodec struct {
	aead                  cipher.AEAD
	fingerprintKey        []byte
	encryptionKeyVersion  string
	fingerprintKeyVersion string
}

type encodedContactValue struct {
	Ciphertext            []byte
	Nonce                 []byte
	Fingerprint           string
	EncryptionKeyVersion  string
	FingerprintKeyVersion string
}

func newContactCodec(config ContactCryptoConfig) (*contactCodec, error) {
	encryptionKey := sha256.Sum256([]byte(config.EncryptionKey))
	block, err := aes.NewCipher(encryptionKey[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	fingerprintKey := sha256.Sum256([]byte(config.FingerprintKey))
	codec := &contactCodec{
		aead:                  aead,
		fingerprintKey:        fingerprintKey[:],
		encryptionKeyVersion:  config.EncryptionKeyVersion,
		fingerprintKeyVersion: config.FingerprintKeyVersion,
	}
	if codec.encryptionKeyVersion == "" {
		codec.encryptionKeyVersion = "local-dev-v1"
	}
	if codec.fingerprintKeyVersion == "" {
		codec.fingerprintKeyVersion = codec.encryptionKeyVersion
	}
	return codec, nil
}

func (c *contactCodec) encode(value string) (encodedContactValue, error) {
	if c == nil || c.aead == nil {
		return encodedContactValue{}, fmt.Errorf("contact codec is not configured")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return encodedContactValue{}, err
	}
	ciphertext := c.aead.Seal(nil, nonce, []byte(value), nil)
	return encodedContactValue{
		Ciphertext:            ciphertext,
		Nonce:                 nonce,
		Fingerprint:           c.fingerprint(value),
		EncryptionKeyVersion:  c.encryptionKeyVersion,
		FingerprintKeyVersion: c.fingerprintKeyVersion,
	}, nil
}

func (c *contactCodec) decode(ciphertext, nonce []byte) (string, error) {
	if c == nil || c.aead == nil {
		return "", fmt.Errorf("contact codec is not configured")
	}
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (c *contactCodec) fingerprint(value string) string {
	mac := hmac.New(sha256.New, c.fingerprintKey)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
