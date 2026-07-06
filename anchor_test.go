package bramble

import (
	"encoding/hex"
	"testing"
)

// Fixed KAT inputs shared with the firmware (test/test_identity_endorsement.c)
// and the webapp (anchor.test.ts): the same anchor seed + node pubkey +
// permanent not_after must produce the SAME 64-byte signature on all three
// implementations, or a cert this SDK signs would be rejected by the device.
const (
	katAnchorSeedHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	katNodePubHex    = "404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f"
	// Expected endorsement signature over katNodePub with not_after=PERMANENT,
	// signed by the KAT anchor key. Pinned by the firmware KAT.
	katSigPermanent = "016e65eae269ec3b1465252b33d526c1d9157d39dfff5f9009c71bb6118a85b3" +
		"7a36afd28bc2f36869f2bba54b601c79cc81213dcc2c41b76ec32ab74740b903"
	// The anchor public key the KAT seed expands to (RFC 8032). Must match the
	// firmware's crypto_ed25519_keypair_from_seed and the webapp's @noble.
	katAnchorPubHex = "03a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}
	return b
}

// The load-bearing cross-implementation parity test: signing the firmware's
// KAT vector must reproduce the firmware's expected signature byte-for-byte.
func TestEndorsementParityKAT(t *testing.T) {
	seed := mustHex(t, katAnchorSeedHex)
	nodePub := mustHex(t, katNodePubHex)

	sig, err := SignEndorsement(seed, nodePub, PermanentNotAfter)
	if err != nil {
		t.Fatalf("SignEndorsement: %v", err)
	}
	got := hex.EncodeToString(sig)
	if got != katSigPermanent {
		t.Fatalf("endorsement signature mismatch with firmware KAT\n got: %s\nwant: %s", got, katSigPermanent)
	}

	// Anchor pubkey derivation must match the firmware/webapp (guarded
	// transitively by the signature above, asserted here explicitly so a
	// future edit to the seed expansion is caught directly).
	pub, err := AnchorPublicKey(seed)
	if err != nil {
		t.Fatalf("AnchorPublicKey: %v", err)
	}
	if hex.EncodeToString(pub) != katAnchorPubHex {
		t.Fatalf("anchor pubkey mismatch\n got: %s\nwant: %s", hex.EncodeToString(pub), katAnchorPubHex)
	}
}

func TestEndorsementMessageLayout(t *testing.T) {
	nodePub := mustHex(t, katNodePubHex)
	// A distinct, non-sentinel not_after exercises the big-endian encoding.
	msg := EndorsementMessage(nodePub, 0x0102030405060708)
	if len(msg) != 58 {
		t.Fatalf("message length = %d, want 58", len(msg))
	}
	if string(msg[:18]) != "bramble-endorse-v1" {
		t.Fatalf("context prefix = %q", msg[:18])
	}
	if hex.EncodeToString(msg[18:50]) != katNodePubHex {
		t.Fatalf("node pub not at offset 18")
	}
	if hex.EncodeToString(msg[50:58]) != "0102030405060708" {
		t.Fatalf("not_after not big-endian at offset 50: %s", hex.EncodeToString(msg[50:58]))
	}
}

func TestNotAfterHexPermanent(t *testing.T) {
	if NotAfterHex(PermanentNotAfter) != "ffffffffffffffff" {
		t.Fatalf("permanent not_after hex = %s", NotAfterHex(PermanentNotAfter))
	}
}

func TestAnchorFingerprintKAT(t *testing.T) {
	pub := mustHex(t, katAnchorPubHex)
	// SHA256(anchorPub)[0:4]; independently recomputed value.
	if got := AnchorFingerprint(pub); got != "56475aa7" {
		t.Fatalf("anchor fingerprint = %s, want 56475aa7", got)
	}
}

func TestAnchorBackupRoundTrip(t *testing.T) {
	seed := mustHex(t, katAnchorSeedHex)
	uri, err := EncodeAnchorBackup(seed)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if uri != "bramble://anchor/v1?sk="+katAnchorSeedHex {
		t.Fatalf("backup uri = %s", uri)
	}
	got, err := ParseAnchorBackup(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if hex.EncodeToString(got) != katAnchorSeedHex {
		t.Fatalf("round-trip seed = %s", hex.EncodeToString(got))
	}
	// Bare hex (no prefix) is also accepted; junk is rejected.
	if _, err := ParseAnchorBackup(katAnchorSeedHex); err != nil {
		t.Fatalf("bare hex should parse: %v", err)
	}
	if _, err := ParseAnchorBackup("bramble://anchor/v1?sk=zzzz"); err == nil {
		t.Fatalf("expected error on non-hex seed")
	}
}
