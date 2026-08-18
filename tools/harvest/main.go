package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	schemaURL            = "https://packageurl.org/schemas/vers-test.schema-0.2.json"
	testTypeEquality     = "equality"
	testGroupRecommended = "recommended"
	licenseMIT           = "MIT"
	testsPerComparison   = 2
	directoryMode        = 0o755
	fileMode             = 0o644
)

type sourceSpec struct {
	name        string
	scheme      string
	repository  string
	commit      string
	license     string
	localPath   string
	sourceFiles []string
	outputFile  string
	extract     func(map[string]string) ([]comparison, error)
}

type comparison struct {
	left          string
	right         string
	result        int
	differentOnly bool
}

type testFile struct {
	Schema string     `json:"$schema"`
	Tests  []testCase `json:"tests"`
}

type testCase struct {
	Description    string          `json:"description"`
	TestGroup      string          `json:"test_group"`
	TestType       string          `json:"test_type"`
	Input          comparisonInput `json:"input"`
	ExpectedOutput any             `json:"expected_output"`
}

type comparisonInput struct {
	InputType string   `json:"input_type"`
	Versions  []string `json:"versions"`
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
		localPath:   "npm/node-semver",
		sourceFiles: []string{"test/fixtures/comparisons.js", "test/fixtures/equality.js"},
		outputFile:  "npm_version_cmp_test.json", extract: extractNodeSemver,
	},
	{
		name: "packaging", scheme: "pypi",
		repository: "https://github.com/pypa/packaging.git",
		commit:     "55cbf1b9426f44455fa1a9e0836f1fc082cc8452", license: "Apache-2.0 OR BSD-2-Clause",
		localPath: "pypa/packaging", sourceFiles: []string{"tests/test_version.py"},
		outputFile: "pypi_version_cmp_test.json", extract: extractPyPI,
	},
	{
		name: "RubyGems", scheme: "gem",
		repository: "https://github.com/rubygems/rubygems.git",
		commit:     "370fe6876eec1714cd0f8824c3f19f4d368dfe7c", license: licenseMIT,
		localPath: "rubygems/rubygems", sourceFiles: []string{"test/rubygems/test_gem_version.rb"},
		outputFile: "gem_version_cmp_test.json", extract: extractRubyGems,
	},
	{
		name: "composer/semver", scheme: "composer",
		repository: "https://github.com/composer/semver.git",
		commit:     "1cbc9b575a27458074d21a3bab95b847c8de387c", license: licenseMIT,
		localPath: "composer/semver", sourceFiles: []string{"tests/ComparatorTest.php"},
		outputFile: "composer_version_cmp_test.json", extract: extractComposer,
	},
	{
		name: "pub_semver", scheme: "pub",
		repository: "https://github.com/dart-lang/tools.git",
		commit:     "a22c3a687dc1b35630fb34c296157d66565429cd", license: "BSD-3-Clause",
		localPath: "dart-lang/tools", sourceFiles: []string{"pkgs/pub_semver/test/version_test.dart"},
		outputFile: "pub_version_cmp_test.json", extract: extractPub,
	},
	{
		name: "semver", scheme: "cargo",
		repository: "https://github.com/dtolnay/semver.git",
		commit:     "280ebcb6edac3aa4cdc545dbff8a26c5ac4861fe", license: "MIT OR Apache-2.0",
		localPath: "dtolnay/semver", sourceFiles: []string{"tests/test_version.rs"},
		outputFile: "cargo_version_cmp_test.json", extract: extractCargo,
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
		files, err := readSource(source, sourceRoot)
		if err != nil {
			return fmt.Errorf("read %s: %w", source.name, err)
		}
		comparisons, err := source.extract(files)
		if err != nil {
			return fmt.Errorf("extract %s: %w", source.name, err)
		}
		if err := writeJSON(filepath.Join(testsDir, source.outputFile), buildTestFile(source, comparisons)); err != nil {
			return err
		}
		provenance.Sources = append(provenance.Sources, provenanceSource{
			Repository: source.repository, Commit: source.commit, License: source.license,
			SourceFiles: source.sourceFiles, GeneratedFiles: []string{filepath.ToSlash(filepath.Join("tests", source.outputFile))},
		})
	}

	return writeJSON(filepath.Join(outputRoot, "provenance.json"), provenance)
}

func readSource(source sourceSpec, sourceRoot string) (map[string]string, error) {
	if sourceRoot != "" {
		return readGitFiles(filepath.Join(sourceRoot, filepath.FromSlash(source.localPath)), source.commit, source.sourceFiles)
	}

	directory, err := os.MkdirTemp("", "vers-harvest-")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(directory)
	}()

	if err := runGit(directory, "init", "--quiet"); err != nil {
		return nil, err
	}
	if err := runGit(directory, "remote", "add", "origin", source.repository); err != nil {
		return nil, err
	}
	if err := runGit(directory, "fetch", "--quiet", "--depth=1", "origin", source.commit); err != nil {
		return nil, err
	}
	return readGitFiles(directory, "FETCH_HEAD", source.sourceFiles)
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
				Input:          comparisonInput{InputType: source.scheme, Versions: []string{left, right}},
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
			Input: comparisonInput{InputType: source.scheme, Versions: input}, ExpectedOutput: expected,
		})
		if result != 0 {
			appendTest(testCase{
				Description: fmt.Sprintf("%s treats %q and %q as different.", source.name, item.left, item.right),
				TestGroup:   testGroupRecommended, TestType: testTypeEquality,
				Input:          comparisonInput{InputType: source.scheme, Versions: []string{item.left, item.right}},
				ExpectedOutput: false,
			})
		}
	}
	return testFile{Schema: schemaURL, Tests: tests}
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
