package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	vers "github.com/git-pkgs/vers"
)

const (
	schemaURL            = "https://packageurl.org/schemas/vers-test.schema-0.2.json"
	testTypeEquality     = "equality"
	testTypeContainment  = "containment"
	testGroupRecommended = "recommended"
	licenseMIT           = "MIT"
	testsPerComparison   = 2
	directoryMode        = 0o755
	fileMode             = 0o644
)

type sourceSpec struct {
	name                     string
	scheme                   string
	repository               string
	commit                   string
	license                  string
	localPath                string
	sourceFiles              []string
	outputFile               string
	extract                  func(map[string]string) ([]comparison, error)
	rangeOutputFile          string
	extractRanges            func(map[string]string) ([]nativeRangeAssertion, error)
	generatedRangeOutputFile string
	referenceRuntimes        []string
	evaluateRanges           rangeEvaluator
}

type comparison struct {
	left          string
	right         string
	result        int
	differentOnly bool
}

type nativeRangeAssertion struct {
	nativeRange string
	version     string
	contains    bool
}

type containmentQuery struct {
	Range   string `json:"range"`
	Version string `json:"version"`
}

type rangeEvaluator func(string, []containmentQuery) ([]bool, error)

type testFile struct {
	Schema string     `json:"$schema"`
	Tests  []testCase `json:"tests"`
}

type testCase struct {
	Description    string    `json:"description"`
	TestGroup      string    `json:"test_group"`
	TestType       string    `json:"test_type"`
	Input          testInput `json:"input"`
	ExpectedOutput any       `json:"expected_output"`
}

type testInput struct {
	InputType string   `json:"input_type,omitempty"`
	Versions  []string `json:"versions,omitempty"`
	Vers      string   `json:"vers,omitempty"`
	Version   string   `json:"version,omitempty"`
}

type provenanceFile struct {
	Sources []provenanceSource `json:"sources"`
}

type provenanceSource struct {
	Repository     string   `json:"repository"`
	Commit         string   `json:"commit"`
	License        string   `json:"license"`
	SourceFiles    []string `json:"source_files"`
	GeneratedFiles []string `json:"generated_files"`
}

var sources = []sourceSpec{
	{
		name: "node-semver", scheme: "npm",
		repository: "https://github.com/npm/node-semver.git",
		commit:     "6e05b7637396ac66522cff8731f07cfe0ef49a29", license: "ISC",
		localPath: "npm/node-semver",
		sourceFiles: []string{
			"test/fixtures/comparisons.js", "test/fixtures/equality.js",
			"test/fixtures/range-include.js", "test/fixtures/range-exclude.js",
		},
		outputFile: "npm_version_cmp_test.json", extract: extractNodeSemver,
		rangeOutputFile: "npm_range_reference_test.json", extractRanges: extractNodeSemverRanges,
		generatedRangeOutputFile: "npm_range_generated_test.json",
		referenceRuntimes:        []string{"node"}, evaluateRanges: evaluateNodeSemver,
	},
	{
		name: "packaging", scheme: "pypi",
		repository: "https://github.com/pypa/packaging.git",
		commit:     "55cbf1b9426f44455fa1a9e0836f1fc082cc8452", license: "Apache-2.0 OR BSD-2-Clause",
		localPath: "pypa/packaging", sourceFiles: []string{"tests/test_version.py", "tests/test_specifiers.py"},
		outputFile: "pypi_version_cmp_test.json", extract: extractPyPI,
		rangeOutputFile: "pypi_range_reference_test.json", extractRanges: extractPyPIRanges,
		generatedRangeOutputFile: "pypi_range_generated_test.json",
		referenceRuntimes:        []string{"python3"}, evaluateRanges: evaluatePyPI,
	},
	{
		name: "RubyGems", scheme: "gem",
		repository: "https://github.com/rubygems/rubygems.git",
		commit:     "370fe6876eec1714cd0f8824c3f19f4d368dfe7c", license: licenseMIT,
		localPath: "rubygems/rubygems", sourceFiles: []string{
			"test/rubygems/test_gem_version.rb", "test/rubygems/test_gem_requirement.rb",
		},
		outputFile: "gem_version_cmp_test.json", extract: extractRubyGems,
		rangeOutputFile: "gem_range_reference_test.json", extractRanges: extractRubyGemsRanges,
		generatedRangeOutputFile: "gem_range_generated_test.json",
		referenceRuntimes:        []string{"ruby"}, evaluateRanges: evaluateRubyGems,
	},
	{
		name: "composer/semver", scheme: "composer",
		repository: "https://github.com/composer/semver.git",
		commit:     "1cbc9b575a27458074d21a3bab95b847c8de387c", license: licenseMIT,
		localPath: "composer/semver", sourceFiles: []string{"tests/ComparatorTest.php", "tests/VersionParserTest.php"},
		outputFile: "composer_version_cmp_test.json", extract: extractComposer,
		rangeOutputFile: "composer_range_reference_test.json", extractRanges: extractComposerRanges,
		generatedRangeOutputFile: "composer_range_generated_test.json",
		referenceRuntimes:        []string{"php"}, evaluateRanges: evaluateComposer,
	},
	{
		name: "pub_semver", scheme: "pub",
		repository: "https://github.com/dart-lang/tools.git",
		commit:     "a22c3a687dc1b35630fb34c296157d66565429cd", license: "BSD-3-Clause",
		localPath: "dart-lang/tools", sourceFiles: []string{
			"pkgs/pub_semver/test/version_test.dart",
			"pkgs/pub_semver/test/version_range_test.dart",
			"pkgs/pub_semver/test/version_constraint_test.dart",
			"pkgs/pub_semver/test/utils.dart",
		},
		outputFile: "pub_version_cmp_test.json", extract: extractPub,
		rangeOutputFile: "pub_range_reference_test.json", extractRanges: extractPubRanges,
		generatedRangeOutputFile: "pub_range_generated_test.json",
		referenceRuntimes:        []string{"dart"}, evaluateRanges: evaluatePub,
	},
	{
		name: "semver", scheme: "cargo",
		repository: "https://github.com/dtolnay/semver.git",
		commit:     "280ebcb6edac3aa4cdc545dbff8a26c5ac4861fe", license: "MIT OR Apache-2.0",
		localPath: "dtolnay/semver", sourceFiles: []string{"tests/test_version.rs", "tests/test_version_req.rs"},
		outputFile: "cargo_version_cmp_test.json", extract: extractCargo,
		rangeOutputFile: "cargo_range_reference_test.json", extractRanges: extractCargoRanges,
		generatedRangeOutputFile: "cargo_range_generated_test.json",
		referenceRuntimes:        []string{"rustc"}, evaluateRanges: evaluateCargo,
	},
	{
		name: "maven-artifact", scheme: "maven",
		repository: "https://github.com/apache/maven.git",
		commit:     "48514f63b2799844c9f8d7746f73a95123526df7", license: "Apache-2.0",
		localPath: "apache/maven",
		sourceFiles: []string{
			"compat/maven-artifact/src/test/java/org/apache/maven/artifact/versioning/VersionRangeTest.java",
		},
		rangeOutputFile: "maven_range_reference_test.json", extractRanges: extractMavenRanges,
		generatedRangeOutputFile: "maven_range_generated_test.json",
		referenceRuntimes:        []string{"javac", "java"}, evaluateRanges: evaluateMaven,
	},
	{
		name: "dpkg", scheme: "deb",
		repository: "https://salsa.debian.org/dpkg-team/dpkg.git",
		commit:     "c5efef926d16a72fd9270c8efceeba8872419292", license: "GPL-2.0-or-later",
		localPath: "dpkg-team/dpkg", sourceFiles: []string{"lib/dpkg/t/t-version.c"},
		outputFile: "deb_version_cmp_test.json", extract: extractDpkg,
	},
	{
		name: "RPM", scheme: "rpm",
		repository: "https://github.com/rpm-software-management/rpm.git",
		commit:     "ac5062c43e2337fb43a9179f02c0cc5d7d069802", license: "GPL-2.0-or-later",
		localPath: "rpm-software-management/rpm", sourceFiles: []string{"tests/rpmvercmp.at"},
		outputFile: "rpm_version_cmp_test.json", extract: extractRPM,
	},
	{
		name: "golang.org/x/mod/semver", scheme: "go",
		repository: "https://github.com/golang/mod.git",
		commit:     "d3398d06de5fa5c71083d3d1c26f2cda73508e0f", license: "BSD-3-Clause",
		localPath: "golang/mod", sourceFiles: []string{"semver/semver_test.go"},
		outputFile: "go_version_cmp_test.json", extract: extractGo,
	},
}

func main() {
	sourceRoot := flag.String("source-root", "", "read pinned commits from checkouts below this directory")
	outputRoot := flag.String("output", filepath.Join("testdata", "local"), "local conformance data directory")
	flag.Parse()

	if err := run(*sourceRoot, *outputRoot); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(sourceRoot, outputRoot string) error {
	testsDir := filepath.Join(outputRoot, "tests")
	if err := os.MkdirAll(testsDir, directoryMode); err != nil {
		return fmt.Errorf("create tests directory: %w", err)
	}

	provenance := provenanceFile{Sources: []provenanceSource{
		{
			Repository:  "https://github.com/git-pkgs/vers",
			Commit:      "aca1342a0af1b6308ced629af1913ab42a66dbb5",
			License:     licenseMIT,
			SourceFiles: []string{"composer_pub_test.go"},
			GeneratedFiles: []string{
				"tests/composer_range_containment_test.json",
				"tests/composer_range_from_native_test.json",
				"tests/pub_range_containment_test.json",
				"tests/pub_range_from_native_test.json",
			},
		},
	}}

	for _, source := range sources {
		files, checkout, cleanup, err := readSource(source, sourceRoot)
		if err != nil {
			return fmt.Errorf("read %s: %w", source.name, err)
		}
		generatedFiles, err := harvestSource(source, files, checkout, testsDir)
		cleanup()
		if err != nil {
			return err
		}
		provenance.Sources = append(provenance.Sources, provenanceSource{
			Repository: source.repository, Commit: source.commit, License: source.license,
			SourceFiles: source.sourceFiles, GeneratedFiles: generatedFiles,
		})
	}

	return writeJSON(filepath.Join(outputRoot, "provenance.json"), provenance)
}

func harvestSource(source sourceSpec, files map[string]string, checkout, testsDir string) ([]string, error) {
	var generatedFiles []string
	var comparisons []comparison
	if source.extract != nil {
		var err error
		comparisons, err = source.extract(files)
		if err != nil {
			return nil, fmt.Errorf("extract %s comparisons: %w", source.name, err)
		}
		if err := writeJSON(filepath.Join(testsDir, source.outputFile), buildTestFile(source, comparisons)); err != nil {
			return nil, err
		}
		generatedFiles = append(generatedFiles, filepath.ToSlash(filepath.Join("tests", source.outputFile)))
	}

	var assertions []nativeRangeAssertion
	if source.extractRanges != nil {
		var err error
		assertions, err = source.extractRanges(files)
		if err != nil {
			return nil, fmt.Errorf("extract %s ranges: %w", source.name, err)
		}
		file, err := buildRangeTestFile(source, assertions)
		if err != nil {
			return nil, fmt.Errorf("build %s ranges: %w", source.name, err)
		}
		if err := writeJSON(filepath.Join(testsDir, source.rangeOutputFile), file); err != nil {
			return nil, err
		}
		generatedFiles = append(generatedFiles, filepath.ToSlash(filepath.Join("tests", source.rangeOutputFile)))
	}

	if source.evaluateRanges != nil {
		generatedFilename := filepath.ToSlash(filepath.Join("tests", source.generatedRangeOutputFile))
		generatedFiles = append(generatedFiles, generatedFilename)
		if missing := missingRuntime(source.referenceRuntimes); missing != "" {
			fmt.Fprintf(os.Stderr, "warning: skip %s generated containment tests: %s not found on PATH\n", source.name, missing)
			return generatedFiles, nil
		}
		queries := crossContainmentQueries(assertions, comparisons)
		results, err := source.evaluateRanges(checkout, queries)
		if err != nil {
			return nil, fmt.Errorf("evaluate %s ranges: %w", source.name, err)
		}
		generatedAssertions, err := applyContainmentResults(queries, results)
		if err != nil {
			return nil, fmt.Errorf("evaluate %s ranges: %w", source.name, err)
		}
		file, err := buildRangeTestFile(source, generatedAssertions)
		if err != nil {
			return nil, fmt.Errorf("build generated %s ranges: %w", source.name, err)
		}
		if err := writeJSON(filepath.Join(testsDir, source.generatedRangeOutputFile), file); err != nil {
			return nil, err
		}
	}

	return generatedFiles, nil
}

func readSource(source sourceSpec, sourceRoot string) (map[string]string, string, func(), error) {
	if sourceRoot != "" {
		repository := filepath.Join(sourceRoot, filepath.FromSlash(source.localPath))
		files, err := readGitFiles(repository, source.commit, source.sourceFiles)
		if err != nil {
			return nil, "", func() {}, err
		}
		checkout, cleanup, err := referenceCheckout(repository, source.commit, source.evaluateRanges != nil)
		return files, checkout, cleanup, err
	}

	directory, err := os.MkdirTemp("", "vers-harvest-")
	if err != nil {
		return nil, "", func() {}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(directory)
	}

	if err := runGit(directory, "init", "--quiet"); err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	if err := runGit(directory, "remote", "add", "origin", source.repository); err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	if err := runGit(directory, "fetch", "--quiet", "--depth=1", "origin", source.commit); err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	if source.evaluateRanges != nil {
		if err := runGit(directory, "checkout", "--quiet", "--detach", "FETCH_HEAD"); err != nil {
			cleanup()
			return nil, "", func() {}, err
		}
	}
	files, err := readGitFiles(directory, "FETCH_HEAD", source.sourceFiles)
	if err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	return files, directory, cleanup, nil
}

func readGitFiles(repository, revision string, filenames []string) (map[string]string, error) {
	if _, err := os.Stat(repository); err != nil {
		return nil, fmt.Errorf("open checkout %s: %w", repository, err)
	}

	files := make(map[string]string, len(filenames))
	for _, filename := range filenames {
		command := exec.Command("git", "-C", repository, "show", revision+":"+filename)
		output, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("git show %s:%s: %w", revision, filename, err)
		}
		files[filename] = string(output)
	}
	return files, nil
}

func runGit(directory string, arguments ...string) error {
	arguments = append([]string{"-C", directory}, arguments...)
	command := exec.Command("git", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", command.String(), err, output)
	}
	return nil
}

func buildTestFile(source sourceSpec, comparisons []comparison) testFile {
	tests := make([]testCase, 0, len(comparisons)*testsPerComparison)
	seen := make(map[string]bool, len(comparisons)*testsPerComparison)
	appendTest := func(test testCase) {
		keyBytes, _ := json.Marshal(test.ExpectedOutput)
		left, right := test.Input.Versions[0], test.Input.Versions[1]
		if test.TestType == testTypeEquality && right < left {
			left, right = right, left
		}
		key := test.TestType + "\x00" + left + "\x00" + right + "\x00" + string(keyBytes)
		if seen[key] {
			return
		}
		seen[key] = true
		tests = append(tests, test)
	}
	for _, item := range comparisons {
		left, right, result := item.left, item.right, item.result
		if left == right {
			continue
		}
		if item.differentOnly {
			appendTest(testCase{
				Description: fmt.Sprintf("%s treats %q and %q as different.", source.name, left, right),
				TestGroup:   testGroupRecommended, TestType: testTypeEquality,
				Input:          testInput{InputType: source.scheme, Versions: []string{left, right}},
				ExpectedOutput: false,
			})
			continue
		}

		testType := "comparison"
		expected := any([]string{left, right})
		input := []string{right, left}
		description := fmt.Sprintf("%s orders %q before %q.", source.name, left, right)
		if result > 0 {
			left, right = right, left
			expected = []string{left, right}
			input = []string{right, left}
			description = fmt.Sprintf("%s orders %q before %q.", source.name, left, right)
		} else if result == 0 {
			testType = testTypeEquality
			expected = true
			input = []string{left, right}
			description = fmt.Sprintf("%s treats %q and %q as equal.", source.name, left, right)
		}

		appendTest(testCase{
			Description: description, TestGroup: testGroupRecommended, TestType: testType,
			Input: testInput{InputType: source.scheme, Versions: input}, ExpectedOutput: expected,
		})
		if result != 0 {
			appendTest(testCase{
				Description: fmt.Sprintf("%s treats %q and %q as different.", source.name, item.left, item.right),
				TestGroup:   testGroupRecommended, TestType: testTypeEquality,
				Input:          testInput{InputType: source.scheme, Versions: []string{item.left, item.right}},
				ExpectedOutput: false,
			})
		}
	}
	return testFile{Schema: schemaURL, Tests: tests}
}

func buildRangeTestFile(source sourceSpec, assertions []nativeRangeAssertion) (testFile, error) {
	tests := make([]testCase, 0, len(assertions))
	seen := make(map[string]bool, len(assertions))
	expectedByInput := make(map[string]bool, len(assertions))
	for _, assertion := range assertions {
		r, err := vers.ParseNative(assertion.nativeRange, source.scheme)
		if err != nil {
			return testFile{}, fmt.Errorf("parse native range %q: %w", assertion.nativeRange, err)
		}
		// RawConstraints preserve legacy from-native rendering. Containment
		// fixtures need the expanded intervals that implement native behavior.
		containmentRange := *r
		containmentRange.RawConstraints = nil
		if r.IsEmpty() {
			containmentRange = *vers.NewRange([]vers.Interval{vers.ExactInterval("0.0.0")})
			containmentRange.Exclusions = []string{"0.0.0"}
			containmentRange.Scheme = source.scheme
		}
		versURI := vers.ToVersString(&containmentRange, source.scheme)
		inputKey := versURI + "\x00" + assertion.version
		if expected, ok := expectedByInput[inputKey]; ok && expected != assertion.contains {
			return testFile{}, fmt.Errorf("native ranges disagree after conversion for %q and version %q", versURI, assertion.version)
		}
		expectedByInput[inputKey] = assertion.contains
		key := inputKey + "\x00" + strconv.FormatBool(assertion.contains)
		if seen[key] {
			continue
		}
		seen[key] = true
		verb := "excludes"
		if assertion.contains {
			verb = "contains"
		}
		tests = append(tests, testCase{
			Description: fmt.Sprintf("%s range %q %s %q.", source.name, assertion.nativeRange, verb, assertion.version),
			TestGroup:   testGroupRecommended, TestType: testTypeContainment,
			Input: testInput{Vers: versURI, Version: assertion.version}, ExpectedOutput: assertion.contains,
		})
	}
	if len(tests) == 0 {
		return testFile{}, errors.New("no native range assertions found")
	}
	return testFile{Schema: schemaURL, Tests: tests}, nil
}

func writeJSON(filename string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filename, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filename, data, fileMode); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	return nil
}

func requireComparisons(items []comparison) ([]comparison, error) {
	if len(items) == 0 {
		return nil, errors.New("no comparisons found")
	}
	return items, nil
}
