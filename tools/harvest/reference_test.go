package main

import (
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
