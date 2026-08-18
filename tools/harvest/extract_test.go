package main

import (
	"reflect"
	"testing"
)

func TestExtractors(t *testing.T) {
	tests := []struct {
		name    string
		extract func(map[string]string) ([]comparison, error)
		files   map[string]string
		want    []comparison
	}{
		{
			name: "node-semver", extract: extractNodeSemver,
			files: map[string]string{
				"test/fixtures/comparisons.js": "['2.0.0', '1.0.0']\n['v2.0.0', '1.0.0', true]\n",
				"test/fixtures/equality.js":    "['1.0.0+one', '1.0.0+two']\n",
			},
			want: []comparison{{left: "2.0.0", right: "1.0.0", result: 1}, {left: "1.0.0+one", right: "1.0.0+two"}},
		},
		{
			name: "pypi", extract: extractPyPI,
			files: map[string]string{"tests/test_version.py": "VERSIONS = [\n  \"1.0.dev0\",\n  \"1.0\",\n]\nassert Version(\"1.0rc1\") == Version(\"1.0c1\")\n"},
			want:  []comparison{{left: "1.0.dev0", right: "1.0", result: -1}, {left: "1.0rc1", right: "1.0c1"}},
		},
		{
			name: "rubygems", extract: extractRubyGems,
			files: map[string]string{"test/rubygems/test_gem_version.rb": "assert_equal(-1, v(\"1.a\") <=> v(\"1\"))\nassert_less_than v(\"2.a\"), v(\"2\")\n"},
			want:  []comparison{{left: "1.a", right: "1", result: -1}, {left: "2.a", right: "2", result: -1}},
		},
		{
			name: "composer", extract: extractComposer,
			files: map[string]string{"tests/ComparatorTest.php": "function compareProvider() { array('2', '>', '1', true); array('1', '>=', '1', true); }\nfunction equalToProvider()\n    {\narray('dev-a', '1', false)\n    }"},
			want:  []comparison{{left: "2", right: "1", result: 1}, {left: "dev-a", right: "1", differentOnly: true}},
		},
		{
			name: "pub", extract: extractPub,
			files: map[string]string{"pkgs/pub_semver/test/version_test.dart": "group('comparison', () { var versions = ['1.0.0-a', '1.0.0']; test('equality', () { expect(Version.parse('01.0.0'), equals(Version.parse('1.0.0'))); expect(Version.parse('1.0.0-a'), isNot(equals(Version.parse('1.0.0-b')))); }); });"},
			want:  []comparison{{left: "1.0.0-a", right: "1.0.0", result: -1}, {left: "01.0.0", right: "1.0.0"}, {left: "1.0.0-a", right: "1.0.0-b", differentOnly: true}},
		},
		{
			name: "cargo", extract: extractCargo,
			files: map[string]string{"tests/test_version.rs": "fn test_spec_order() { let vs = [\"1.0.0-a\", \"1.0.0\"]; }\nassert!(version(\"1.0.0+1\") < version(\"1.0.0+2\"));\nassert_ne!(version(\"1.0.0+1\"), version(\"1.0.0+2\"));"},
			want:  []comparison{{left: "1.0.0-a", right: "1.0.0", result: -1}, {left: "1.0.0+1", right: "1.0.0+2", result: -1}, {left: "1.0.0+1", right: "1.0.0+2", differentOnly: true}},
		},
		{
			name: "dpkg", extract: extractDpkg,
			files: map[string]string{"lib/dpkg/t/t-version.c": "a = DPKG_VERSION_OBJECT(0, \"1\", \"1\"); b = DPKG_VERSION_OBJECT(1, \"1\", \"1\"); test_pass(dpkg_version_compare(&a, &b) < 0);"},
			want:  []comparison{{left: "1-1", right: "1:1-1", result: -1}},
		},
		{
			name: "rpm", extract: extractRPM,
			files: map[string]string{"tests/rpmvercmp.at": "RPMVERCMP(1.0~rc1, 1.0, -1)\ndnl RPMVERCMP(2, 1, 1)\n"},
			want:  []comparison{{left: "1.0~rc1", right: "1.0", result: -1}},
		},
		{
			name: "go", extract: extractGo,
			files: map[string]string{"semver/semver_test.go": "var tests = []struct { in string; out string }{\n{\"bad\", \"\"},\n{\"also-bad\", \"\"},\n{\"v1\", \"v1.0.0\"},\n}\n"},
			want:  []comparison{{left: "bad", right: "also-bad"}, {left: "also-bad", right: "v1", result: -1}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.extract(test.files)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("comparisons = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestBuildTestFileNormalizesResultsAndDeduplicates(t *testing.T) {
	source := sourceSpec{name: "reference", scheme: "example"}
	file := buildTestFile(source, []comparison{
		{left: "2", right: "1", result: 1},
		{left: "1", right: "2", result: -1},
		{left: "1.0", right: "1", result: 0},
	})

	if len(file.Tests) != 3 {
		t.Fatalf("got %d tests, want 3", len(file.Tests))
	}
	if got := file.Tests[0].Input.Versions; !reflect.DeepEqual(got, []string{"2", "1"}) {
		t.Errorf("comparison input = %v", got)
	}
	if got := file.Tests[2].ExpectedOutput; got != true {
		t.Errorf("equality output = %v", got)
	}
}

func TestExtractGoReportsMissingTable(t *testing.T) {
	_, err := extractGo(map[string]string{"semver/semver_test.go": "package semver\n"})
	if err == nil {
		t.Fatal("extractGo succeeded without a tests table")
	}
}
