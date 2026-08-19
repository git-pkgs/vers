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

func TestRangeExtractors(t *testing.T) {
	tests := []struct {
		name    string
		extract func(map[string]string) ([]nativeRangeAssertion, error)
		files   map[string]string
		want    []nativeRangeAssertion
	}{
		{
			name: "node-semver", extract: extractNodeSemverRanges,
			files: map[string]string{
				"test/fixtures/range-include.js": "['^1.0.0', '1.2.0']\n['<\\t2.0.0', '0.2.9']\n['*', '1.0.0-a', { includePrerelease: true }]\n",
				"test/fixtures/range-exclude.js": "['^1.0.0', '2.0.0']\n",
			},
			want: []nativeRangeAssertion{
				{nativeRange: "^1.0.0", version: "1.2.0", contains: true},
				{nativeRange: "<\t2.0.0", version: "0.2.9", contains: true},
				{nativeRange: "^1.0.0", version: "2.0.0", contains: false},
			},
		},
		{
			name: "pypi", extract: extractPyPIRanges,
			files: map[string]string{"tests/test_specifiers.py": `
("version", "spec_str", "expected")
[(v, s, True) for v, s in [("1.2", ">=1")]]
+ [(v, s, False) for v, s in [("0.9", ">=1")]]
def test_specifiers(self):
`},
			want: []nativeRangeAssertion{
				{nativeRange: ">=1", version: "1.2", contains: true},
				{nativeRange: ">=1", version: "0.9", contains: false},
			},
		},
		{
			name: "rubygems", extract: extractRubyGemsRanges,
			files: map[string]string{"test/rubygems/test_gem_requirement.rb": `
assert_satisfied_by "1.5", "~> 1.4"
refute_satisfied_by "2.0", "~> 1.4"
`},
			want: []nativeRangeAssertion{
				{nativeRange: "~> 1.4", version: "1.5", contains: true},
				{nativeRange: "~> 1.4", version: "2.0", contains: false},
			},
		},
		{
			name: "composer", extract: extractComposerRanges,
			files: map[string]string{"tests/VersionParserTest.php": `
function simpleConstraints()
    {
        array('>=1.2', new Constraint('>=', '1.2.0.0-dev')),
        array('<2', new Constraint('<', '2.0.0.0-dev')),
		array('>=dev-master', new Constraint('>=', 'dev-master')),
    }
`},
			want: []nativeRangeAssertion{
				{nativeRange: ">=1.2", version: "1.2.0.0-dev", contains: true},
				{nativeRange: "<2", version: "2.0.0.0-dev", contains: false},
				{nativeRange: ">=dev-master", version: "dev-master", contains: false},
			},
		},
		{
			name: "pub", extract: extractPubRanges,
			files: map[string]string{
				"pkgs/pub_semver/test/version_constraint_test.dart": `
var constraint = VersionConstraint.parse('>=1.2.3');
expect(constraint, allows(Version.parse('1.2.3'), Version.parse('2.0.0')));
expect(constraint, doesNotAllow(Version.parse('1.2.2')));
expect(constraint.allows(Version.parse('3.0.0')), isFalse);
    test('next', () {
expect(constraint, allows(Version.parse('4.0.0')));
				`,
				"pkgs/pub_semver/test/version_range_test.dart": `
var range = VersionRange(min: v123, includeMin: true,
    max: Version.parse('2.0.0'));
expect(range, allows(Version.parse('1.2.3')));
expect(range, doesNotAllow(Version.parse('2.0.0')));
`,
				"pkgs/pub_semver/test/utils.dart": "final v123 = Version.parse('1.2.3');\n",
			},
			want: []nativeRangeAssertion{
				{nativeRange: ">=1.2.3", version: "1.2.3", contains: true},
				{nativeRange: ">=1.2.3", version: "2.0.0", contains: true},
				{nativeRange: ">=1.2.3", version: "1.2.2", contains: false},
				{nativeRange: ">=1.2.3 <2.0.0", version: "1.2.3", contains: true},
				{nativeRange: ">=1.2.3 <2.0.0", version: "2.0.0", contains: false},
			},
		},
		{
			name: "cargo", extract: extractCargoRanges,
			files: map[string]string{"tests/test_version_req.rs": `
let ref r = req("^1.0.0");
assert_match_all(r, &["1.0.0", "1.2.0"]);
assert_match_none(r, &["2.0.0"]);
}
`},
			want: []nativeRangeAssertion{
				{nativeRange: "^1.0.0", version: "1.0.0", contains: true},
				{nativeRange: "^1.0.0", version: "1.2.0", contains: true},
				{nativeRange: "^1.0.0", version: "2.0.0", contains: false},
			},
		},
		{
			name: "maven", extract: extractMavenRanges,
			files: map[string]string{"compat/maven-artifact/src/test/java/org/apache/maven/artifact/versioning/VersionRangeTest.java": `
VersionRange range = VersionRange.createFromVersionSpec("[1.0,2.0)");
assertTrue(range.containsVersion(new DefaultArtifactVersion("1.5")));
assertFalse(range.containsVersion(new DefaultArtifactVersion("2.0")));
`},
			want: []nativeRangeAssertion{
				{nativeRange: "[1.0,2.0)", version: "1.5", contains: true},
				{nativeRange: "[1.0,2.0)", version: "2.0", contains: false},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.extract(test.files)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("range assertions = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestBuildRangeTestFileNormalizesAndDeduplicates(t *testing.T) {
	source := sourceSpec{name: "reference", scheme: "npm"}
	file, err := buildRangeTestFile(source, []nativeRangeAssertion{
		{nativeRange: "^1.0.0", version: "1.2.0", contains: true},
		{nativeRange: "^1.0.0", version: "1.2.0", contains: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Tests) != 1 {
		t.Fatalf("got %d tests, want 1", len(file.Tests))
	}
	if got := file.Tests[0].Input.Vers; got != "vers:npm/>=1.0.0|<2.0.0" {
		t.Errorf("VERS input = %q", got)
	}
}

func TestBuildRangeTestFileRepresentsEmptyRange(t *testing.T) {
	source := sourceSpec{name: "reference", scheme: "cargo"}
	file, err := buildRangeTestFile(source, []nativeRangeAssertion{
		{nativeRange: "0.3.0, 0.4.0", version: "0.3.0", contains: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := file.Tests[0].Input.Vers; got == "vers:cargo/" {
		t.Errorf("empty native range encoded as unbounded VERS %q", got)
	}
}

func TestBuildRangeTestFileRejectsConflictingConversions(t *testing.T) {
	source := sourceSpec{name: "reference", scheme: "npm"}
	_, err := buildRangeTestFile(source, []nativeRangeAssertion{
		{nativeRange: "*", version: "1.0.0", contains: true},
		{nativeRange: "x", version: "1.0.0", contains: false},
	})
	if err == nil {
		t.Fatal("buildRangeTestFile accepted conflicting results for the same VERS input")
	}
}

func TestExtractGoReportsMissingTable(t *testing.T) {
	_, err := extractGo(map[string]string{"semver/semver_test.go": "package semver\n"})
	if err == nil {
		t.Fatal("extractGo succeeded without a tests table")
	}
}
