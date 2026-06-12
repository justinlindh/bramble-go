package bramble

import (
	"strconv"
	"strings"
)

// Protocol and SDK version constants.
const (
	// MinProtocolVersion is the minimum protocol version this SDK can speak.
	MinProtocolVersion = "0.1.0"

	// MaxProtocolVersion is the maximum protocol version this SDK can speak.
	MaxProtocolVersion = "0.5.0"

	// SDKVersion is the version of this SDK itself. Releases are tagged by
	// semantic-release; bump this alongside the release tag when it moves.
	SDKVersion = "0.9.0"
)

// IsCompatible reports whether the given firmware protocol version string
// is compatible with this SDK.
func IsCompatible(version string) bool {
	return isVersionInRange(version, MinProtocolVersion, MaxProtocolVersion)
}

func isVersionInRange(version, min, max string) bool {
	return compareVersion(version, min) >= 0 && compareVersion(version, max) <= 0
}

func compareVersion(a, b string) int {
	aParts, ok := parseVersion(a)
	if !ok {
		return -1
	}
	bParts, ok := parseVersion(b)
	if !ok {
		return -1
	}

	for i := range 3 {
		switch {
		case aParts[i] < bParts[i]:
			return -1
		case aParts[i] > bParts[i]:
			return 1
		}
	}
	return 0
}

func parseVersion(v string) ([3]int, bool) {
	var parts [3]int

	v = strings.TrimPrefix(v, "v")
	segments := strings.Split(v, ".")
	if len(segments) != 3 {
		return parts, false
	}

	for i, segment := range segments {
		n, err := strconv.Atoi(segment)
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}

	return parts, true
}
