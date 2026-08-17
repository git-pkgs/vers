package vers

import "testing"

func TestParseComposerRange(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		version    string
		want       bool
	}{
		{name: "wildcard matches all", constraint: "*", version: "99.0.0", want: true},
		{name: "x wildcard matches all", constraint: "x", version: "99.0.0", want: true},
		{name: "exact matches", constraint: "1.2.3", version: "1.2.3", want: true},
		{name: "exact excludes another version", constraint: "1.2.3", version: "1.2.4", want: false},
		{name: "double equals matches", constraint: "==1.2.3", version: "1.2.3", want: true},
		{name: "angle exclusion removes exact version", constraint: "<>1.2.3", version: "1.2.3", want: false},
		{name: "space intersection includes", constraint: ">=1.0 <2.0", version: "1.5.0", want: true},
		{name: "space intersection excludes upper", constraint: ">=1.0 <2.0", version: "2.0.0", want: false},
		{name: "greater equal includes lower dev", constraint: ">=1.2", version: "1.2.0-dev", want: true},
		{name: "less than excludes upper beta", constraint: "<2.0", version: "2.0.0-beta", want: false},
		{name: "comma intersection includes", constraint: ">=1.0, <2.0", version: "1.5.0", want: true},
		{name: "double pipe includes second range", constraint: "^1.0 || ^2.0", version: "2.5.0", want: true},
		{name: "alias in first OR branch preserves second", constraint: "dev-main as 2.x-dev || ^2.0", version: "2.5.0", want: true},
		{name: "single pipe includes second range", constraint: "^1.0 | ^2.0", version: "2.5.0", want: true},
		{name: "or excludes gap", constraint: "<1.1 || >=1.2", version: "1.1.5", want: false},
		{name: "partial hyphen includes completed upper", constraint: "1.0 - 2.0", version: "2.0.9", want: true},
		{name: "partial hyphen excludes next minor", constraint: "1.0 - 2.0", version: "2.1.0", want: false},
		{name: "full hyphen includes upper", constraint: "1.0.0 - 2.1.0", version: "2.1.0", want: true},
		{name: "minor wildcard includes", constraint: "1.2.*", version: "1.2.9", want: true},
		{name: "minor wildcard includes lower dev", constraint: "1.2.*", version: "1.2.0-dev", want: true},
		{name: "minor wildcard excludes next minor", constraint: "1.2.*", version: "1.3.0", want: false},
		{name: "repeated wildcard includes", constraint: "2.*.*", version: "2.9.0", want: true},
		{name: "tilde partial includes later minor", constraint: "~1.2", version: "1.9.0", want: true},
		{name: "tilde partial includes lower dev", constraint: "~1.2", version: "1.2.0-dev", want: true},
		{name: "tilde partial excludes next major prerelease", constraint: "~1.2", version: "2.0.0-beta.1", want: false},
		{name: "tilde full excludes next minor", constraint: "~1.2.3", version: "1.3.0", want: false},
		{name: "caret accepts shorthand stability", constraint: "^1.2.3beta1", version: "1.2.3-beta2", want: true},
		{name: "tilde accepts dotted stability", constraint: "~1.2.3.RC1", version: "1.2.3-RC2", want: true},
		{name: "caret zero minor includes patch", constraint: "^0.3", version: "0.3.9", want: true},
		{name: "caret includes lower dev", constraint: "^1.2", version: "1.2.0-dev", want: true},
		{name: "caret zero minor excludes next minor", constraint: "^0.3", version: "0.4.0", want: false},
		{name: "four component caret uses patch boundary", constraint: "^0.0.0.3", version: "0.0.0.9", want: true},
		{name: "four component caret excludes next patch", constraint: "^0.0.0.3", version: "0.0.1.0-dev", want: false},
		{name: "exclusion removes exact version", constraint: ">=1.0 !=1.5", version: "1.5.0", want: false},
		{name: "stability flag is accepted", constraint: "^1.2@beta", version: "1.5.0", want: true},
		{name: "stability flag sets comparator bound", constraint: ">=1.0@beta", version: "1.0.0-alpha", want: false},
		{name: "stability flag includes matching stability", constraint: ">=1.0@beta", version: "1.0.0-beta", want: true},
		{name: "standalone stability flag matches all", constraint: "@dev", version: "1.0.0-dev", want: true},
		{name: "hyphen includes lower dev", constraint: "1.0 - 2.0", version: "1.0.0-dev", want: true},
		{name: "hyphen prerelease upper is inclusive", constraint: "1.0 - 2.1-beta", version: "2.1.0-beta", want: true},
		{name: "branch alias uses branch constraint", constraint: "dev-main as 1.0.x-dev", version: "dev-main", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := ParseNative(test.constraint, schemeComposer)
			if err != nil {
				t.Fatalf("ParseNative(%q, composer): %v", test.constraint, err)
			}
			if got := r.Contains(test.version); got != test.want {
				t.Errorf("Contains(%q) = %v, want %v", test.version, got, test.want)
			}
		})
	}
}

func TestParseComposerRangeErrors(t *testing.T) {
	for _, constraint := range []string{">=2.*", "^", "1.0 -", ">=1.0 ||"} {
		if _, err := ParseNative(constraint, schemeComposer); err == nil {
			t.Errorf("ParseNative(%q, composer) succeeded", constraint)
		}
	}
}

func TestParsePubRange(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		version    string
		want       bool
	}{
		{name: "any matches all", constraint: "any", version: "99.0.0", want: true},
		{name: "exact matches", constraint: "1.2.3", version: "1.2.3", want: true},
		{name: "exact excludes another version", constraint: "1.2.3", version: "1.2.4", want: false},
		{name: "traditional range includes", constraint: ">=1.2.3 <2.0.0", version: "1.9.0", want: true},
		{name: "traditional range excludes lower", constraint: ">=1.2.3 <2.0.0", version: "1.2.2", want: false},
		{name: "traditional range excludes upper", constraint: ">=1.2.3 <2.0.0", version: "2.0.0", want: false},
		{name: "traditional range excludes upper prerelease", constraint: ">=1.2.3 <2.0.0", version: "2.0.0-alpha", want: false},
		{name: "caret includes compatible major", constraint: "^1.2.3", version: "1.9.0", want: true},
		{name: "caret excludes next major", constraint: "^1.2.3", version: "2.0.0", want: false},
		{name: "caret excludes next major prerelease", constraint: "^1.2.3", version: "2.0.0-alpha", want: false},
		{name: "caret zero includes compatible patch", constraint: "^0.1.2", version: "0.1.9", want: true},
		{name: "caret zero excludes next minor", constraint: "^0.1.2", version: "0.2.0", want: false},
		{name: "caret zero zero includes later patch", constraint: "^0.0.3", version: "0.0.9", want: true},
		{name: "caret zero zero excludes next minor prerelease", constraint: "^0.0.3", version: "0.1.0-alpha", want: false},
		{name: "adjacent constraints include", constraint: ">1.0.0>=1.2.3 < 1.3.0", version: "1.2.5", want: true},
		{name: "greater than includes build", constraint: ">1.2.3", version: "1.2.3+foo", want: true},
		{name: "less than excludes upper prerelease", constraint: "<1.2.3", version: "1.2.3-alpha", want: false},
		{name: "less than excludes upper build", constraint: "<1.2.3", version: "1.2.3+foo", want: false},
		{name: "prerelease range includes later prerelease", constraint: ">=1.2.3-alpha <1.2.3", version: "1.2.3-beta", want: true},
		{name: "exact excludes build", constraint: "1.2.3", version: "1.2.3+foo", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := ParseNative(test.constraint, schemePub)
			if err != nil {
				t.Fatalf("ParseNative(%q, pub): %v", test.constraint, err)
			}
			if got := r.Contains(test.version); got != test.want {
				t.Errorf("Contains(%q) = %v, want %v", test.version, got, test.want)
			}
		})
	}
}

func TestParsePubRangeErrors(t *testing.T) {
	for _, constraint := range []string{"", "   ", "^1.2", "1.2.*", ">=1.0.0 || <2.0.0", ">=1.0.0, <2.0.0", "!=1.2.3"} {
		if _, err := ParseNative(constraint, schemePub); err == nil {
			t.Errorf("ParseNative(%q, pub) succeeded", constraint)
		}
	}
}

func TestComposerAndPubVersionComparison(t *testing.T) {
	composerVersions := []string{
		"1.0.0-dev",
		"1.0.0-alpha2",
		"1.0.0-beta1",
		"1.0.0-RC1",
		"1.0.0",
	}
	for i := 1; i < len(composerVersions); i++ {
		if got := CompareWithScheme(composerVersions[i-1], composerVersions[i], schemeComposer); got >= 0 {
			t.Errorf("CompareWithScheme(%q, %q, composer) = %d, want less", composerVersions[i-1], composerVersions[i], got)
		}
	}
	if got := CompareWithScheme("1.0.0-beta2", "1.0.0-beta10", schemeComposer); got >= 0 {
		t.Errorf("Composer beta2 comparison = %d, want less than beta10", got)
	}
	if got := CompareWithScheme("1.2.3+2", "1.2.3+11", schemePub); got >= 0 {
		t.Errorf("Pub numeric build comparison = %d, want less", got)
	}
	if got := CompareWithScheme("1.2.3", "1.2.3+one", schemePub); got >= 0 {
		t.Errorf("Pub empty build comparison = %d, want less", got)
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

func TestComposerAndPubVersOutput(t *testing.T) {
	tests := []struct {
		constraint string
		scheme     string
		want       string
	}{
		{constraint: "~1.2", scheme: schemeComposer, want: "vers:composer/>=1.2.0-dev|<2.0.0-dev"},
		{constraint: "^1.2.3", scheme: schemePub, want: "vers:pub/>=1.2.3|<2.0.0-0"},
		{constraint: "^0.0.3", scheme: schemePub, want: "vers:pub/>=0.0.3|<0.1.0-0"},
		{constraint: ">=1.2.3 <2.0.0", scheme: schemePub, want: "vers:pub/>=1.2.3|<2.0.0-0"},
	}
	for _, test := range tests {
		r, err := ParseNative(test.constraint, test.scheme)
		if err != nil {
			t.Fatalf("ParseNative(%q, %q): %v", test.constraint, test.scheme, err)
		}
		if got := ToVersString(r, test.scheme); got != test.want {
			t.Errorf("ToVersString() = %q, want %q", got, test.want)
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
