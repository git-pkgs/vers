package vers

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input   string
		major   int
		minor   int
		patch   int
		prerel  string
		wantErr bool
	}{
		{"1", 1, 0, 0, "", false},
		{"1.2", 1, 2, 0, "", false},
		{"1.2.3", 1, 2, 3, "", false},
		{"1.2.3-alpha", 1, 2, 3, "alpha", false},
		{"1.2.3-alpha.1", 1, 2, 3, "alpha.1", false},
		{"1.2.3-beta.2+build.123", 1, 2, 3, "beta.2", false},
		{"0.0.1", 0, 0, 1, "", false},
		{"10.20.30", 10, 20, 30, "", false},
		{"", 0, 0, 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if v.Major != tt.major {
				t.Errorf("Major = %d, want %d", v.Major, tt.major)
			}
			if v.Minor != tt.minor {
				t.Errorf("Minor = %d, want %d", v.Minor, tt.minor)
			}
			if v.Patch != tt.patch {
				t.Errorf("Patch = %d, want %d", v.Patch, tt.patch)
			}
			if v.Prerelease != tt.prerel {
				t.Errorf("Prerelease = %q, want %q", v.Prerelease, tt.prerel)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// Equal versions
		{"1.0.0", "1.0.0", 0},
		{"1.2.3", "1.2.3", 0},

		// Major version differences
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},

		// Minor version differences
		{"1.1.0", "1.2.0", -1},
		{"1.2.0", "1.1.0", 1},

		// Patch version differences
		{"1.0.1", "1.0.2", -1},
		{"1.0.2", "1.0.1", 1},

		// Prerelease vs stable (stable > prerelease)
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0", -1},

		// Prerelease comparison
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-beta", "1.0.0-alpha", 1},
		{"1.0.0-alpha.1", "1.0.0-alpha.2", -1},

		// Different lengths
		{"1", "1.0.0", 0},
		{"1.2", "1.2.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.2.3", "1.2.3"},
		{"1.2.3-alpha", "1.2.3-alpha"},
		{"1", "1.0.0"},
		{"1.2", "1.2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			if err != nil {
				t.Fatalf("ParseVersion(%q) error = %v", tt.input, err)
			}
			got := v.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionIncrement(t *testing.T) {
	v, _ := ParseVersion("1.2.3")

	major := v.IncrementMajor()
	if major.String() != "2.0.0" {
		t.Errorf("IncrementMajor() = %q, want %q", major.String(), "2.0.0")
	}

	minor := v.IncrementMinor()
	if minor.String() != "1.3.0" {
		t.Errorf("IncrementMinor() = %q, want %q", minor.String(), "1.3.0")
	}

	patch := v.IncrementPatch()
	if patch.String() != "1.2.4" {
		t.Errorf("IncrementPatch() = %q, want %q", patch.String(), "1.2.4")
	}
}

func TestVersionIsStable(t *testing.T) {
	stable, _ := ParseVersion("1.2.3")
	if !stable.IsStable() {
		t.Error("1.2.3 should be stable")
	}

	prerelease, _ := ParseVersion("1.2.3-alpha")
	if prerelease.IsStable() {
		t.Error("1.2.3-alpha should not be stable")
	}
}
