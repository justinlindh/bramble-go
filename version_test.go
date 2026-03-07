package bramble

import "testing"

func TestIsCompatible_EdgeCaseSemverOrdering(t *testing.T) {
	if IsCompatible("0.10.0") {
		t.Fatalf("expected 0.10.0 to be incompatible because it is greater than max 0.5.0")
	}
}

func TestIsVersionInRange_NormalRange(t *testing.T) {
	if !isVersionInRange("0.5.0", "0.3.0", "0.7.0") {
		t.Fatalf("expected 0.5.0 to be in [0.3.0, 0.7.0]")
	}
}

func TestIsVersionInRange_EdgeCaseSemverOrdering(t *testing.T) {
	if !isVersionInRange("0.10.0", "0.9.0", "0.10.0") {
		t.Fatalf("expected 0.10.0 to be >= 0.9.0 and <= 0.10.0")
	}
}

func TestIsVersionInRange_EqualBounds(t *testing.T) {
	if !isVersionInRange("0.5.0", "0.5.0", "0.5.0") {
		t.Fatalf("expected 0.5.0 to be in [0.5.0, 0.5.0]")
	}
}

func TestIsVersionInRange_OutOfRange(t *testing.T) {
	if isVersionInRange("0.2.9", "0.3.0", "0.7.0") {
		t.Fatalf("expected 0.2.9 to be out of [0.3.0, 0.7.0]")
	}
	if isVersionInRange("0.7.1", "0.3.0", "0.7.0") {
		t.Fatalf("expected 0.7.1 to be out of [0.3.0, 0.7.0]")
	}
}

func TestIsVersionInRange_VPrefix(t *testing.T) {
	if !isVersionInRange("v0.5.0", "0.3.0", "0.7.0") {
		t.Fatalf("expected v0.5.0 to be treated as semver 0.5.0")
	}
}
