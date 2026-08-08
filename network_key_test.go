package bramble

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// A fixed key and its fingerprint, so the fingerprint stays pinned to a known
// answer rather than to whatever the implementation happens to produce.
// SHA256 of the 32 zero bytes is
// 66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925, so the
// first four bytes render as 66687aad.
const (
	zeroKeyHex      = "0000000000000000000000000000000000000000000000000000000000000000"
	zeroKeyFpHex    = "66687aad"
	sampleKeyHex    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sampleShareURI  = "bramble://net/v1?k=" + sampleKeyHex
	shortKeyHex     = "0123456789abcdef"
	nonHexSampleKey = "zzzz456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func TestNetworkKeyFingerprint_KnownAnswer(t *testing.T) {
	key := make([]byte, NetworkKeySize)
	if got := NetworkKeyFingerprint(key); got != zeroKeyFpHex {
		t.Fatalf("fingerprint: got %q, want %q", got, zeroKeyFpHex)
	}
}

func TestNetworkKeyFingerprint_MatchesAcrossEqualKeys(t *testing.T) {
	a, err := hex.DecodeString(sampleKeyHex)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	b, err := hex.DecodeString(strings.ToUpper(sampleKeyHex))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if NetworkKeyFingerprint(a) != NetworkKeyFingerprint(b) {
		t.Fatal("same key decoded from different hex casing produced different fingerprints")
	}
}

func TestEncodeNetworkKeyShare_RoundTrip(t *testing.T) {
	key, err := hex.DecodeString(sampleKeyHex)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	uri, err := EncodeNetworkKeyShare(key)
	if err != nil {
		t.Fatalf("EncodeNetworkKeyShare: %v", err)
	}
	if uri != sampleShareURI {
		t.Fatalf("uri: got %q, want %q", uri, sampleShareURI)
	}
	back, err := ParseNetworkKeyShare(uri)
	if err != nil {
		t.Fatalf("ParseNetworkKeyShare: %v", err)
	}
	if !bytes.Equal(back, key) {
		t.Fatalf("round trip: got %x, want %x", back, key)
	}
}

func TestEncodeNetworkKeyShare_RejectsWrongLength(t *testing.T) {
	if _, err := EncodeNetworkKeyShare([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected an error for a 3-byte key")
	}
}

func TestParseNetworkKeyShare(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"share uri", sampleShareURI, false},
		{"bare hex", sampleKeyHex, false},
		{"uppercase bare hex", strings.ToUpper(sampleKeyHex), false},
		{"surrounding whitespace", "  " + sampleShareURI + "\n", false},
		{"uri missing k", "bramble://net/v1?x=" + sampleKeyHex, true},
		{"uri with short key", "bramble://net/v1?k=" + shortKeyHex, true},
		{"bare hex too short", shortKeyHex, true},
		{"not hex", nonHexSampleKey, true},
		{"empty", "", true},
		{"anchor share string", "bramble://anchor/v1?sk=" + sampleKeyHex, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNetworkKeyShare(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got key %x", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != NetworkKeySize {
				t.Fatalf("key length: got %d, want %d", len(got), NetworkKeySize)
			}
		})
	}
}

func TestGenerateNetworkKeySeed(t *testing.T) {
	a, err := GenerateNetworkKeySeed()
	if err != nil {
		t.Fatalf("GenerateNetworkKeySeed: %v", err)
	}
	if len(a) != NetworkKeySize {
		t.Fatalf("length: got %d, want %d", len(a), NetworkKeySize)
	}
	b, err := GenerateNetworkKeySeed()
	if err != nil {
		t.Fatalf("GenerateNetworkKeySeed: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two generated keys were identical")
	}
	if bytes.Equal(a, make([]byte, NetworkKeySize)) {
		t.Fatal("generated key was all zeros")
	}
}

func TestClient_SetNetworkKey(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.SetNetworkKey(ctx, sampleKeyHex); err != nil {
		t.Fatalf("SetNetworkKey error: %v", err)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent request, got %d", len(sent))
	}
	if !strings.Contains(sent[0], `"method":"bramble.setNetworkKey"`) {
		t.Fatalf("expected bramble.setNetworkKey request, got: %s", sent[0])
	}
	if !strings.Contains(sent[0], `"key":"`+sampleKeyHex+`"`) {
		t.Fatalf("expected the key in params, got: %s", sent[0])
	}
}

func TestClient_SetNetworkKey_DeviceRejection(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":false}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.SetNetworkKey(ctx, sampleKeyHex); err == nil {
		t.Fatal("expected an error when the device reports ok:false")
	}
}

func TestClient_NetworkKeyStatus_Provisioned(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"provisioned":true,"fingerprint":"deadbeef"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	st, err := c.NetworkKeyStatus(ctx)
	if err != nil {
		t.Fatalf("NetworkKeyStatus error: %v", err)
	}
	if !st.Provisioned {
		t.Fatal("provisioned: got false, want true")
	}
	if st.Fingerprint != "deadbeef" {
		t.Fatalf("fingerprint: got %q, want deadbeef", st.Fingerprint)
	}
	if !strings.Contains(mock.Sent()[0], `"method":"bramble.getNetworkKeyStatus"`) {
		t.Fatalf("expected bramble.getNetworkKeyStatus request, got: %s", mock.Sent()[0])
	}
}

func TestClient_NetworkKeyStatus_Unprovisioned(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"provisioned":false,"fingerprint":"00000000"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	st, err := c.NetworkKeyStatus(ctx)
	if err != nil {
		t.Fatalf("NetworkKeyStatus error: %v", err)
	}
	if st.Provisioned {
		t.Fatal("provisioned: got true, want false")
	}
	if st.Fingerprint != "00000000" {
		t.Fatalf("fingerprint: got %q, want the all-zero sentinel", st.Fingerprint)
	}
}

func TestClient_GenerateNetworkKey(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"key":"` + sampleKeyHex + `","fingerprint":"a1b2c3d4"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := c.GenerateNetworkKey(ctx)
	if err != nil {
		t.Fatalf("GenerateNetworkKey error: %v", err)
	}
	if resp.Key != sampleKeyHex {
		t.Fatalf("key: got %q, want %q", resp.Key, sampleKeyHex)
	}
	if resp.Fingerprint != "a1b2c3d4" {
		t.Fatalf("fingerprint: got %q, want a1b2c3d4", resp.Fingerprint)
	}
	if !strings.Contains(mock.Sent()[0], `"method":"bramble.generateNetworkKey"`) {
		t.Fatalf("expected bramble.generateNetworkKey request, got: %s", mock.Sent()[0])
	}
}

// The fingerprint the node reports for a generated key must match the one
// derived locally from that key, so an operator can compare a fingerprint
// computed offline against what every node displays.
func TestGenerateNetworkKey_FingerprintAgreesWithLocalDerivation(t *testing.T) {
	key, err := hex.DecodeString(sampleKeyHex)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	local := NetworkKeyFingerprint(key)

	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"key":"` + sampleKeyHex + `","fingerprint":"` + local + `"}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := c.GenerateNetworkKey(ctx)
	if err != nil {
		t.Fatalf("GenerateNetworkKey error: %v", err)
	}
	if resp.Fingerprint != local {
		t.Fatalf("node fingerprint %q disagrees with local derivation %q", resp.Fingerprint, local)
	}
}
