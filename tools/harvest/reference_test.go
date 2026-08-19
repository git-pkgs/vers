package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCrossContainmentQueries(t *testing.T) {
	assertions := []nativeRangeAssertion{
		{nativeRange: "^1", version: "1.2.0"},
		{nativeRange: "^2", version: "2.1.0"},
		{nativeRange: "^1", version: "1.3.0"},
	}
	comparisons := []comparison{
		{left: "1.0.0", right: "2.0.0"},
		{left: "2.0.0", right: "3.0.0"},
	}

	got := crossContainmentQueries(assertions, comparisons)
	want := []containmentQuery{
		{Range: "^1", Version: "1.0.0"},
		{Range: "^1", Version: "2.0.0"},
		{Range: "^1", Version: "3.0.0"},
		{Range: "^2", Version: "1.0.0"},
		{Range: "^2", Version: "2.0.0"},
		{Range: "^2", Version: "3.0.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("queries = %#v, want %#v", got, want)
	}
}

func TestCrossContainmentQueriesFallsBackToAssertionVersions(t *testing.T) {
	assertions := []nativeRangeAssertion{
		{nativeRange: "[1,2)", version: "1.0"},
		{nativeRange: "[2,3)", version: "2.0"},
		{nativeRange: "[1,2)", version: "1.0"},
	}

	got := crossContainmentQueries(assertions, nil)
	want := []containmentQuery{
		{Range: "[1,2)", Version: "1.0"},
		{Range: "[1,2)", Version: "2.0"},
		{Range: "[2,3)", Version: "1.0"},
		{Range: "[2,3)", Version: "2.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("queries = %#v, want %#v", got, want)
	}
}

func TestApplyContainmentResults(t *testing.T) {
	queries := []containmentQuery{
		{Range: "^1", Version: "1.0.0"},
		{Range: "^1", Version: "2.0.0"},
	}
	got, err := applyContainmentResults(queries, []bool{true, false})
	if err != nil {
		t.Fatal(err)
	}
	want := []nativeRangeAssertion{
		{nativeRange: "^1", version: "1.0.0", contains: true},
		{nativeRange: "^1", version: "2.0.0", contains: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("assertions = %#v, want %#v", got, want)
	}

	if _, err := applyContainmentResults(queries, []bool{true}); err == nil {
		t.Error("applyContainmentResults accepted a short result list")
	}
}

func TestMissingRuntime(t *testing.T) {
	const name = "vers-harvest-runtime-that-does-not-exist"
	if got := missingRuntime([]string{name}); got != name {
		t.Errorf("missingRuntime() = %q, want %q", got, name)
	}
}

func TestHarvestSourceDoesNotReportSkippedGeneratedFixture(t *testing.T) {
	source := sourceSpec{
		name:                     "reference",
		generatedRangeOutputFile: "generated.json",
		referenceRuntimes:        []string{"vers-harvest-runtime-that-does-not-exist"},
		evaluateRanges: func(string, []containmentQuery) ([]bool, error) {
			return nil, nil
		},
	}
	generated, err := harvestSource(source, nil, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(generated) != 0 {
		t.Errorf("generated files = %v, want none", generated)
	}
}

func TestReadSourceReturnsAbsoluteCheckout(t *testing.T) {
	repository := filepath.Join(t.TempDir(), "reference")
	if err := os.Mkdir(repository, directoryMode); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repository, "init", "--quiet"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "fixture.txt"), []byte("fixture\n"), fileMode); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repository, "add", "fixture.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repository, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--quiet", "-m", "fixture"); err != nil {
		t.Fatal(err)
	}
	revision, err := gitOutput(repository, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sourceRoot, err := filepath.Rel(workingDirectory, filepath.Dir(repository))
	if err != nil {
		t.Fatal(err)
	}
	source := sourceSpec{
		localPath: "reference", commit: revision, sourceFiles: []string{"fixture.txt"},
		evaluateRanges: func(string, []containmentQuery) ([]bool, error) { return nil, nil },
	}
	_, checkout, cleanup, err := readSource(source, sourceRoot)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(checkout) {
		t.Errorf("checkout = %q, want an absolute path", checkout)
	}
}
