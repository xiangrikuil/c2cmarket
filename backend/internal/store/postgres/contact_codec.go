package postgres

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
)

const (
	contactCipherFormatLegacy = "legacy_no_aad_v1"
	contactCipherFormatAADV1  = "aad_v1"

	contactFieldMethodValue   = "contact_method_value"
	contactFieldModelAPIKey   = "model_audit_api_key"
	contactFieldProbeAPIKey   = "api_service_probe_credential"
	contactFieldQuotaAPIKey   = "api_quota_api_key"
	contactFieldQuotaPassword = "api_quota_password"
	contactFieldOrderAPIKey   = "api_order_api_key"
	contactFieldOrderPassword = "api_order_password"
)

var (
	errUnknownContactEncryptionKey = errors.New("contact encryption key version is unavailable")
	errUnknownContactCipherFormat  = errors.New("contact cipher format is unsupported")
)

type ContactCryptoConfig struct {
	EncryptionKey         string
	FingerprintKey        string
	EncryptionKeyVersion  string
	FingerprintKeyVersion string
	EncryptionKeys        map[string]string
	FingerprintKeys       map[string]string
}

type contactCodec struct {
	aeadByVersion         map[string]cipher.AEAD
	fingerprintByVersion  map[string][]byte
	encryptionKeyVersion  string
	fingerprintKeyVersion string
	decryptSuccessTotal   atomic.Uint64
	decryptFailureTotal   atomic.Uint64
	unknownKeyTotal       atomic.Uint64
}

type encodedContactValue struct {
	Ciphertext            []byte
	Nonce                 []byte
	Fingerprint           string
	EncryptionKeyVersion  string
	FingerprintKeyVersion string
	CipherFormat          string
}

type ContactCryptoStats struct {
	DecryptSuccessTotal uint64
	DecryptFailureTotal uint64
	UnknownKeyTotal     uint64
}

func newContactCodec(config ContactCryptoConfig) (*contactCodec, error) {
	encryptionVersion := strings.TrimSpace(config.EncryptionKeyVersion)
	if encryptionVersion == "" {
		encryptionVersion = "local-dev-v1"
	}
	fingerprintVersion := strings.TrimSpace(config.FingerprintKeyVersion)
	if fingerprintVersion == "" {
		fingerprintVersion = encryptionVersion
	}

	encryptionKeys := cloneContactKeyring(config.EncryptionKeys)
	if strings.TrimSpace(config.EncryptionKey) != "" {
		if _, exists := encryptionKeys[encryptionVersion]; !exists {
			encryptionKeys[encryptionVersion] = config.EncryptionKey
		}
	}
	fingerprintKeys := cloneContactKeyring(config.FingerprintKeys)
	if strings.TrimSpace(config.FingerprintKey) != "" {
		if _, exists := fingerprintKeys[fingerprintVersion]; !exists {
			fingerprintKeys[fingerprintVersion] = config.FingerprintKey
		}
	}
	if len(encryptionKeys) == 0 {
		return nil, fmt.Errorf("contact encryption keyring is empty")
	}
	if len(fingerprintKeys) == 0 {
		return nil, fmt.Errorf("contact fingerprint keyring is empty")
	}

	codec := &contactCodec{
		aeadByVersion:         make(map[string]cipher.AEAD, len(encryptionKeys)),
		fingerprintByVersion:  make(map[string][]byte, len(fingerprintKeys)),
		encryptionKeyVersion:  encryptionVersion,
		fingerprintKeyVersion: fingerprintVersion,
	}
	for version, key := range encryptionKeys {
		version = strings.TrimSpace(version)
		if version == "" || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("contact encryption keyring contains an empty version or key")
		}
		derived := sha256.Sum256([]byte(key))
		block, err := aes.NewCipher(derived[:])
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		codec.aeadByVersion[version] = aead
	}
	for version, key := range fingerprintKeys {
		version = strings.TrimSpace(version)
		if version == "" || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("contact fingerprint keyring contains an empty version or key")
		}
		derived := sha256.Sum256([]byte(key))
		codec.fingerprintByVersion[version] = append([]byte(nil), derived[:]...)
	}
	if _, ok := codec.aeadByVersion[codec.encryptionKeyVersion]; !ok {
		return nil, fmt.Errorf("%w: current version", errUnknownContactEncryptionKey)
	}
	if _, ok := codec.fingerprintByVersion[codec.fingerprintKeyVersion]; !ok {
		return nil, fmt.Errorf("contact fingerprint key version is unavailable")
	}
	return codec, nil
}

func cloneContactKeyring(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for version, key := range source {
		cloned[version] = key
	}
	return cloned
}

func (c *contactCodec) encode(value, recordID, fieldType string) (encodedContactValue, error) {
	if c == nil {
		return encodedContactValue{}, fmt.Errorf("contact codec is not configured")
	}
	aead, ok := c.aeadByVersion[c.encryptionKeyVersion]
	if !ok {
		return encodedContactValue{}, errUnknownContactEncryptionKey
	}
	aad, err := contactAAD(recordID, fieldType, c.encryptionKeyVersion)
	if err != nil {
		return encodedContactValue{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return encodedContactValue{}, err
	}
	ciphertext := aead.Seal(nil, nonce, []byte(value), aad)
	return encodedContactValue{
		Ciphertext:            ciphertext,
		Nonce:                 nonce,
		Fingerprint:           c.fingerprint(value),
		EncryptionKeyVersion:  c.encryptionKeyVersion,
		FingerprintKeyVersion: c.fingerprintKeyVersion,
		CipherFormat:          contactCipherFormatAADV1,
	}, nil
}

func (c *contactCodec) decode(ciphertext, nonce []byte, keyVersion, cipherFormat, recordID, fieldType string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("contact codec is not configured")
	}
	aead, ok := c.aeadByVersion[strings.TrimSpace(keyVersion)]
	if !ok {
		c.unknownKeyTotal.Add(1)
		c.decryptFailureTotal.Add(1)
		return "", errUnknownContactEncryptionKey
	}
	var aad []byte
	switch cipherFormat {
	case contactCipherFormatLegacy:
	case contactCipherFormatAADV1:
		var err error
		aad, err = contactAAD(recordID, fieldType, keyVersion)
		if err != nil {
			c.decryptFailureTotal.Add(1)
			return "", err
		}
	default:
		c.decryptFailureTotal.Add(1)
		return "", errUnknownContactCipherFormat
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		c.decryptFailureTotal.Add(1)
		return "", err
	}
	c.decryptSuccessTotal.Add(1)
	return string(plaintext), nil
}

func (c *contactCodec) fingerprint(value string) string {
	key := c.fingerprintByVersion[c.fingerprintKeyVersion]
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *contactCodec) stats() ContactCryptoStats {
	if c == nil {
		return ContactCryptoStats{}
	}
	return ContactCryptoStats{
		DecryptSuccessTotal: c.decryptSuccessTotal.Load(),
		DecryptFailureTotal: c.decryptFailureTotal.Load(),
		UnknownKeyTotal:     c.unknownKeyTotal.Load(),
	}
}

func contactAAD(recordID, fieldType, keyVersion string) ([]byte, error) {
	values := []string{
		strings.TrimSpace(recordID),
		strings.TrimSpace(fieldType),
		strings.TrimSpace(keyVersion),
	}
	for _, value := range values {
		if value == "" {
			return nil, fmt.Errorf("contact cipher context is incomplete")
		}
	}
	var buffer bytes.Buffer
	buffer.WriteString("c2c-market-contact-aad-v1")
	for _, value := range values {
		if err := binary.Write(&buffer, binary.BigEndian, uint32(len(value))); err != nil {
			return nil, err
		}
		buffer.WriteString(value)
	}
	return buffer.Bytes(), nil
}
