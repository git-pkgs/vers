package vers

import (
	"strings"
	"testing"
)

func BenchmarkPackageVersionValidation(b *testing.B) {
	for _, scheme := range []string{"pypi", "composer"} {
		for _, version := range []string{"1.2.3", "1.2.3rc1", "v1.2.3", "dev-main", "1!2.0.post1+local"} {
			b.Run(scheme+"/"+version, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					ValidWithScheme(version, scheme)
				}
			})
		}
	}
}

func FuzzPackageVersionValidation(f *testing.F) {
	for _, s := range []string{"1.2.3", "1.2.3rc1", "1!2.0.post1+local", "v1.2.3", "dev-main", "1.2.x-dev", "1.2.3-unknown", "1.2.3-ſtable", "1.2.3-ALPHA1", "1.2.3-patch1", "1.2.3+foo", "\t1.2.3\n", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v := strings.TrimSpace(s)
		_, py := parsePEP440(v)
		_, co := parseComposerVersion(v)
		co = v != "" && !strings.ContainsAny(v, " \t\r\n") && (co || isComposerBranchVersion(v) || composerNumericBranchRegex.MatchString(v))
		for scheme, want := range map[string]bool{"pypi": py, "composer": co} {
			if got := ValidWithScheme(s, scheme); got != want {
				t.Fatalf("%s %q: got %v want %v", scheme, s, got, want)
			}
		}
	})
}

func TestPackageVersionValidation(t *testing.T) {
	for _, tc := range []struct {
		scheme, version string
		want            bool
	}{
		{"pypi", "1!2.0rc1.post2.dev3+local.4", true},
		{"pypi", "1.2.3junk", false},
		{"composer", "v1.2.3-ALPHA1", true},
		{"composer", "1.2.3-patch1", true},
		{"composer", "1.2.3-unknown", false},
		{"composer", "1.2.3-ſtable", false},
		{"composer", "dev-main", true},
		{"composer", "1.2.x-dev", true},
		{"composer", "1.2.3+build", true},
		{"composer", "1.2.3-DEV1", true},
	} {
		if got := ValidWithScheme(tc.version, tc.scheme); got != tc.want {
			t.Errorf("%s %q: got %v want %v", tc.scheme, tc.version, got, tc.want)
		}
	}
}
