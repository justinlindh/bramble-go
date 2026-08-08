package bramble

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// Network-key client-side helpers. The network key is the fleet's symmetric
// control-plane secret: it authenticates RREP, RERR, ACK, delivery receipt and
// beacon MACs, and a node without one is INERT (it neither emits nor accepts
// authenticated control traffic). These helpers mint a key offline, carry one
// between nodes as a share string, and derive the one-way fingerprint used to
// confirm a fleet converged, byte-compatible with the firmware's
// network_key_fingerprint and the webapp's networkKeyShare.ts.
//
// The key itself is write-only at the device boundary: a node never reads a
// stored key back, so whatever mints the key is the only copy that exists
// until the operator records it.

// NetworkKeySize is the raw network key length in bytes.
const NetworkKeySize = 32

const networkKeySharePrefix = "bramble://net/v1?"

// GenerateNetworkKeySeed returns a fresh random 32-byte network key minted on
// this host. Prefer Client.GenerateNetworkKey, which mints on the node from its
// entropy-gated source and provisions atomically; use this only when founding a
// fleet from a host that will distribute the key to every node itself.
func GenerateNetworkKeySeed() ([]byte, error) {
	key := make([]byte, NetworkKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("bramble: generate network key: %w", err)
	}
	return key, nil
}

// NetworkKeyFingerprint is SHA256(key)[0:4] as 8 lowercase hex, matching the
// firmware's network_key_fingerprint and getNetworkKeyStatus.fingerprint. Two
// nodes reporting the same fingerprint hold the same key; the fingerprint never
// reveals the key.
func NetworkKeyFingerprint(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:4])
}

// EncodeNetworkKeyShare encodes a network key as the share URI the webapp's QR
// code and paste field emit. The result carries the raw key: treat it as the
// secret it is.
func EncodeNetworkKeyShare(key []byte) (string, error) {
	if len(key) != NetworkKeySize {
		return "", fmt.Errorf("bramble: network key must be %d bytes, got %d", NetworkKeySize, len(key))
	}
	params := url.Values{}
	params.Set("k", hex.EncodeToString(key))
	return networkKeySharePrefix + params.Encode(), nil
}

// ParseNetworkKeyShare extracts the network key from a bramble://net/v1?k=
// share string (or a bare 64-hex key).
func ParseNetworkKeyShare(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, networkKeySharePrefix); ok {
		params, err := url.ParseQuery(rest)
		if err != nil {
			return nil, fmt.Errorf("bramble: network key share: malformed query: %w", err)
		}
		k := params.Get("k")
		if k == "" {
			return nil, fmt.Errorf("bramble: network key share: missing k parameter")
		}
		return decodeHexLen(k, NetworkKeySize, "network key")
	}
	return decodeHexLen(s, NetworkKeySize, "network key")
}
