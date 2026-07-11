package vers

import "testing"

// Issue #24: PyPI prerelease versions are mis-parsed (5.2b1 becomes 5.0.0).
// https://github.com/git-pkgs/vers/issues/24

func TestIssue24Normalize(t *testing.T) {
	// Normalize is scheme-agnostic so we don't assert an exact PEP 440 form,
	// only that the prerelease information is not silently dropped.
	tests := []struct {
		in       string
		notEqual string
	}{
		{"5.2b1", "5.0.0"},
		{"1.0a1", "1.0.0"},
		{"1.0rc1", "1.0.0"},
		{"1.0.dev1", "1.0.0"},
		{"1.0.post1", "1.0.0"},
	}
	for _, tt := range tests {
		got, err := Normalize(tt.in)
		if err != nil {
			t.Errorf("Normalize(%q) error = %v", tt.in, err)
			continue
		}
		if got == tt.notEqual {
			t.Errorf("Normalize(%q) = %q, prerelease info was dropped", tt.in, got)
		}
	}
}

func TestIssue24CompareWithScheme(t *testing.T) {
	if got := CompareWithScheme("5.1", "5.2b1", "pypi"); got != -1 {
		t.Errorf("CompareWithScheme(5.1, 5.2b1, pypi) = %d, want -1", got)
	}
}

func TestIssue24RangeContains(t *testing.T) {
	r, err := Parse("vers:pypi/<5.2b1|>=5.1")
	if err != nil {
		t.Fatalf("Parse error = %v", err)
	}
	if r.Contains("6.0.3") {
		t.Errorf("vers:pypi/<5.2b1|>=5.1 should not contain 6.0.3")
	}
	if !r.Contains("5.1") {
		t.Errorf("vers:pypi/<5.2b1|>=5.1 should contain 5.1")
	}
	if !r.Contains("5.2a1") {
		t.Errorf("vers:pypi/<5.2b1|>=5.1 should contain 5.2a1")
	}
	if r.Contains("5.2b1") {
		t.Errorf("vers:pypi/<5.2b1|>=5.1 should not contain 5.2b1 (exclusive upper bound)")
	}
	if r.Contains("5.0") {
		t.Errorf("vers:pypi/<5.2b1|>=5.1 should not contain 5.0")
	}
}

// TestPyPICompareOrdering verifies the ordering example from PEP 440
// (https://peps.python.org/pep-0440/#summary-of-permitted-suffixes-and-relative-ordering).
// Each entry must compare strictly less than the one after it.
func TestPyPICompareOrdering(t *testing.T) {
	ordered := []string{
		"1.dev0",
		"1.0.dev456",
		"1.0a1",
		"1.0a2.dev456",
		"1.0a12.dev456",
		"1.0a12",
		"1.0b1.dev456",
		"1.0b2",
		"1.0b2.post345.dev456",
		"1.0b2.post345",
		"1.0rc1.dev456",
		"1.0rc1",
		"1.0",
		"1.0+abc.5",
		"1.0+abc.7",
		"1.0+5",
		"1.0.post456.dev34",
		"1.0.post456",
		"1.1.dev1",
	}
	for i := 0; i < len(ordered)-1; i++ {
		a, b := ordered[i], ordered[i+1]
		if got := CompareWithScheme(a, b, "pypi"); got != -1 {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want -1", a, b, got)
		}
		if got := CompareWithScheme(b, a, "pypi"); got != 1 {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want 1", b, a, got)
		}
	}
}

// TestPyPICompareEqual verifies PEP 440 normalization: different spellings of
// the same version compare equal.
func TestPyPICompareEqual(t *testing.T) {
	tests := []struct{ a, b string }{
		// release segment padding
		{"1", "1.0"},
		{"1.0", "1.0.0"},
		{"1.0.0", "1.0.0.0"},
		// case insensitivity and v prefix
		{"1.0RC1", "1.0rc1"},
		{"v1.0", "1.0"},
		// pre-release separators
		{"1.1a1", "1.1.a1"},
		{"1.1a1", "1.1-a1"},
		{"1.1a1", "1.1_a1"},
		// pre-release spelling
		{"1.0a1", "1.0alpha1"},
		{"1.0b2", "1.0beta2"},
		{"1.0rc1", "1.0c1"},
		{"1.0rc1", "1.0pre1"},
		{"1.0rc1", "1.0preview1"},
		// implicit pre-release number
		{"1.2a", "1.2a0"},
		// post release spelling and separators
		{"1.0.post1", "1.0-post1"},
		{"1.0.post1", "1.0post1"},
		{"1.0.post1", "1.0.rev1"},
		{"1.0.post1", "1.0.r1"},
		// implicit post release (bare -N)
		{"1.0-1", "1.0.post1"},
		// implicit post release number
		{"1.0.post", "1.0.post0"},
		// dev separators
		{"1.0.dev1", "1.0-dev1"},
		{"1.0.dev1", "1.0dev1"},
		// implicit dev number
		{"1.0.dev", "1.0.dev0"},
		// leading zeros in release segments
		{"1.01", "1.1"},
		{"01.1", "1.1"},
		// local version separators
		{"1.0+ubuntu.1", "1.0+ubuntu-1"},
		{"1.0+ubuntu.1", "1.0+ubuntu_1"},
		// local version case
		{"1.0+ABC", "1.0+abc"},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.a, tt.b, "pypi"); got != 0 {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want 0", tt.a, tt.b, got)
		}
		if got := CompareWithScheme(tt.b, tt.a, "pypi"); got != 0 {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want 0", tt.b, tt.a, got)
		}
	}
}

func TestPyPICompareEpoch(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1!1.0", "2.0", 1},
		{"2!1.0", "1!2.0", 1},
		{"1.0", "0!1.0", 0},
		{"1!1.0", "1!1.0.0", 0},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.a, tt.b, "pypi"); got != tt.want {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPyPICompareLocal(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// no local < any local
		{"1.0", "1.0+1", -1},
		// numeric segments compared numerically
		{"1.0+2", "1.0+10", -1},
		// numeric segment > string segment
		{"1.0+abc", "1.0+1", -1},
		// longer local with matching prefix sorts higher
		{"1.0+a", "1.0+a.1", -1},
		// string segments compared lexically
		{"1.0+a", "1.0+b", -1},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.a, tt.b, "pypi"); got != tt.want {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := CompareWithScheme(tt.b, tt.a, "pypi"); got != -tt.want {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want %d", tt.b, tt.a, got, -tt.want)
		}
	}
}

func TestPyPIComparePairs(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// dev < alpha < beta < rc < release < post
		{"1.0.dev1", "1.0a1", -1},
		{"1.0a1", "1.0b1", -1},
		{"1.0b1", "1.0rc1", -1},
		{"1.0rc1", "1.0", -1},
		{"1.0", "1.0.post1", -1},
		// numeric ordering within each phase
		{"1.0a2", "1.0a10", -1},
		{"1.0.post2", "1.0.post10", -1},
		{"1.0.dev2", "1.0.dev10", -1},
		// release segment ordering with different lengths
		{"1.0", "1.0.1", -1},
		{"1.0.1", "1.1", -1},
		// pre-release with dev sorts before same pre-release without
		{"1.0a1.dev1", "1.0a1", -1},
		// post on a pre-release sorts after that pre-release, before next pre-release
		{"1.0a1", "1.0a1.post1", -1},
		{"1.0a1.post1", "1.0a2", -1},
		// dev-only release sorts before all pre-releases of that release
		{"1.0.dev1", "1.0rc1", -1},
		// but after post of the previous release
		{"0.9.post1", "1.0.dev1", -1},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.a, tt.b, "pypi"); got != tt.want {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := CompareWithScheme(tt.b, tt.a, "pypi"); got != -tt.want {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want %d", tt.b, tt.a, got, -tt.want)
		}
	}
}

func TestPyPIRangeContains(t *testing.T) {
	tests := []struct {
		versURI string
		version string
		want    bool
	}{
		// bounded prerelease window
		{"vers:pypi/>=5.1|<5.2b1", "5.1", true},
		{"vers:pypi/>=5.1|<5.2b1", "5.1.9", true},
		{"vers:pypi/>=5.1|<5.2b1", "5.2a1", true},
		{"vers:pypi/>=5.1|<5.2b1", "5.2b1", false},
		{"vers:pypi/>=5.1|<5.2b1", "5.2", false},
		{"vers:pypi/>=5.1|<5.2b1", "6.0.3", false},
		// constraint order reversed (as in the issue)
		{"vers:pypi/<5.2b1|>=5.1", "6.0.3", false},
		{"vers:pypi/<5.2b1|>=5.1", "5.1.5", true},
		// upper bound is a release, prereleases of that release are inside
		{"vers:pypi/>=1.0|<2.0", "2.0a1", true},
		{"vers:pypi/>=1.0|<2.0", "2.0", false},
		{"vers:pypi/>=1.0|<2.0", "1.0", true},
		// dev release below lower bound
		{"vers:pypi/>=1.0|<2.0", "1.0.dev1", false},
		// post release above upper bound
		{"vers:pypi/>=1.0|<=2.0", "2.0.post1", false},
		{"vers:pypi/>=1.0|<=2.0", "2.0", true},
		// exclusion with PEP 440 equality
		{"vers:pypi/>=1.0|!=1.5.0|<2.0", "1.5", false},
		{"vers:pypi/>=1.0|!=1.5.0|<2.0", "1.5.1", true},
		// epoch in range bound
		{"vers:pypi/>=1!1.0", "2.0", false},
		{"vers:pypi/>=1!1.0", "1!1.0", true},
		{"vers:pypi/>=1!1.0", "1!2.0", true},
	}
	for _, tt := range tests {
		r, err := Parse(tt.versURI)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", tt.versURI, err)
			continue
		}
		if got := r.Contains(tt.version); got != tt.want {
			t.Errorf("Parse(%q).Contains(%q) = %v, want %v", tt.versURI, tt.version, got, tt.want)
		}
	}
}

func TestPyPIRangeIsEmpty(t *testing.T) {
	// [1.0.dev1, 1.0a1) is non-empty under PEP 440 even though generic
	// comparison would order dev1 > a1.
	r, err := Parse("vers:pypi/>=1.0.dev1|<1.0a1")
	if err != nil {
		t.Fatalf("Parse error = %v", err)
	}
	if r.IsEmpty() {
		t.Errorf("vers:pypi/>=1.0.dev1|<1.0a1 IsEmpty() = true, want false")
	}
	if !r.Contains("1.0.dev2") {
		t.Errorf("vers:pypi/>=1.0.dev1|<1.0a1 should contain 1.0.dev2")
	}
	if got := defaultParser.ToVersString(r, "pypi"); got == "vers:pypi/" {
		t.Errorf("ToVersString serialized non-empty range as %q", got)
	}
}

func TestPyPISchemePropagation(t *testing.T) {
	// ParseNative ~= path
	ok, err := Satisfies("1.0a1", "~=1.0.dev1", "pypi")
	if err != nil {
		t.Fatalf("Satisfies error = %v", err)
	}
	if !ok {
		t.Errorf("Satisfies(1.0a1, ~=1.0.dev1, pypi) = false, want true")
	}

	// Wildcard
	r, _ := Parse("vers:pypi/*")
	if r.Scheme != "pypi" {
		t.Errorf("Parse(vers:pypi/*).Scheme = %q, want pypi", r.Scheme)
	}

	// Exclude preserves scheme
	r, _ = Parse("vers:pypi/>=1.0")
	r2 := r.Exclude("1.5")
	if r2.Scheme != "pypi" {
		t.Errorf("Exclude dropped scheme: got %q", r2.Scheme)
	}
	if r2.Contains("1.5.0") {
		t.Errorf("excluded 1.5 should also exclude 1.5.0 under PEP 440 equality")
	}

	// Union and Intersect preserve scheme
	a, _ := Parse("vers:pypi/>=1.0|<2.0")
	b, _ := Parse("vers:pypi/>=1.5|<3.0")
	if u := a.Union(b); u.Scheme != "pypi" {
		t.Errorf("Union dropped scheme: got %q", u.Scheme)
	}
	if i := a.Intersect(b); i.Scheme != "pypi" {
		t.Errorf("Intersect dropped scheme: got %q", i.Scheme)
	}
}

func TestPyPIRangeAlgebra(t *testing.T) {
	a, _ := Parse("vers:pypi/>=1.0.dev1")
	b, _ := Parse("vers:pypi/<1.0a1")

	i := a.Intersect(b)
	if i.IsEmpty() {
		t.Errorf("Intersect(>=1.0.dev1, <1.0a1) is empty, want [1.0.dev1, 1.0a1)")
	}
	if !i.Contains("1.0.dev2") {
		t.Errorf("Intersect(>=1.0.dev1, <1.0a1) should contain 1.0.dev2")
	}
	if i.Contains("1.0a1") {
		t.Errorf("Intersect(>=1.0.dev1, <1.0a1) should not contain 1.0a1")
	}

	// Union of two intervals whose bounds only order correctly under PEP 440
	// should merge into one.
	c, _ := Parse("vers:pypi/>=1.0.dev1|<1.0a1")
	d, _ := Parse("vers:pypi/>=1.0.dev5|<1.0b1")
	u := c.Union(d)
	if !u.Contains("1.0.dev2") || !u.Contains("1.0a5") || u.Contains("1.0b1") {
		t.Errorf("Union of overlapping pypi intervals gave wrong containment: %v", u.Intervals)
	}

	// Intersect where both sides have a lower bound and PEP 440 disagrees
	// with generic ordering on which is higher.
	e, _ := Parse("vers:pypi/>=1.0.dev1|<2.0")
	f, _ := Parse("vers:pypi/>=1.0a1|<2.0")
	ef := e.Intersect(f)
	if ef.Contains("1.0.dev5") {
		t.Errorf("Intersect should have raised lower bound to 1.0a1")
	}
	if !ef.Contains("1.0a1") {
		t.Errorf("Intersect should contain 1.0a1")
	}
}

func TestPyPICompareLargeNumbers(t *testing.T) {
	// Numeric components can exceed any fixed sentinel; presence of dev
	// must still sort before absence regardless of magnitude.
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0a1.dev2000000000", "1.0a1", -1},
		{"1.0.dev2000000000", "1.0a1", -1},
		{"1.0", "1.0.post2000000000", -1},
		{"1.0a2000000000", "1.0", -1},
		// Values that overflow int must still order distinctly.
		{"1.0a888888888888888888888888", "1.0a999999999999999999999999", -1},
		{"1.0.dev888888888888888888888888", "1.0.dev999999999999999999999999", -1},
		{"1.0.post888888888888888888888888", "1.0.post999999999999999999999999", -1},
		{"888888888888888888888888!1.0", "999999999999999999999999!1.0", -1},
		{"1.888888888888888888888888", "1.999999999999999999999999", -1},
		// Large local numeric segments stay numeric.
		{"1.0+888888888888888888888888", "1.0+999999999999999999999999", -1},
		{"1.0+abc", "1.0+888888888888888888888888", -1},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.a, tt.b, "pypi"); got != tt.want {
			t.Errorf("CompareWithScheme(%q, %q, pypi) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPyPICompatibleRelease(t *testing.T) {
	// ~= counts release segments, not dots; pre/post/dev suffixes and
	// epochs must not affect the derived upper bound.
	tests := []struct {
		constraint string
		version    string
		want       bool
	}{
		// ~=1.4.dev1 -> >=1.4.dev1, <2 (release is 1.4, two segments)
		{"~=1.4.dev1", "1.4.dev1", true},
		{"~=1.4.dev1", "1.9", true},
		{"~=1.4.dev1", "2.0", false},
		{"~=1.4.dev1", "1.3", false},
		// ~=1.4.5 -> >=1.4.5, <1.5 (three release segments)
		{"~=1.4.5", "1.4.6", true},
		{"~=1.4.5", "1.5", false},
		// ~=1.4.5.post1 -> >=1.4.5.post1, <1.5 (still three release segments)
		{"~=1.4.5.post1", "1.4.9", true},
		{"~=1.4.5.post1", "1.5", false},
		// trailing-zero release segments still count
		{"~=2.0.dev1", "2.5", true},
		{"~=2.0.dev1", "3.0", false},
		{"~=2.0.0", "2.0.9", true},
		{"~=2.0.0", "2.1", false},
		// epoch preserved in the upper bound
		{"~=1!1.4.5", "1!1.4.6", true},
		{"~=1!1.4.5", "1!1.5", false},
		{"~=1!1.4.5", "1.4.6", false},
		// ~= composed with other comma-separated specifiers (comma is AND)
		{"~=1.4.2,!=1.4.5", "1.4.5", false},
		{"~=1.4.2,!=1.4.5", "1.4.6", true},
		{"~=1.4.2,!=1.4.5", "1.5", false},
		{">=1.0,~=1.4", "2.0", false},
		{">=1.0,~=1.4", "1.4", true},
		{">=1.0,~=1.4", "1.9", true},
		{"~=1.4, !=1.4.5, <1.4.8", "1.4.6", true},
		{"~=1.4, !=1.4.5, <1.4.8", "1.4.5", false},
		{"~=1.4, !=1.4.5, <1.4.8", "1.4.9", false},
	}
	for _, tt := range tests {
		got, err := Satisfies(tt.version, tt.constraint, "pypi")
		if err != nil {
			t.Errorf("Satisfies(%q, %q, pypi) error = %v", tt.version, tt.constraint, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Satisfies(%q, %q, pypi) = %v, want %v", tt.version, tt.constraint, got, tt.want)
		}
	}
}

func TestPyPIUnionExclusionEquality(t *testing.T) {
	a, _ := Parse("vers:pypi/>=1.0|!=1.5|<2.0")
	b, _ := Parse("vers:pypi/>=1.0|!=1.5.0|<2.0")
	u := a.Union(b)
	if u.Contains("1.5") {
		t.Errorf("union should still exclude 1.5 (both inputs exclude a version equal to it)")
	}
	if u.Contains("1.5.0") {
		t.Errorf("union should still exclude 1.5.0")
	}
	if !u.Contains("1.6") {
		t.Errorf("union should contain 1.6")
	}
}

func TestPyPINativeRangeContains(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		want       bool
	}{
		{">=5.1,<5.2b1", "5.1.5", true},
		{">=5.1,<5.2b1", "6.0.3", false},
		{">=1.0,<2.0", "2.0a1", true},
		{">=1.0,<2.0", "2.0", false},
	}
	for _, tt := range tests {
		r, err := ParseNative(tt.constraint, "pypi")
		if err != nil {
			t.Errorf("ParseNative(%q, pypi) error = %v", tt.constraint, err)
			continue
		}
		if got := r.Contains(tt.version); got != tt.want {
			t.Errorf("ParseNative(%q, pypi).Contains(%q) = %v, want %v", tt.constraint, tt.version, got, tt.want)
		}
	}
}
