package vers

import "testing"

func TestBazelComparisonThroughPublicAPI(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{left: "0.7.1", right: "0.7.1.bcr.1", want: -1},
		{left: "0.7.1.bcr.2", right: "0.7.1.bcr.10", want: -1},
		{left: "36.0-rc2", right: "36.0", want: -1},
		{left: "36.0", right: "36.0.bcr.1", want: -1},
		{left: "2.0", right: "1.0", want: 1},
		{left: "2.0", right: "1.9", want: 1},
		{left: "11.0", right: "3.0", want: 1},
		{left: "1.0.1", right: "1.0", want: 1},
		{left: "1.0.0", right: "1.0", want: 1},
		{left: "1.0", right: "1.0-pre", want: 1},
		{left: "1.0.patch.3", right: "1.0", want: 1},
		{left: "1.0.patch.3", right: "1.0.patch.2", want: 1},
		{left: "1.0.patch.3", right: "1.0.patch.10", want: -1},
		{left: "1.0.patch3", right: "1.0.patch10", want: 1},
		{left: "4", right: "a", want: -1},
		{left: "abc", right: "abd", want: -1},
		{left: "1.0-pre", right: "1.0-are", want: 1},
		{left: "1.0-3", right: "1.0-2", want: 1},
		{left: "1.0-pre", right: "1.0-pre.foo", want: -1},
		{left: "1.0-pre.3", right: "1.0-pre.2", want: 1},
		{left: "1.0-pre.10", right: "1.0-pre.2", want: 1},
		{left: "1.0-pre.10a", right: "1.0-pre.2a", want: -1},
		{left: "1.0-pre.99", right: "1.0-pre.2a", want: -1},
		{left: "1.0-pre.patch.3", right: "1.0-pre.patch.4", want: -1},
		{left: "1.0--", right: "1.0----", want: -1},
		{left: "2.1.1-develop.bcr.20250113215904", right: "2.1.1-develop.bcr.20250113215903", want: 1},
		{left: "1.0+build2", right: "1.0+build3", want: 0},
		{left: "1.0", right: "1.0+build-notpre", want: 0},
		{left: "01", right: "1", want: -1},
		{left: "", right: "1.0", want: 1},
		{left: "", right: "1.0-pre+build-kek.lol", want: 1},
	}

	for _, test := range tests {
		if got := CompareWithScheme(test.left, test.right, "bazel"); got != test.want {
			t.Errorf("CompareWithScheme(%q, %q, bazel) = %d, want %d", test.left, test.right, got, test.want)
		}
	}

	if got := Compare("0.7.1", "0.7.1.bcr.1"); got <= 0 {
		t.Errorf("generic Compare() = %d, want existing generic ordering", got)
	}
}

func TestBazelValidationAndNormalizationThroughPublicAPI(t *testing.T) {
	valid := []string{
		"35.1",
		"0.7.1.bcr.1",
		"20210324.2",
		"1.0.patch.3",
		"1.0--",
		"1.0-pre+build-kek.lol",
		"01",
		"v1.0",
		"18446744073709551615",
	}
	for _, version := range valid {
		if !ValidWithScheme(version, "bazel") {
			t.Errorf("ValidWithScheme(%q, bazel) = false", version)
		}
	}

	invalid := []string{
		"",
		"-abc",
		"1_2",
		"ßážëł",
		"1.0-pre?",
		"18446744073709551616",
		"1.0-18446744073709551616",
		"1.0-pre///",
		"1..0",
		"1.0-pre..erp",
		"1.0+build..metadata",
		" 1.0",
	}
	for _, version := range invalid {
		if ValidWithScheme(version, "bazel") {
			t.Errorf("ValidWithScheme(%q, bazel) = true", version)
		}
	}

	normalizations := map[string]string{
		"35.1":                "35.1",
		"0.7.1.bcr.1":         "0.7.1.bcr.1",
		"1.0.patch.3":         "1.0.patch.3",
		"1.0-pre+build.1":     "1.0-pre",
		"v20210324.2+build.1": "v20210324.2",
	}
	for input, want := range normalizations {
		normalized, err := NormalizeWithScheme(input, "bazel")
		if err != nil {
			t.Fatal(err)
		}
		if normalized != want {
			t.Errorf("NormalizeWithScheme(%q, bazel) = %q, want %q", input, normalized, want)
		}
	}

	if _, err := NormalizeWithScheme("1..0", "bazel"); err == nil {
		t.Error("NormalizeWithScheme accepted an invalid Bazel version")
	}
}

func TestBazelClassificationThroughPublicAPI(t *testing.T) {
	for _, version := range []string{"35.1", "0.7.1.bcr.1"} {
		if !IsStableWithScheme(version, "bazel") {
			t.Errorf("IsStableWithScheme(%q, bazel) = false", version)
		}
		if IsPrereleaseWithScheme(version, "bazel") {
			t.Errorf("IsPrereleaseWithScheme(%q, bazel) = true", version)
		}
	}

	if IsStableWithScheme("36.0-rc2", "bazel") {
		t.Error("IsStableWithScheme(36.0-rc2, bazel) = true")
	}
	if !IsPrereleaseWithScheme("36.0-rc2", "bazel") {
		t.Error("IsPrereleaseWithScheme(36.0-rc2, bazel) = false")
	}
	if !IsPrereleaseWithScheme("36.0-rc2.bcr.1", "bazel") {
		t.Error("IsPrereleaseWithScheme(36.0-rc2.bcr.1, bazel) = false")
	}
	if IsStableWithScheme("1..0", "bazel") || IsPrereleaseWithScheme("1..0", "bazel") {
		t.Error("invalid Bazel version was classified")
	}
}

func TestBazelRangesThroughPublicAPI(t *testing.T) {
	r, err := Parse("vers:bazel/>=36.0")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Contains("36.0.bcr.1") {
		t.Error("Parse Bazel range does not contain 36.0.bcr.1")
	}
	if r.Contains("36..0") {
		t.Error("Bazel range contains an invalid version")
	}

	native, err := ParseNative(">=36.0", "bazel")
	if err != nil {
		t.Fatal(err)
	}
	if !native.Contains("36.0.bcr.1") {
		t.Error("ParseNative Bazel range does not contain 36.0.bcr.1")
	}
	bounded, err := ParseNative(">=36.0|<36.0.bcr.2", "bazel")
	if err != nil {
		t.Fatal(err)
	}
	if !bounded.Contains("36.0.bcr.1") || bounded.Contains("36.0.bcr.2") {
		t.Error("ParseNative Bazel range did not apply both constraints")
	}

	satisfies, err := Satisfies("36.0.bcr.1", ">=36.0", "bazel")
	if err != nil {
		t.Fatal(err)
	}
	if !satisfies {
		t.Error("Satisfies returned false for 36.0.bcr.1 >= 36.0")
	}

	vPrefixed, err := Parse("vers:bazel/>=v36.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(vPrefixed.Intervals) != 1 || vPrefixed.Intervals[0].Min != "v36.0" {
		t.Errorf("Parse did not preserve Bazel release text: %#v", vPrefixed.Intervals)
	}
	if _, err := ParseNative(">=36..0", "bazel"); err == nil {
		t.Error("ParseNative accepted an invalid Bazel constraint version")
	}
}

func TestHighestSatisfyingBazelThroughPublicAPI(t *testing.T) {
	versions := []string{"36.0-rc2", "35.1", "36.0", "invalid_version", "36.0.bcr.1"}
	got, err := HighestSatisfying(versions, ">=35.1", "bazel")
	if err != nil {
		t.Fatal(err)
	}
	if got != "36.0.bcr.1" {
		t.Errorf("HighestSatisfying() = %q, want 36.0.bcr.1", got)
	}
}
