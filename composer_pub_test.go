package vers

import "testing"

func TestParseComposerRangeErrors(t *testing.T) {
	for _, constraint := range []string{">=2.*", "^", "1.0 -", ">=1.0 ||"} {
		if _, err := ParseNative(constraint, schemeComposer); err == nil {
			t.Errorf("ParseNative(%q, composer) succeeded", constraint)
		}
	}
}

func TestParsePubRangeErrors(t *testing.T) {
	for _, constraint := range []string{"", "   ", "^1.2", "1.2.*", ">=1.0.0 || <2.0.0", ">=1.0.0, <2.0.0", "!=1.2.3"} {
		if _, err := ParseNative(constraint, schemePub); err == nil {
			t.Errorf("ParseNative(%q, pub) succeeded", constraint)
		}
	}
}

func TestComposerAndPubValidation(t *testing.T) {
	valid := []struct {
		version string
		scheme  string
	}{
		{version: "v1.2.3", scheme: schemeComposer},
		{version: "dev-main", scheme: schemeComposer},
		{version: "1.x-dev", scheme: schemeComposer},
		{version: "1.2.3", scheme: schemePub},
	}
	for _, test := range valid {
		if !ValidWithScheme(test.version, test.scheme) {
			t.Errorf("ValidWithScheme(%q, %q) = false", test.version, test.scheme)
		}
	}
	invalid := []struct {
		version string
		scheme  string
	}{
		{version: "not a version", scheme: schemeComposer},
		{version: "v1.2.3", scheme: schemePub},
		{version: "1.2", scheme: schemePub},
	}
	for _, test := range invalid {
		if ValidWithScheme(test.version, test.scheme) {
			t.Errorf("ValidWithScheme(%q, %q) = true", test.version, test.scheme)
		}
	}
}

func TestNormalizeComposerAndPubVersions(t *testing.T) {
	tests := []struct {
		version string
		scheme  string
		want    string
	}{
		{version: "v01.02.003", scheme: schemeComposer, want: "1.2.3"},
		{version: "001.02.0003-01.dev+pre.002", scheme: schemePub, want: "1.2.3-1.dev+pre.2"},
	}
	for _, test := range tests {
		got, err := NormalizeWithScheme(test.version, test.scheme)
		if err != nil {
			t.Fatalf("NormalizeWithScheme(%q, %q): %v", test.version, test.scheme, err)
		}
		if got != test.want {
			t.Errorf("NormalizeWithScheme(%q, %q) = %q, want %q", test.version, test.scheme, got, test.want)
		}
	}
}

func TestComposerComparisonNormalizesStabilityNumbers(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{left: "1.25.0-beta2.1", right: "1.25.0-b.3", want: -1},
		{left: "1.25.0-beta2.1", right: "1.25.0-b.2.1", want: 0},
		{left: "1.25.0beta2.1", right: "1.25.0-b2.1", want: 0},
		{left: "dev-main", right: "1.0.0", want: -1},
	}
	for _, test := range tests {
		if got := CompareWithScheme(test.left, test.right, schemeComposer); got != test.want {
			t.Errorf("CompareWithScheme(%q, %q, composer) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestComposerAndPubVersRoundTrip(t *testing.T) {
	tests := []struct {
		constraint string
		scheme     string
		inside     string
		outside    string
	}{
		{constraint: "~1.2", scheme: schemeComposer, inside: "1.2.0-dev", outside: "2.0.0-beta"},
		{constraint: ">=1.2.3 <2.0.0", scheme: schemePub, inside: "1.9.0", outside: "2.0.0-alpha"},
	}
	for _, test := range tests {
		native, err := ParseNative(test.constraint, test.scheme)
		if err != nil {
			t.Fatalf("ParseNative(%q, %q): %v", test.constraint, test.scheme, err)
		}
		uri := ToVersString(native, test.scheme)
		roundTrip, err := Parse(uri)
		if err != nil {
			t.Fatalf("Parse(%q): %v", uri, err)
		}
		if !roundTrip.Contains(test.inside) {
			t.Errorf("Parse(%q).Contains(%q) = false", uri, test.inside)
		}
		if roundTrip.Contains(test.outside) {
			t.Errorf("Parse(%q).Contains(%q) = true", uri, test.outside)
		}
	}
}
