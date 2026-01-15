package vers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type versTestFile struct {
	Tests []versTestCase `json:"tests"`
}

type versTestCase struct {
	Description    string          `json:"description"`
	TestGroup      string          `json:"test_group"`
	TestType       string          `json:"test_type"`
	Input          json.RawMessage `json:"input"`
	ExpectedOutput json.RawMessage `json:"expected_output"`
}

type fromNativeInput struct {
	NativeRange string `json:"native_range"`
	Scheme      string `json:"scheme"`
}

type containmentInput struct {
	Vers    string `json:"vers"`
	Version string `json:"version"`
}

type versionCmpInput struct {
	InputScheme string   `json:"input_scheme"`
	Versions    []string `json:"versions"`
}

func loadTestFile(t *testing.T, filename string) *versTestFile {
	t.Helper()
	path := filepath.Join("testdata", "vers-spec", "tests", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %s: %v", filename, err)
	}
	var tf versTestFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatalf("failed to parse test file %s: %v", filename, err)
	}
	return &tf
}

func TestConformance_FromNative(t *testing.T) {
	files := []string{
		"gem_range_from_native_test.json",
		"npm_range_from_native_test.json",
		"pypi_range_from_native_test.json",
		"nuget_range_from_native_test.json",
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			tf := loadTestFile(t, file)
			for _, tc := range tf.Tests {
				if tc.TestType != "from_native" {
					continue
				}

				var input fromNativeInput
				if err := json.Unmarshal(tc.Input, &input); err != nil {
					t.Errorf("failed to parse input: %v", err)
					continue
				}

				var expected string
				if err := json.Unmarshal(tc.ExpectedOutput, &expected); err != nil {
					t.Errorf("failed to parse expected output: %v", err)
					continue
				}

				t.Run(input.NativeRange, func(t *testing.T) {
					r, err := ParseNative(input.NativeRange, input.Scheme)
					if err != nil {
						t.Errorf("ParseNative(%q, %q) error: %v", input.NativeRange, input.Scheme, err)
						return
					}

					got := ToVersString(r, input.Scheme)
					if got != expected {
						t.Errorf("ParseNative(%q, %q) = %q, want %q", input.NativeRange, input.Scheme, got, expected)
					}
				})
			}
		})
	}
}

func TestConformance_Containment(t *testing.T) {
	files := []string{
		"npm_range_containment_test.json",
		"pypi_range_containment_test.json",
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			tf := loadTestFile(t, file)
			for _, tc := range tf.Tests {
				if tc.TestType != "containment" {
					continue
				}

				var input containmentInput
				if err := json.Unmarshal(tc.Input, &input); err != nil {
					t.Errorf("failed to parse input: %v", err)
					continue
				}

				var expected bool
				if err := json.Unmarshal(tc.ExpectedOutput, &expected); err != nil {
					t.Errorf("failed to parse expected output: %v", err)
					continue
				}

				t.Run(input.Vers+"_"+input.Version, func(t *testing.T) {
					r, err := Parse(input.Vers)
					if err != nil {
						t.Errorf("Parse(%q) error: %v", input.Vers, err)
						return
					}

					got := r.Contains(input.Version)
					if got != expected {
						t.Errorf("Parse(%q).Contains(%q) = %v, want %v", input.Vers, input.Version, got, expected)
					}
				})
			}
		})
	}
}

func TestConformance_VersionComparison(t *testing.T) {
	files := []string{
		"nuget_version_cmp_test.json",
		"maven_version_cmp_test.json",
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			tf := loadTestFile(t, file)
			for _, tc := range tf.Tests {
				var input versionCmpInput
				if err := json.Unmarshal(tc.Input, &input); err != nil {
					t.Errorf("failed to parse input: %v", err)
					continue
				}

				if len(input.Versions) != 2 {
					t.Errorf("expected 2 versions, got %d", len(input.Versions))
					continue
				}

				v1, v2 := input.Versions[0], input.Versions[1]

				switch tc.TestType {
				case "equality":
					var expected bool
					if err := json.Unmarshal(tc.ExpectedOutput, &expected); err != nil {
						t.Errorf("failed to parse expected output: %v", err)
						continue
					}

					t.Run("eq_"+v1+"_"+v2, func(t *testing.T) {
						cmp := CompareWithScheme(v1, v2, input.InputScheme)
						got := cmp == 0
						if got != expected {
							t.Errorf("CompareWithScheme(%q, %q, %q) == 0 is %v, want %v (cmp=%d)", v1, v2, input.InputScheme, got, expected, cmp)
						}
					})

				case "comparison":
					var expected []string
					if err := json.Unmarshal(tc.ExpectedOutput, &expected); err != nil {
						t.Errorf("failed to parse expected output: %v", err)
						continue
					}

					if len(expected) != 2 {
						t.Errorf("expected 2 versions in output, got %d", len(expected))
						continue
					}

					t.Run("cmp_"+v1+"_"+v2, func(t *testing.T) {
						cmp := CompareWithScheme(v1, v2, input.InputScheme)
						// expected[0] should be less than expected[1]
						// Use comparison to determine which version v1 matches (handles case normalization)
						v1MatchesFirst := CompareWithScheme(v1, expected[0], input.InputScheme) == 0
						if expected[0] == expected[1] || CompareWithScheme(expected[0], expected[1], input.InputScheme) == 0 {
							if cmp != 0 {
								t.Errorf("CompareWithScheme(%q, %q, %q) = %d, want 0 (equal versions)", v1, v2, input.InputScheme, cmp)
							}
						} else if v1MatchesFirst {
							// v1 matches the smaller version, so cmp(v1, v2) should be < 0
							if cmp >= 0 {
								t.Errorf("CompareWithScheme(%q, %q, %q) = %d, want < 0 (expected order: %v)", v1, v2, input.InputScheme, cmp, expected)
							}
						} else {
							// v1 matches the larger version, so cmp(v1, v2) should be > 0
							if cmp <= 0 {
								t.Errorf("CompareWithScheme(%q, %q, %q) = %d, want > 0 (expected order: %v)", v1, v2, input.InputScheme, cmp, expected)
							}
						}
					})
				}
			}
		})
	}
}
