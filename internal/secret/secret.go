// Package secret seals values that must leave the server but stay unreadable.
//
// The cipher is AES-256-GCM. AES-256 is the AES block cipher keyed with 32 bytes;
// GCM is an AEAD mode, so a single pass both encrypts and authenticates: counter
// mode turns the block cipher into a keystream for confidentiality, and GHASH
// derives a 16-byte tag over the ciphertext and the AAD for integrity. Open
// recomputes that tag, so an edited byte is refused rather than decrypted into
// plausible garbage.
//
// GCM's one hard rule is that a nonce is never reused with a given key: two
// messages sharing one leak their XOR and let an attacker recover the hash key
// and forge tags (Joux's "forbidden attack"). Seal therefore draws a fresh
// 12-byte nonce per value and stores it in the clear beside the ciphertext,
// where it is safe — a nonce must be unique, not secret.
//
// Specifications: AES is FIPS 197; GCM, the nonce uniqueness requirement and the
// 2^32-message ceiling for random 96-bit nonces are NIST SP 800-38D §8; the AEAD
// interface shape is RFC 5116.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// keyLen is fixed at AES-256. A shorter key is a configuration mistake, not a
// weaker mode to fall back to.
const keyLen = 32

// ErrOpen is the only error Open reports: which of the version, nonce, tag or
// AAD was wrong is something no caller can act on and no attacker should learn.
var ErrOpen = errors.New("secret: cannot open")

// Box seals and opens values with one master key. version is written into every
// output so a future key rotation can tell which key sealed a stored value.
type Box struct {
	aead    cipher.AEAD
	version byte
}

// NewBox takes the master key as standard base64 — 32 bytes once decoded, which
// is what `openssl rand -base64 32` produces.
func NewBox(base64Key string, version int) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("secret: key is not valid base64: %w", err)
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("secret: key must decode to %d bytes, got %d", keyLen, len(key))
	}
	if version < 1 || version > 255 {
		return nil, fmt.Errorf("secret: key version must be 1..255, got %d", version)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new gcm: %w", err)
	}

	return &Box{aead: aead, version: byte(version)}, nil
}

// Seal returns base64url(version ‖ nonce ‖ ciphertext‖tag). aad is bound into
// the tag without being stored, so a value sealed in one context cannot be
// opened in another under the same key — pass a constant naming the context.
func (b *Box) Seal(plaintext, aad []byte) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secret: nonce: %w", err)
	}

	// The version and nonce prefix is not secret; GCM only requires that a nonce
	// is never reused with the same key.
	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+b.aead.Overhead())
	out = append(out, b.version)
	out = append(out, nonce...)
	out = b.aead.Seal(out, nonce, plaintext, aad)

	return base64.RawURLEncoding.EncodeToString(out), nil
}

// Open reverses Seal. Tampering fails the GCM tag check, so a returned
// plaintext is exactly what was sealed under this key.
func (b *Box) Open(token string, aad []byte) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrOpen
	}

	nonceSize := b.aead.NonceSize()
	if len(raw) < 1+nonceSize+b.aead.Overhead() {
		return nil, ErrOpen
	}
	if raw[0] != b.version {
		return nil, ErrOpen
	}

	plaintext, err := b.aead.Open(nil, raw[1:1+nonceSize], raw[1+nonceSize:], aad)
	if err != nil {
		return nil, ErrOpen
	}
	return plaintext, nil
}
