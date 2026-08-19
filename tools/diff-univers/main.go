package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	vers "github.com/git-pkgs/vers"
)

const universScript = `
import json
import sys

from univers.version_range import RANGE_CLASS_BY_SCHEMES
from univers.version_range import VersionRange
from univers.versions import AlpineLinuxVersion
from univers.versions import SemverVersion


ALIASES = {
    "alpine": "apk",
    "debian": "deb",
    "gentoo": "ebuild",
    "go": "golang",
    "rubygems": "gem",
}


def version_class(scheme):
    scheme = ALIASES.get(scheme, scheme)
    if scheme == "apk":
        return AlpineLinuxVersion
    if scheme == "semver":
        return SemverVersion
    range_class = RANGE_CLASS_BY_SCHEMES.get(scheme)
    if range_class is None:
        raise ValueError(f"univers does not support the {scheme!r} scheme")
    return range_class.version_class


def compare(query):
    cls = version_class(query["scheme"])
    left = cls(query["left"])
    right = cls(query["right"])
    return (left > right) - (left < right)


def contains(query):
    value = VersionRange.from_string(query["vers"])
    version = value.version_class(query["version"])
    return version in value


results = []
for query in json.load(sys.stdin):
    result = {"id": query["id"]}
    try:
        if query["operation"] == "compare":
            result["comparison"] = compare(query)
        else:
            result["contains"] = contains(query)
    except Exception as error:
        result["error"] = f"{type(error).__name__}: {error}"
    results.append(result)

json.dump(results, sys.stdout)
`

type testFile struct {
	Tests []testCase `json:"tests"`
}

type testCase struct {
	Description string          `json:"description"`
	TestType    string          `json:"test_type"`
	Input       json.RawMessage `json:"input"`
}

type comparisonInput struct {
	InputType string   `json:"input_type"`
	Versions  []string `json:"versions"`
}

type containmentInput struct {
	VERS    string `json:"vers"`
	Version string `json:"version"`
}

type query struct {
	ID        int    `json:"id"`
	Operation string `json:"operation"`
	Scheme    string `json:"scheme,omitempty"`
	Left      string `json:"left,omitempty"`
	Right     string `json:"right,omitempty"`
	VERS      string `json:"vers,omitempty"`
	Version   string `json:"version,omitempty"`
	Source    string `json:"-"`
}

type referenceResult struct {
	ID         int    `json:"id"`
	Comparison int    `json:"comparison"`
	Contains   bool   `json:"contains"`
	Error      string `json:"error"`
}

func main() {
	fixtures := flag.String("fixtures", "testdata/local/tests/*_univers_*_test.json", "glob of conformance fixtures to compare")
	python := flag.String("python", "python3", "Python executable with univers installed")
	universPath := flag.String("univers", "", "optional package-url/univers checkout")
	flag.Parse()

	queries, err := loadQueries(*fixtures)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(queries) == 0 {
		fmt.Fprintln(os.Stderr, "no comparison or containment cases found")
		os.Exit(1)
	}

	results, err := runUnivers(*python, *universPath, queries)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	differences, err := compareResults(queries, results, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if differences > 0 {
		fmt.Fprintf(os.Stderr, "%d difference(s) found\n", differences)
		os.Exit(1)
	}
	fmt.Printf("univers agrees with vers for %d cases\n", len(queries))
}

func loadQueries(pattern string) ([]query, error) {
	filenames, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("find fixtures: %w", err)
	}
	if len(filenames) == 0 {
		return nil, fmt.Errorf("no fixtures match %q", pattern)
	}
	sort.Strings(filenames)

	queries := make([]query, 0)
	seen := make(map[string]bool)
	for _, filename := range filenames {
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filename, err)
		}
		var tf testFile
		if err := json.Unmarshal(data, &tf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", filename, err)
		}
		for _, tc := range tf.Tests {
			q, ok, err := queryForCase(filename, tc)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			key := queryKey(q)
			if seen[key] {
				continue
			}
			seen[key] = true
			q.ID = len(queries)
			queries = append(queries, q)
		}
	}
	return queries, nil
}

func queryForCase(filename string, tc testCase) (query, bool, error) {
	source := filename + ": " + tc.Description
	switch tc.TestType {
	case "comparison", "equality":
		var input comparisonInput
		if err := json.Unmarshal(tc.Input, &input); err != nil {
			return query{}, false, fmt.Errorf("parse input for %s: %w", source, err)
		}
		if input.InputType == "" || len(input.Versions) != 2 {
			return query{}, false, fmt.Errorf("invalid comparison input for %s", source)
		}
		return query{
			Operation: "compare", Scheme: input.InputType,
			Left: input.Versions[0], Right: input.Versions[1], Source: source,
		}, true, nil
	case "containment":
		var input containmentInput
		if err := json.Unmarshal(tc.Input, &input); err != nil {
			return query{}, false, fmt.Errorf("parse input for %s: %w", source, err)
		}
		if input.VERS == "" || input.Version == "" {
			return query{}, false, fmt.Errorf("invalid containment input for %s", source)
		}
		return query{
			Operation: "contains", VERS: input.VERS, Version: input.Version, Source: source,
		}, true, nil
	default:
		return query{}, false, nil
	}
}

func queryKey(q query) string {
	return strings.Join([]string{q.Operation, q.Scheme, q.Left, q.Right, q.VERS, q.Version}, "\x00")
}

func runUnivers(python, checkout string, queries []query) ([]referenceResult, error) {
	input, err := json.Marshal(queries)
	if err != nil {
		return nil, fmt.Errorf("encode univers queries: %w", err)
	}

	command := exec.Command(python, "-c", universScript)
	command.Stdin = bytes.NewReader(input)
	if checkout != "" {
		sourcePath, err := filepath.Abs(filepath.Join(checkout, "src"))
		if err != nil {
			return nil, fmt.Errorf("resolve univers checkout: %w", err)
		}
		pythonPath := sourcePath
		if existing := os.Getenv("PYTHONPATH"); existing != "" {
			pythonPath += string(os.PathListSeparator) + existing
		}
		command.Env = append(os.Environ(), "PYTHONPATH="+pythonPath)
	}

	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("run univers: %s", message)
	}

	var results []referenceResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("decode univers results: %w", err)
	}
	return results, nil
}

func compareResults(queries []query, results []referenceResult, output io.Writer) (int, error) {
	if len(results) != len(queries) {
		return 0, fmt.Errorf("univers returned %d results for %d queries", len(results), len(queries))
	}

	differences := 0
	for i, q := range queries {
		result := results[i]
		if result.ID != q.ID {
			return 0, fmt.Errorf("univers result %d has query ID %d, want %d", i, result.ID, q.ID)
		}
		different, err := compareResult(q, result, output)
		if err != nil {
			return 0, err
		}
		if different {
			differences++
		}
	}
	return differences, nil
}

func compareResult(q query, result referenceResult, output io.Writer) (bool, error) {
	if result.Error != "" {
		if _, err := fmt.Fprintf(output, "%s\n  univers error: %s\n", q.Source, result.Error); err != nil {
			return false, fmt.Errorf("write difference: %w", err)
		}
		return true, nil
	}

	switch q.Operation {
	case "compare":
		actual := vers.CompareWithScheme(q.Left, q.Right, q.Scheme)
		if actual == result.Comparison {
			return false, nil
		}
		if _, err := fmt.Fprintf(output, "%s\n  vers: %d; univers: %d\n", q.Source, actual, result.Comparison); err != nil {
			return false, fmt.Errorf("write difference: %w", err)
		}
		return true, nil
	case "contains":
		return compareContainment(q, result, output)
	default:
		return false, errors.New("unknown query operation " + q.Operation)
	}
}

func compareContainment(q query, result referenceResult, output io.Writer) (bool, error) {
	rangeValue, err := vers.Parse(q.VERS)
	if err != nil {
		if _, writeErr := fmt.Fprintf(output, "%s\n  vers error: %s\n", q.Source, err); writeErr != nil {
			return false, fmt.Errorf("write difference: %w", writeErr)
		}
		return true, nil
	}
	actual := rangeValue.Contains(q.Version)
	if actual == result.Contains {
		return false, nil
	}
	if _, err := fmt.Fprintf(output, "%s\n  vers: %t; univers: %t\n", q.Source, actual, result.Contains); err != nil {
		return false, fmt.Errorf("write difference: %w", err)
	}
	return true, nil
}
