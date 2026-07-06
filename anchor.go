package bramble

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

// Trust-anchor client-side crypto. The fleet anchor is one Ed25519 keypair
// held by the operator (its PRIVATE seed never leaves this client, never goes
// to a node). The anchor signs an endorsement over each node's Ed25519
// identity key; an anchored node pins only anchor-endorsed identities. These
// helpers mint the anchor key, sign endorsements, and encode the operator's
// backup, byte-compatible with the firmware's identity_endorsement_verify and
// the webapp's anchor.ts (verified by the KAT in anchor_test.go).

const (
	// endorsementContext is the 18-byte domain separator (no NUL) that opens
	// the signed endorsement message.
	endorsementContext = "bramble-endorse-v1"
	// anchorBackupPrefix carries the operator's SECRET anchor seed.
	anchorBackupPrefix = "bramble://anchor/v1?sk="
)

// PermanentNotAfter is the not_after sentinel for a cert that never expires
// (v1 always issues this).
const PermanentNotAfter uint64 = math.MaxUint64

// GenerateAnchorSeed returns a fresh random 32-byte anchor seed. The seed is
// the operator's root secret: back it up, keep it offline, never send it to a
// node.
func GenerateAnchorSeed() ([]byte, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("bramble: anchor seed rng: %w", err)
	}
	return seed, nil
}

// AnchorPublicKey derives the anchor Ed25519 public key from a 32-byte seed
// (RFC 8032, matching the firmware and webapp).
func AnchorPublicKey(seed []byte) (ed25519.PublicKey, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("bramble: anchor seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	return ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey), nil
}

// AnchorFingerprint is SHA256(anchorPub)[0:4] as 8 lowercase hex, matching the
// firmware's identity_anchor_fingerprint and getAnchorStatus.anchor_fingerprint.
func AnchorFingerprint(anchorPub []byte) string {
	sum := sha256.Sum256(anchorPub)
	return hex.EncodeToString(sum[:4])
}

// EndorsementMessage builds the 58-byte canonical signed message:
// "bramble-endorse-v1"(18) || nodeEd25519Pub(32) || notAfter(8, big-endian).
func EndorsementMessage(nodeEd25519Pub []byte, notAfter uint64) []byte {
	msg := make([]byte, 0, len(endorsementContext)+32+8)
	msg = append(msg, endorsementContext...)
	msg = append(msg, nodeEd25519Pub...)
	var na [8]byte
	binary.BigEndian.PutUint64(na[:], notAfter)
	return append(msg, na[:]...)
}

// SignEndorsement signs an endorsement cert for nodeEd25519Pub with the anchor
// seed. Returns the 64-byte Ed25519 signature over the canonical message.
func SignEndorsement(seed, nodeEd25519Pub []byte, notAfter uint64) ([]byte, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("bramble: anchor seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	if len(nodeEd25519Pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("bramble: node ed25519 pub must be %d bytes, got %d", ed25519.PublicKeySize, len(nodeEd25519Pub))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return ed25519.Sign(priv, EndorsementMessage(nodeEd25519Pub, notAfter)), nil
}

// NotAfterHex renders a not_after value as the 16-char big-endian hex string
// the setEndorsement RPC expects.
func NotAfterHex(notAfter uint64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], notAfter)
	return hex.EncodeToString(b[:])
}

// SignEndorsementHex is the hex-in/hex-out convenience the CLI uses: given the
// anchor seed and the node's ed25519 pubkey as hex, it returns the
// (notAfterHex, sigHex) pair to pass straight to Client.SetEndorsement.
func SignEndorsementHex(seedHex, nodeEd25519PubHex string, notAfter uint64) (notAfterHex, sigHex string, err error) {
	seed, err := decodeHexLen(seedHex, ed25519.SeedSize, "anchor seed")
	if err != nil {
		return "", "", err
	}
	nodePub, err := decodeHexLen(nodeEd25519PubHex, ed25519.PublicKeySize, "node ed25519 pub")
	if err != nil {
		return "", "", err
	}
	sig, err := SignEndorsement(seed, nodePub, notAfter)
	if err != nil {
		return "", "", err
	}
	return NotAfterHex(notAfter), hex.EncodeToString(sig), nil
}

// EncodeAnchorBackup encodes the operator's SECRET anchor seed as a backup
// URI. Guard it like a root key: anyone holding it can enroll fleet members.
func EncodeAnchorBackup(seed []byte) (string, error) {
	if len(seed) != ed25519.SeedSize {
		return "", fmt.Errorf("bramble: anchor seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	return anchorBackupPrefix + hex.EncodeToString(seed), nil
}

// ParseAnchorBackup extracts the anchor seed from a bramble://anchor/v1?sk=
// backup URI (or a bare 64-hex seed).
func ParseAnchorBackup(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, anchorBackupPrefix)
	return decodeHexLen(s, ed25519.SeedSize, "anchor seed")
}

func decodeHexLen(s string, wantBytes int, what string) ([]byte, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("bramble: %s: invalid hex: %w", what, err)
	}
	if len(b) != wantBytes {
		return nil, fmt.Errorf("bramble: %s must be %d bytes (%d hex chars), got %d", what, wantBytes, wantBytes*2, len(b))
	}
	return b, nil
}
