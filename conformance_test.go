package vers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

const tupleSize = 2

type versTestFile struct {
	Tests []versTestCase `json:"tests"`
}

type versTestCase struct {
	Description     string          `json:"description"`
	TestGroup       string          `json:"test_group"`
	TestType        string          `json:"test_type"`
	Input           json.RawMessage `json:"input"`
	ExpectedOutput  json.RawMessage `json:"expected_output"`
	ExpectedFailure bool            `json:"expected_failure"`
	ExpectedMessage string          `json:"expected_message"`
}

type conformanceSkipFile struct {
	Skips []conformanceSkip `json:"skips"`
}

type conformanceSkip struct {
	File           string          `json:"file"`
	TestType       string          `json:"test_type"`
	Input          json.RawMessage `json:"input"`
	ExpectedOutput json.RawMessage `json:"expected_output,omitempty"`
	Reason         string          `json:"reason"`
	used           bool
}

func TestConformance(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*", "tests", "*.json"))
	if err != nil {
		t.Fatalf("find conformance files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no conformance files found")
	}

	skips := loadConformanceSkips(t)
	for _, filename := range files {
		relative, err := filepath.Rel("testdata", filename)
		if err != nil {
			t.Fatalf("make %s relative to testdata: %v", filename, err)
		}
		relative = filepath.ToSlash(relative)

		t.Run(relative, func(t *testing.T) {
			tf := loadTestFile(t, filename)
			for i, tc := range tf.Tests {
				name := fmt.Sprintf("%04d_%s", i, tc.Description)
				t.Run(name, func(t *testing.T) {
					if reason, ok := skips.match(relative, tc); ok {
						t.Skip(reason)
					}
					runConformanceCase(t, tc)
				})
			}
		})
	}
	skips.assertAllUsed(t)
}

func loadTestFile(t *testing.T, filename string) *versTestFile {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read test file %s: %v", filename, err)
	}
	var tf versTestFile
	if err := json.Unmarshal(data, &tf); err != nil {
		t.Fatalf("parse test file %s: %v", filename, err)
	}
	return &tf
}

func loadConformanceSkips(t *testing.T) *conformanceSkipFile {
	t.Helper()
	filename := filepath.Join("testdata", "local", "skip.json")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read conformance skips: %v", err)
	}
	var skips conformanceSkipFile
	if err := json.Unmarshal(data, &skips); err != nil {
		t.Fatalf("parse conformance skips: %v", err)
	}

	seen := make(map[string]bool, len(skips.Skips))
	for i := range skips.Skips {
		skip := &skips.Skips[i]
		if skip.File == "" || skip.TestType == "" || len(skip.Input) == 0 || skip.Reason == "" {
			t.Fatalf("conformance skip %d must have file, test_type, input, and reason", i)
		}
		key := skip.identity(t)
		if seen[key] {
			t.Fatalf("duplicate conformance skip %d for %s", i, skip.File)
		}
		seen[key] = true
	}
	return &skips
}

func (skips *conformanceSkipFile) match(filename string, tc versTestCase) (string, bool) {
	for i := range skips.Skips {
		skip := &skips.Skips[i]
		if skip.File != filename || skip.TestType != tc.TestType || !equalJSON(skip.Input, tc.Input) {
			continue
		}
		if len(skip.ExpectedOutput) > 0 && !equalJSON(skip.ExpectedOutput, tc.ExpectedOutput) {
			continue
		}
		skip.used = true
		return skip.Reason, true
	}
	return "", false
}

func (skips *conformanceSkipFile) assertAllUsed(t *testing.T) {
	t.Helper()
	for _, skip := range skips.Skips {
		if !skip.used {
			t.Errorf("unused conformance skip for %s %s: %s", skip.File, skip.TestType, skip.Reason)
		}
	}
}

func (skip conformanceSkip) identity(t *testing.T) string {
	t.Helper()
	return skip.File + "\x00" + skip.TestType + "\x00" + normalizedJSON(t, skip.Input) +
		"\x00" + normalizedJSON(t, skip.ExpectedOutput)
}

func normalizedJSON(t *testing.T, data json.RawMessage) string {
	t.Helper()
	if len(data) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("parse JSON value: %v", err)
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("normalize JSON value: %v", err)
	}
	return string(normalized)
}

func equalJSON(a, b json.RawMessage) bool {
	var av, bv any
	if json.Unmarshal(a, &av) != nil || json.Unmarshal(b, &bv) != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

func runConformanceCase(t *testing.T, tc versTestCase) {
	t.Helper()
	switch tc.TestType {
	case "from_native":
		runFromNativeCase(t, tc)
	case "containment":
		runContainmentCase(t, tc)
	case "validate", "roundtrip":
		runValidateCase(t, tc)
	case "comparison":
		runComparisonCase(t, tc)
	case "equality":
		runEqualityCase(t, tc)
	case "parse":
		runParseCase(t, tc)
	default:
		t.Fatalf("unsupported conformance test type %q", tc.TestType)
	}
}

func runFromNativeCase(t *testing.T, tc versTestCase) {
	t.Helper()
	var input struct {
		NativeRange string `json:"native_range"`
		Type        string `json:"type"`
		Scheme      string `json:"scheme"`
	}
	decodeConformanceValue(t, tc.Input, &input)
	scheme := chooseAlias(t, "from-native type", input.Type, input.Scheme)

	r, err := ParseNative(input.NativeRange, scheme)
	if !checkConformanceError(t, tc, err) {
		return
	}
	var expected string
	decodeConformanceValue(t, tc.ExpectedOutput, &expected)
	if got := ToVersString(r, scheme); got != expected {
		t.Errorf("ParseNative(%q, %q) = %q, want %q", input.NativeRange, scheme, got, expected)
	}
}

func runContainmentCase(t *testing.T, tc versTestCase) {
	t.Helper()
	var input struct {
		Vers    string `json:"vers"`
		Version string `json:"version"`
	}
	decodeConformanceValue(t, tc.Input, &input)
	r, err := Parse(input.Vers)
	if !checkConformanceError(t, tc, err) {
		return
	}
	var expected bool
	decodeConformanceValue(t, tc.ExpectedOutput, &expected)
	if got := r.Contains(input.Version); got != expected {
		t.Errorf("Parse(%q).Contains(%q) = %v, want %v", input.Vers, input.Version, got, expected)
	}
}

func runValidateCase(t *testing.T, tc versTestCase) {
	t.Helper()
	input := decodeVersInput(t, tc.Input)
	r, err := defaultParser.parseVersURI(input, false)
	if !checkConformanceError(t, tc, err) {
		return
	}
	var expected string
	decodeConformanceValue(t, tc.ExpectedOutput, &expected)
	if got := ToVersString(r, r.Scheme); got != expected {
		t.Errorf("validate %q = %q, want %q", input, got, expected)
	}
}

func runComparisonCase(t *testing.T, tc versTestCase) {
	t.Helper()
	input, scheme := decodeComparisonInput(t, tc.Input)
	var expected []string
	decodeConformanceValue(t, tc.ExpectedOutput, &expected)
	if len(input) != len(expected) || len(input) < tupleSize {
		t.Fatalf("comparison has %d inputs and %d outputs, want equal lengths of at least %d", len(input), len(expected), tupleSize)
	}

	got := append([]string(nil), input...)
	sort.SliceStable(got, func(i, j int) bool {
		return CompareWithScheme(got[i], got[j], scheme) < 0
	})
	for i := range got {
		if CompareWithScheme(got[i], expected[i], scheme) != 0 {
			t.Errorf("sorted versions = %v, want %v", got, expected)
			return
		}
	}
}

func runEqualityCase(t *testing.T, tc versTestCase) {
	t.Helper()
	versions, scheme := decodeComparisonInput(t, tc.Input)
	if len(versions) != tupleSize {
		t.Fatalf("equality has %d inputs, want %d", len(versions), tupleSize)
	}
	var expected bool
	decodeConformanceValue(t, tc.ExpectedOutput, &expected)
	got := CompareWithScheme(versions[0], versions[1], scheme) == 0
	if got != expected {
		t.Errorf("CompareWithScheme(%q, %q, %q) == 0 is %v, want %v", versions[0], versions[1], scheme, got, expected)
	}
}

func runParseCase(t *testing.T, tc versTestCase) {
	t.Helper()
	var input string
	decodeConformanceValue(t, tc.Input, &input)
	r, err := defaultParser.parseVersURI(input, true)
	if !checkConformanceError(t, tc, err) {
		return
	}

	var output struct {
		Type               string     `json:"type"`
		Scheme             string     `json:"scheme"`
		Constraints        [][]string `json:"constraints"`
		VersionConstraints [][]string `json:"version_constraints"`
	}
	decodeConformanceValue(t, tc.ExpectedOutput, &output)
	scheme := chooseAlias(t, "parse output type", output.Type, output.Scheme)
	constraints := chooseConstraintsAlias(t, output.Constraints, output.VersionConstraints)
	if r.Scheme != scheme {
		t.Errorf("Parse(%q) scheme = %q, want %q", input, r.Scheme, scheme)
	}
	if got := rangeConstraints(r); !reflect.DeepEqual(got, constraints) {
		t.Errorf("Parse(%q) constraints = %v, want %v", input, got, constraints)
	}
}

func decodeComparisonInput(t *testing.T, data json.RawMessage) ([]string, string) {
	t.Helper()
	var input struct {
		InputType   string   `json:"input_type"`
		InputScheme string   `json:"input_scheme"`
		Versions    []string `json:"versions"`
	}
	decodeConformanceValue(t, data, &input)
	return input.Versions, chooseAlias(t, "comparison input type", input.InputType, input.InputScheme)
}

func decodeVersInput(t *testing.T, data json.RawMessage) string {
	t.Helper()
	var input string
	if err := json.Unmarshal(data, &input); err == nil {
		return input
	}
	var legacy struct {
		Vers string `json:"vers"`
	}
	decodeConformanceValue(t, data, &legacy)
	if legacy.Vers == "" {
		t.Fatal("VERS input is empty")
	}
	return legacy.Vers
}

func decodeConformanceValue(t *testing.T, data json.RawMessage, value any) {
	t.Helper()
	if len(data) == 0 {
		t.Fatal("missing conformance value")
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode conformance value: %v", err)
	}
}

func chooseAlias(t *testing.T, name, current, legacy string) string {
	t.Helper()
	if current != "" && legacy != "" && current != legacy {
		t.Fatalf("conflicting %s values %q and %q", name, current, legacy)
	}
	if current != "" {
		return current
	}
	if legacy == "" {
		t.Fatalf("missing %s", name)
	}
	return legacy
}

func chooseConstraintsAlias(t *testing.T, current, legacy [][]string) [][]string {
	t.Helper()
	if current != nil && legacy != nil && !reflect.DeepEqual(current, legacy) {
		t.Fatalf("conflicting parse output constraints %v and %v", current, legacy)
	}
	if current != nil {
		return current
	}
	if legacy == nil {
		t.Fatal("missing parse output constraints")
	}
	return legacy
}

func checkConformanceError(t *testing.T, tc versTestCase, err error) bool {
	t.Helper()
	if tc.ExpectedFailure {
		if err == nil {
			t.Errorf("expected failure: %s", tc.ExpectedMessage)
		} else if tc.ExpectedMessage != "" && err.Error() != tc.ExpectedMessage {
			t.Errorf("error = %q, want %q", err, tc.ExpectedMessage)
		}
		return false
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return true
}

func rangeConstraints(r *Range) [][]string {
	intervals := r.Intervals
	if len(r.RawConstraints) > 0 {
		intervals = r.RawConstraints
	}
	constraints := make([][]string, 0, len(intervals)+len(r.Exclusions))
	for _, interval := range intervals {
		if interval.Min != "" && interval.Min == interval.Max && interval.MinInclusive && interval.MaxInclusive {
			constraints = append(constraints, []string{"=", interval.Min})
			continue
		}
		if interval.Min != "" {
			operator := ">"
			if interval.MinInclusive {
				operator = ">="
			}
			constraints = append(constraints, []string{operator, interval.Min})
		}
		if interval.Max != "" {
			operator := "<"
			if interval.MaxInclusive {
				operator = "<="
			}
			constraints = append(constraints, []string{operator, interval.Max})
		}
	}
	for _, version := range r.Exclusions {
		constraints = append(constraints, []string{"!=", version})
	}
	return constraints
}

func TestConformanceSkipMatch(t *testing.T) {
	skips := conformanceSkipFile{Skips: []conformanceSkip{{
		File:           "vers-spec/tests/example.json",
		TestType:       "containment",
		Input:          json.RawMessage(`{"vers":"vers:npm/*","version":"1.0.0"}`),
		ExpectedOutput: json.RawMessage(`true`),
		Reason:         "documented divergence",
	}}}
	tc := versTestCase{
		TestType:       "containment",
		Input:          json.RawMessage(`{"version":"1.0.0","vers":"vers:npm/*"}`),
		ExpectedOutput: json.RawMessage(`true`),
	}
	reason, ok := skips.match("vers-spec/tests/example.json", tc)
	if !ok || reason != "documented divergence" || !skips.Skips[0].used {
		t.Fatalf("match() = %q, %v; used = %v", reason, ok, skips.Skips[0].used)
	}
}
