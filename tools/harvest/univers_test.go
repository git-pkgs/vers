package main

import (
	"reflect"
	"testing"
)

func TestUniversComparisonExtractors(t *testing.T) {
	tests := []struct {
		name    string
		extract func(map[string]string) ([]comparison, error)
		files   map[string]string
		want    []comparison
	}{
		{
			name: "debian", extract: extractUniversDebian,
			files: map[string]string{"tests/test_debian_version.py": `
assert compare_versions("1.0", "1.0-0") == 0
assert compare_versions("1.0", "2.0") == -1
`},
			want: []comparison{{left: "1.0", right: "1.0-0"}, {left: "1.0", right: "2.0", result: -1}},
		},
		{
			name: "rpm", extract: extractUniversRPM,
			files: map[string]string{"tests/data/rpmvercmp.at": `
RPMVERCMP(1.0, 2.0, -1)
dnl RPMVERCMP(1a, 1b, -1)
`},
			want: []comparison{{left: "1.0", right: "2.0", result: -1}, {left: "1a", right: "1b", result: -1}},
		},
		{
			name: "gentoo", extract: extractUniversGentoo,
			files: map[string]string{"tests/test_gentoo_pkgcore.py": `
assert older("1_alpha", "1")
assert newer("2", "1")
assert equal("1-r0", "1")
assert not equal("1_p1", "1")
`},
			want: []comparison{
				{left: "1_alpha", right: "1", result: -1},
				{left: "2", right: "1", result: 1},
				{left: "1-r0", right: "1"},
				{left: "1_p1", right: "1", differentOnly: true},
			},
		},
		{
			name: "alpine", extract: extractUniversAlpine,
			files: map[string]string{"tests/data/alpine_test.txt": "1.0 < 2.0\n2.0 = 2.0\n"},
			want:  []comparison{{left: "1.0", right: "2.0", result: -1}, {left: "2.0", right: "2.0"}},
		},
		{
			name: "pacman", extract: extractUniversPacman,
			files: map[string]string{"tests/test_pacman_vercmp.py": `
assert ArchLinuxVersion("1.0") < ArchLinuxVersion("2.0")
assert ArchLinuxVersion("1.0") == ArchLinuxVersion("1.0-1")
# assert ArchLinuxVersion("2___a") == ArchLinuxVersion("2_a")
`},
			want: []comparison{{left: "1.0", right: "2.0", result: -1}, {left: "1.0", right: "1.0-1"}},
		},
		{
			name: "conan", extract: extractUniversConan,
			files: map[string]string{"tests/test_conan_version_comparison.py": `
v = [
    # Ordered pairs.
    ("1", "2"),
]
e = [
    ("1", "1.0"),
]
`},
			want: []comparison{{left: "1", right: "2", result: -1}, {left: "1", right: "1.0"}},
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

func TestExtractUniversConanRanges(t *testing.T) {
	files := map[string]string{"tests/test_conan_version_range.py": `
values = [
    ["", [], ["1.0"], ["1.0-pre"]],
    ["*, include_prerelease=True", [], ["1.0-pre"], []],
    # A supported range spread over several lines.
    [
        "^1.2",
        [[[]]],
        ["1.2.1", "1.9"],
        ["2.0"],
    ],
]
`}
	got, err := extractUniversConanRanges(files)
	if err != nil {
		t.Fatal(err)
	}
	want := []nativeRangeAssertion{
		{nativeRange: "^1.2", version: "1.2.1", contains: true},
		{nativeRange: "^1.2", version: "1.9", contains: true},
		{nativeRange: "^1.2", version: "2.0", contains: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("assertions = %#v, want %#v", got, want)
	}
}

func TestSplitPythonListHandlesCommentsAndEmptyStrings(t *testing.T) {
	got, err := splitPythonList(`[
        # first item
        ["", ["1"]],
        ["two", []],
    ]`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`["", ["1"]]`, `["two", []]`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("items = %#v, want %#v", got, want)
	}
}
