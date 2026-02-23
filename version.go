package bramble

// Protocol and SDK version constants.
const (
	// MinProtocolVersion is the minimum protocol version this SDK can speak.
	MinProtocolVersion = "0.1.0"

	// MaxProtocolVersion is the maximum protocol version this SDK can speak.
	MaxProtocolVersion = "0.2.1"

	// SDKVersion is the version of this SDK itself.
	SDKVersion = "0.2.1"
)

// IsCompatible reports whether the given firmware protocol version string
// is compatible with this SDK.
func IsCompatible(version string) bool {
	return version >= MinProtocolVersion && version <= MaxProtocolVersion
}
