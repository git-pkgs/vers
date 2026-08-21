package vers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// parityCorpus is a deliberately awkward set of inputs for the ALPM, Conan and
// Gentoo comparators. It covers the separators each scheme treats specially,
// the empty and separator-only strings that decide how many components a
// version has, leading and trailing zeros, and the letter and suffix forms that
// drive Gentoo ordering.
func parityCorpus() []string {
	return []string{
		"", "0", "1", "00", "01", "0.0", "1.0", "1.0.0", "1.2", "1.2.3", "1.2.3.4",
		".", "..", "1.", ".1", "1..2", "1.0.", "0.0.0",
		"1.2.3a", "1.2.3z", "1.2.3A", "a", "a.b", "1a.2b", "abc", "ABC",
		"-", "1-", "-1", "1-2", "1.2-3", "1.2.3-1", "1.2.3-2", "1.2.3-10",
		"+", "1+", "+1", "1+2", "1.0+build", "1.0-alpha+build", "1.0+build-alpha",
		"1.0-alpha", "1.0-alpha.1", "1.0-alpha-beta", "1.0-beta", "1.0-rc1", "1.0-rc.1",
		"_", "1_", "_1", "1.2.3_", "1.2.3_p", "1.2.3_p1", "1.2.3_pre", "1.2.3_pre1",
		"1.2.3_alpha", "1.2.3_alpha1", "1.2.3_beta", "1.2.3_beta2", "1.2.3_rc", "1.2.3_rc1",
		"1.2.3_alpha_p1", "1.2.3_rc1_p2", "1.2.3_p1_alpha", "1.2.3__p",
		"1.2.3-r0", "1.2.3-r1", "1.2.3-r10", "1.2.3-r", "1.2.3-rx", "1.2.3_p1-r2",
		"1.2.3a-r1", "1.2.3a_p1", "0.1.0_alpha", "2.34",
		":", "1:1.0", "2:1.0", "0:1.0", ":1.0", "1:", "1:2:3",
		"1:1.2.3-1", "1:1.2.3-2", "2:1.2.3-1",
		"1.0.0.0.0", "10.0", "9.0", "1.10", "1.9",
		"20240101", "2024.01.01", "1.2.3.post1", "1.2.3rc1",
		"~", "1~2", "1.2.3~rc1", "$", "1.2.3$", "1.2.3+-_",
	}
}

// fixtureVersions harvests every version string the local conformance fixtures
// use for the schemes this file covers, so the parity check runs against real
// upstream data as well as the synthetic corpus.
func fixtureVersions(t *testing.T) []string {
	t.Helper()

	files := []string{
		"alpm_univers_version_cmp_test.json",
		"conan_univers_version_cmp_test.json",
		"gentoo_univers_version_cmp_test.json",
		"apk_univers_version_cmp_test.json",
	}

	seen := make(map[string]bool)
	var versions []string
	for _, name := range files {
		path := filepath.Join("testdata", "local", "tests", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Logf("skipping %s: %v", name, err)
			continue
		}
		// Only input.versions is decoded. expected_output is typed per test
		// kind, a bool for containment cases and a list for ordering ones, and
		// it never carries a version the input did not already list.
		var doc struct {
			Tests []struct {
				Input struct {
					Versions []string `json:"versions"`
				} `json:"input"`
			} `json:"tests"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, test := range doc.Tests {
			for _, v := range test.Input.Versions {
				if !seen[v] {
					seen[v] = true
					versions = append(versions, v)
				}
			}
		}
	}
	if len(versions) == 0 {
		t.Fatal("no fixture versions harvested; the testdata submodule may be missing")
	}
	return versions
}

// assertParity checks that the rewritten comparator returns exactly what the
// pre-optimization reference returns for every ordered pair, including a value
// against itself.
func assertParity(t *testing.T, name string, versions []string, got, want func(a, b string) int) {
	t.Helper()
	for _, a := range versions {
		for _, b := range versions {
			if g, w := got(a, b), want(a, b); g != w {
				t.Fatalf("%s(%q, %q) = %d, reference implementation returns %d", name, a, b, g, w)
			}
		}
	}
}

func TestCompareALPMMatchesReference(t *testing.T) {
	assertParity(t, "compareALPM", parityCorpus(), compareALPM, refCompareALPM)
}

func TestCompareConanMatchesReference(t *testing.T) {
	assertParity(t, "compareConan", parityCorpus(), compareConan, refCompareConan)
}

func TestCompareGentooMatchesReference(t *testing.T) {
	assertParity(t, "compareGentoo", parityCorpus(), compareGentoo, refCompareGentoo)
}

func TestEcosystemComparatorsMatchReferenceOnFixtures(t *testing.T) {
	versions := fixtureVersions(t)
	assertParity(t, "compareALPM", versions, compareALPM, refCompareALPM)
	assertParity(t, "compareConan", versions, compareConan, refCompareConan)
	assertParity(t, "compareGentoo", versions, compareGentoo, refCompareGentoo)
}

// TestEcosystemComparatorsDoNotAllocate is the point of the rewrite: these
// comparators must run entirely out of stack storage.
func TestEcosystemComparatorsDoNotAllocate(t *testing.T) {
	tests := []struct {
		name    string
		compare func(a, b string) int
		a, b    string
	}{
		{name: "ALPM", compare: compareALPM, a: "1:1.2.3-1", b: "1:1.2.4-1"},
		{name: "ALPM equal prefix", compare: compareALPM, a: "1.2.3alpha1-1", b: "1.2.3beta1-1"},
		{name: "Conan", compare: compareConan, a: "1.2.3-rc1", b: "1.2.3"},
		{name: "Conan build", compare: compareConan, a: "1.2.0-alpha+build1", b: "1.2-alpha+build2"},
		{name: "Gentoo", compare: compareGentoo, a: "1.2.3_rc1", b: "1.2.3"},
		{name: "Gentoo revision", compare: compareGentoo, a: "1.2.3a_p1-r2", b: "1.2.3a_p2-r1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testing.AllocsPerRun(100, func() {
				tt.compare(tt.a, tt.b)
			})
			if got != 0 {
				t.Errorf("%s comparison allocated %v times per run, want 0", tt.name, got)
			}
		})
	}
}
