package vers

import "testing"

func TestSemverSchemeComparison(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0-2", "1.0.0-0a", -1},
		{"1.0.0-999999999999999999999999", "1.0.0-1000000000000000000000000", -1},
		{"999999999999999999999999.0.0", "888888888888888888888888.0.0", 1},
		{"1.0.0+one", "1.0.0+two", 0},
	}
	for _, scheme := range []string{"semver", "npm", "cargo", "go", "golang", "hex", "elixir"} {
		for _, tt := range tests {
			if got := CompareWithScheme(tt.a, tt.b, scheme); got != tt.want {
				t.Errorf("CompareWithScheme(%q, %q, %q) = %d, want %d", tt.a, tt.b, scheme, got, tt.want)
			}
		}
	}
	if got := Compare("999999999999999999999999.0.0", "888888888888888888888888.0.0"); got != 1 {
		t.Errorf("Compare() with arbitrary-width SemVer components = %d, want 1", got)
	}
}

func TestGemSchemeComparison(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0.1", -1},
		{"1.0.beta1", "1.0", -1},
		{"1.0.a10", "1.0.a2", 1},
		{"1.0-1", "1.0.pre.1", 0},
		{"1.0", "1.0.0", 0},
	}
	for _, scheme := range []string{"gem", "rubygems"} {
		for _, tt := range tests {
			if got := CompareWithScheme(tt.a, tt.b, scheme); got != tt.want {
				t.Errorf("CompareWithScheme(%q, %q, %q) = %d, want %d", tt.a, tt.b, scheme, got, tt.want)
			}
		}
	}
}

func TestDebianSchemeComparison(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0~rc1", "1.0", -1},
		{"1:1.0", "2.0", 1},
		{"1.0", "1.0-0", 0},
		{"1.0-1", "1.0-2", -1},
		{"1.0~~", "1.0~", -1},
		{"1.0+a", "1.0~a", 1},
		{"0.0.0", "0:0.0.0", 0},
		{"0:0.0.0-foo", "0.0.0-foo", 0},
		{"1.2.3-1~deb7u1", "1.2.3-1", -1},
		{"2.7.4+reloaded2-13ubuntu1", "2.7.4+reloaded2-13+deb9u1", -1},
	}
	for _, scheme := range []string{"deb", "debian"} {
		for _, tt := range tests {
			if got := CompareWithScheme(tt.a, tt.b, scheme); got != tt.want {
				t.Errorf("CompareWithScheme(%q, %q, %q) = %d, want %d", tt.a, tt.b, scheme, got, tt.want)
			}
		}
	}
}

func TestRPMSchemeComparison(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0~rc1", "1.0", -1},
		{"1:1.0", "2.0", 1},
		{"1.0", "1.0-1", -1},
		{"2.0^20250611", "2.0", 1},
		{"2.0^20250611", "2.0.1", -1},
		{"1.xyz", "1.0", -1},
		{"2.0.1a", "2.0.1", 1},
		{"1.0p10", "1.0p2", 1},
		{"1.01", "1.1", 0},
		{"1.0", "1_0", 0},
		{"1.0", "1.0+", 0},
		{"2.0", "2.0.1", -1},
		{"5.5p1", "5.5p2", -1},
		{"5.5p1", "5.5p10", -1},
		{"10xyz", "10.1xyz", -1},
		{"xyz10", "xyz10.1", -1},
		{"xyz.4", "8", -1},
		{"6.0.rc1", "6.0", 1},
		{"10b2", "10a1", 1},
		{"1.0a", "1.0aa", -1},
		{"10.0001", "10.0039", -1},
		{"20101121", "20101122", -1},
		{"a+", "a_", 0},
		{"+", "_", 0},
		{"1.0~rc1", "1.0~rc2", -1},
		{"1.0~rc1~git123", "1.0~rc1", -1},
		{"1.0^", "1.0", 1},
		{"1.0^git1", "1.0^git2", -1},
		{"1.0^git1", "1.01", -1},
		{"1.0^20160101", "1.0.1", -1},
		{"1.0^20160102", "1.0^20160101^git1", 1},
		{"1.0~rc1^git1", "1.0~rc1", 1},
		{"1.0^git1~pre", "1.0^git1", -1},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.a, tt.b, "rpm"); got != tt.want {
			t.Errorf("CompareWithScheme(%q, %q, rpm) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestExistingSchemeComparatorEdges(t *testing.T) {
	tests := []struct {
		scheme, a, b string
		want         int
	}{
		{"nuget", "1.0.0-2", "1.0.0-0a", -1},
		{"nuget", "1.0.0-999999999999999999999999", "1.0.0-1000000000000000000000000", -1},
		{"maven", "1.999999999999999999999999", "1.2", 1},
		{"lexicographic", "10", "2", -1},
		{"intdot", "1.0.0.1", "1.0.0", 1},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.a, tt.b, tt.scheme); got != tt.want {
			t.Errorf("CompareWithScheme(%q, %q, %q) = %d, want %d", tt.a, tt.b, tt.scheme, got, tt.want)
		}
	}
}

func TestSchemeAwareSharedAPIs(t *testing.T) {
	best, err := HighestSatisfying([]string{"1.0.dev1", "1.0a1"}, "vers:pypi/*", "")
	if err != nil {
		t.Fatal(err)
	}
	if best != "1.0a1" {
		t.Errorf("HighestSatisfying pypi VERS = %q, want 1.0a1", best)
	}

	r, err := Parse("vers:pypi/>=1.0.dev1|<1.0a1")
	if err != nil {
		t.Fatal(err)
	}
	if got := r.String(); got == "empty" {
		t.Errorf("Range.String() = %q for a non-empty pypi range", got)
	}
	if got := ToVersString(r, "pypi"); got != "vers:pypi/>=1.0.dev1|<1.0a1" {
		t.Errorf("ToVersString() = %q, want scheme-sorted constraints", got)
	}

	c, err := ParseConstraintWithScheme("<1.0a1", "pypi")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Satisfies("1.0.dev1") {
		t.Error("scheme-aware constraint should order dev before alpha")
	}

	interval := NewInterval("1.0.dev1", "1.0a1", true, false)
	if interval.IsEmptyWithScheme("pypi") || !interval.ContainsWithScheme("1.0.dev2", "pypi") {
		t.Error("scheme-aware interval methods should use PEP 440 ordering")
	}
}

func TestCheckedRangeAlgebraRejectsMixedSchemes(t *testing.T) {
	pypi, _ := Parse("vers:pypi/>=1.0")
	maven, _ := Parse("vers:maven/<2.0")
	if _, err := pypi.UnionChecked(maven); err == nil {
		t.Error("UnionChecked should reject different schemes")
	}
	if _, err := pypi.IntersectChecked(maven); err == nil {
		t.Error("IntersectChecked should reject different schemes")
	}

	generic := NewRange([]Interval{GreaterThanInterval("1.0.dev1", true)})
	typedBase, _ := Parse("vers:pypi/*")
	typed, err := generic.IntersectChecked(typedBase)
	if err != nil {
		t.Fatal(err)
	}
	if typed.Scheme != "pypi" || !typed.Contains("1.0a1") {
		t.Error("checked range algebra should inherit the non-empty scheme")
	}
}

func TestSchemeAwareValidationAndNormalization(t *testing.T) {
	tests := []struct {
		scheme, input, normalized string
	}{
		{"pypi", "01!02.0RC1", "1!2.0rc1"},
		{"gem", "1.0.beta1", "1.0.beta1"},
		{"deb", "1:2.3.4-1", "1:2.3.4-1"},
		{"rpm", "1.0~rc1", "1.0~rc1"},
		{"npm", "1.2", "1.2.0"},
		{"go", "v1.2", "v1.2.0"},
	}
	for _, tt := range tests {
		if !ValidWithScheme(tt.input, tt.scheme) {
			t.Errorf("ValidWithScheme(%q, %q) = false", tt.input, tt.scheme)
			continue
		}
		got, err := NormalizeWithScheme(tt.input, tt.scheme)
		if err != nil {
			t.Errorf("NormalizeWithScheme(%q, %q) error = %v", tt.input, tt.scheme, err)
			continue
		}
		if got != tt.normalized {
			t.Errorf("NormalizeWithScheme(%q, %q) = %q, want %q", tt.input, tt.scheme, got, tt.normalized)
		}
		if CompareWithScheme(got, tt.input, tt.scheme) != 0 {
			t.Errorf("normalized %q is not equivalent to %q under %s", got, tt.input, tt.scheme)
		}
	}
	if ValidWithScheme("x:1.0", "deb") {
		t.Error("ValidWithScheme accepted a non-numeric Debian epoch")
	}
	if _, err := NormalizeWithScheme("x:1.0", "deb"); err == nil {
		t.Error("NormalizeWithScheme accepted an invalid Debian version")
	}
}

func TestNativeRangeSchemeEdges(t *testing.T) {
	tests := []struct {
		scheme, constraint, version string
		want                        bool
	}{
		{"cargo", "1.2.3", "1.9.0", true},
		{"cargo", "1.2.3", "2.0.0", false},
		{"gem", "~> 1.0.beta1", "1.9", true},
		{"gem", "~> 1.0.beta1", "2.0", false},
		{"gem", "~> 1.0.0.beta1", "1.0.9", true},
		{"gem", "~> 1.0.0.beta1", "1.1", false},
		{"gem", "~> 1.0, < 1.5", "1.4", true},
		{"gem", "~> 1.0, < 1.5", "1.6", false},
		{"cargo", ">=1.2.3, <2.0.0", "1.5.0", true},
	}
	for _, tt := range tests {
		got, err := Satisfies(tt.version, tt.constraint, tt.scheme)
		if err != nil {
			t.Errorf("Satisfies(%q, %q, %q) error = %v", tt.version, tt.constraint, tt.scheme, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Satisfies(%q, %q, %q) = %v, want %v", tt.version, tt.constraint, tt.scheme, got, tt.want)
		}
	}
}

func TestOpenSSLLegacyPatchOrdering(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0a", -1},
		{"1.0.0-beta1", "1.0.0", -1},
		{"1.0.0z", "3.0.0", -1},
		{"3.0.0-rc1", "3.0.0", -1},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.a, tt.b, "openssl"); got != tt.want {
			t.Errorf("CompareWithScheme(%q, %q, openssl) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
