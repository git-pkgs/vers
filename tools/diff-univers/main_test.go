package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadQueries(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "sample.json")
	data := []byte(`{
  "tests": [
    {
      "description": "orders versions",
      "test_type": "comparison",
      "input": {"input_type": "deb", "versions": ["2", "1"]}
    },
    {
      "description": "the same pair is unequal",
      "test_type": "equality",
      "input": {"input_type": "deb", "versions": ["2", "1"]}
    },
    {
      "description": "contains a version",
      "test_type": "containment",
      "input": {"vers": "vers:conan/>=1|<2", "version": "1.5"}
    },
    {
      "description": "ignored native parse case",
      "test_type": "native_parse",
      "input": {}
    }
  ]
}`)
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadQueries(filepath.Join(directory, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []query{
		{
			ID: 0, Operation: "compare", Scheme: "deb", Left: "2", Right: "1",
			Source: filename + ": orders versions",
		},
		{
			ID: 1, Operation: "contains", VERS: "vers:conan/>=1|<2", Version: "1.5",
			Source: filename + ": contains a version",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("queries = %#v, want %#v", got, want)
	}
}

func TestQueryForCaseRejectsInvalidComparison(t *testing.T) {
	_, _, err := queryForCase("fixture.json", testCase{
		Description: "bad input",
		TestType:    "comparison",
		Input:       []byte(`{"input_type":"rpm","versions":["1"]}`),
	})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestQueryKeyIncludesOperationAndInputs(t *testing.T) {
	first := query{Operation: "compare", Scheme: "deb", Left: "1", Right: "2"}
	second := query{Operation: "compare", Scheme: "deb", Left: "1", Right: "3"}
	if queryKey(first) == queryKey(second) {
		t.Fatal("different queries have the same key")
	}
}

func TestCompareResultsReportsDifferences(t *testing.T) {
	queries := []query{
		{ID: 0, Operation: "compare", Scheme: "deb", Left: "1", Right: "2", Source: "comparison"},
		{ID: 1, Operation: "contains", VERS: "vers:conan/>=1|<2", Version: "1.5", Source: "containment"},
	}
	results := []referenceResult{
		{ID: 0, Comparison: 1},
		{ID: 1, Contains: true},
	}
	var output bytes.Buffer
	differences, err := compareResults(queries, results, &output)
	if err != nil {
		t.Fatal(err)
	}
	if differences != 1 {
		t.Fatalf("differences = %d, want 1", differences)
	}
	if output.String() != "comparison\n  vers: -1; univers: 1\n" {
		t.Errorf("output = %q", output.String())
	}
}
