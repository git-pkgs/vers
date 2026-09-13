package vers

import (
	"strings"
	"testing"
)

func TestSemverValidation(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"1", true}, {"1.2", true}, {"1.2.3", true},
		{"v01.002.0003", true}, {"1.2.3-alpha.01+build.5", true},
		{"1.2.3--+0", true}, {"\t1.2.3\r\n", true},
		{"", false}, {"v", false}, {"V1.2.3", false},
		{"1.2.3.4", false}, {"1.", false}, {"1..2", false},
		{"1.2.3-", false}, {"1.2.3+", false},
		{"1.2.3-alpha..1", false}, {"1.2.3+build..1", false},
		{"1.2.3-alpha_1", false}, {"1.2.3+é", false},
		{"1.2.3-alpha\n1", false}, {"1.2.3+a+b", false},
		{"^1.2.3", false}, {"1.2.x", false},
	}
	for _, scheme := range []string{"npm", "semver", "cargo", "go", "golang", "hex", "elixir"} {
		t.Run(scheme, func(t *testing.T) {
			for _, tt := range tests {
				if got := ValidWithScheme(tt.version, scheme); got != tt.valid {
					t.Errorf("ValidWithScheme(%q, %q) = %v, want %v", tt.version, scheme, got, tt.valid)
				}
			}
		})
	}
}

func FuzzSemverValidation(f *testing.F) {
	for _, version := range []string{"1.2.3", "v01.2", "1.2.3-alpha.01+build.5", "1.2.3-", "1.2.3+", "1.2.3-a\nb", "1.2.3+a+b", "", "1.2.3.4"} {
		f.Add(version)
	}
	f.Fuzz(func(t *testing.T, version string) {
		want := referenceSemverValidation(strings.TrimSpace(version))
		if got := ValidWithScheme(version, "npm"); got != want {
			t.Fatalf("ValidWithScheme(%q, npm) = %v, want %v", version, got, want)
		}
	})
}

// Preserve the regex implementation as an independent compatibility oracle.
func referenceSemverValidation(version string) bool {
	m := SemanticVersionRegex.FindStringSubmatch(version)
	if m == nil {
		return false
	}
	for _, field := range []string{m[4], m[5]} {
		if field == "" {
			continue
		}
		for _, part := range strings.Split(field, ".") {
			if part == "" || strings.Trim(part, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-") != "" {
				return false
			}
		}
	}
	return true
}

func BenchmarkValidWithSchemeNPM(b *testing.B) {
	for _, version := range []string{"1.2.3", "v01.2", "1.2.3-alpha.1+build.5", "1.2.3-alpha..1"} {
		b.Run(version, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ValidWithScheme(version, "npm")
			}
		})
	}
}
