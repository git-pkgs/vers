package vers

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPKVersionData(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "local", "data", "apk_version.data"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var comparisons, validity int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if a, op, b, ok := splitAPKComparison(line); ok {
			comparisons++
			got := CompareWithScheme(a, b, "apk")
			if got != op {
				t.Errorf("CompareWithScheme(%q, %q, apk) = %d, want %d", a, b, got, op)
			}
			continue
		}

		if strings.ContainsRune(line, ' ') {
			continue
		}

		validity++
		version := line
		want := true
		if strings.HasPrefix(line, "!") {
			version = line[1:]
			want = false
		}
		if got := ValidWithScheme(version, "apk"); got != want {
			t.Errorf("ValidWithScheme(%q, apk) = %v, want %v", version, got, want)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	t.Logf("apk_version.data: %d comparisons, %d validity checks", comparisons, validity)
	if comparisons < 730 || validity < 30 {
		t.Fatalf("apk_version.data yielded %d comparisons and %d validity checks", comparisons, validity)
	}
}

func splitAPKComparison(line string) (a string, op int, b string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", 0, "", false
	}
	switch fields[1] {
	case "<":
		return fields[0], -1, fields[2], true
	case ">":
		return fields[0], 1, fields[2], true
	case "=":
		return fields[0], 0, fields[2], true
	}
	return "", 0, "", false
}

func TestAPKComparisonThroughPublicAPI(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"1.0", "1.0_alpha", 1},
		{"1.0", "1.0_p1", -1},
		{"1.0_cvs", "1.0", 1},
		{"1.0_git20240101", "1.0", 1},
		{"1.0_svn", "1.0_git", -1},
		{"1.0~1234", "1.0~1235", -1},
		{"1.0~1234-r1", "1.0~1234-r0", 1},
		{"1.0", "1.0bc", -1},
		{"1.06", "1.6", -1},
		{"1.006", "1.06", -1},
	}
	for _, tt := range tests {
		if got := CompareWithScheme(tt.left, tt.right, "apk"); got != tt.want {
			t.Errorf("CompareWithScheme(%q, %q, apk) = %d, want %d", tt.left, tt.right, got, tt.want)
		}
		if got := CompareWithScheme(tt.right, tt.left, "apk"); got != -tt.want {
			t.Errorf("CompareWithScheme(%q, %q, apk) = %d, want %d", tt.right, tt.left, got, -tt.want)
		}
	}
}

func TestAPKClassificationThroughPublicAPI(t *testing.T) {
	if !IsPrereleaseWithScheme("1.0_alpha", "apk") || !IsPrereleaseWithScheme("1.0_rc1", "apk") {
		t.Error("apk pre-release suffixes should classify as prerelease")
	}
	for _, stable := range []string{"1.0", "1.0_p1", "1.0_git20240101", "1.0-r1", "1.0~abcd"} {
		if IsPrereleaseWithScheme(stable, "apk") {
			t.Errorf("IsPrereleaseWithScheme(%q, apk) = true", stable)
		}
		if !IsStableWithScheme(stable, "apk") {
			t.Errorf("IsStableWithScheme(%q, apk) = false", stable)
		}
	}
	if IsStableWithScheme("0.1bc", "apk") || IsPrereleaseWithScheme("0.1bc", "apk") {
		t.Error("invalid apk version should not classify")
	}
}

func TestAPKRangesThroughPublicAPI(t *testing.T) {
	r, err := Parse("vers:apk/>=1.0|<2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Contains("1.0_git20240101") {
		t.Error("apk range should contain a git snapshot above the lower bound")
	}
	if r.Contains("1.0_alpha") {
		t.Error("apk range should not contain a pre-release below the lower bound")
	}

	if !ValidWithScheme("1.0_hg1", "alpine") {
		t.Error("alpine alias should validate apk versions")
	}
}
