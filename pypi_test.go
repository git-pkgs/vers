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
