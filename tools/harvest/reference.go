package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	nodeSemverEvaluator = `
const fs = require('fs')
const semver = require(process.argv[2])
const queries = JSON.parse(fs.readFileSync(0, 'utf8'))
process.stdout.write(JSON.stringify(queries.map(query => semver.satisfies(query.version, query.range))))
`
	pyPIEvaluator = `
import json
import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(sys.argv[1]) / "src"))
from packaging.specifiers import SpecifierSet

queries = json.load(sys.stdin)
json.dump([SpecifierSet(query["range"]).contains(query["version"]) for query in queries], sys.stdout)
`
	rubyGemsEvaluator = `
require "json"
require "rubygems/requirement"
require "rubygems/version"

queries = JSON.parse(STDIN.read)
results = queries.map do |query|
  Gem::Requirement.new(query.fetch("range")).satisfied_by?(Gem::Version.new(query.fetch("version")))
end
STDOUT.write(JSON.generate(results))
`
	composerEvaluator = `
<?php
$root = $argv[1];
spl_autoload_register(function ($class) use ($root) {
    $prefix = 'Composer\\Semver\\';
    if (strncmp($class, $prefix, strlen($prefix)) !== 0) {
        return;
    }
    $file = $root . '/src/' . str_replace('\\', '/', substr($class, strlen($prefix))) . '.php';
    if (is_file($file)) {
        require $file;
    }
});

$queries = json_decode(stream_get_contents(STDIN), true, 512, JSON_THROW_ON_ERROR);
$parser = new Composer\Semver\VersionParser();
$results = array();
foreach ($queries as $query) {
    $range = $parser->parseConstraints($query['range']);
    $version = new Composer\Semver\Constraint\Constraint('==', $parser->normalize($query['version']));
    $results[] = $range->matches($version);
}
echo json_encode($results, JSON_THROW_ON_ERROR);
`
	pubEvaluator = `
import 'dart:convert';
import 'dart:io';

import 'package:pub_semver/pub_semver.dart';

Future<void> main() async {
  final input = await stdin.transform(utf8.decoder).join();
  final queries = jsonDecode(input) as List<dynamic>;
  final results = queries.map((item) {
    final query = item as Map<String, dynamic>;
    return VersionConstraint.parse(query['range'] as String)
        .allows(Version.parse(query['version'] as String));
  }).toList();
  stdout.write(jsonEncode(results));
}
`
)

func missingRuntime(runtimes []string) string {
	for _, runtime := range runtimes {
		if _, err := exec.LookPath(runtime); err != nil {
			return runtime
		}
	}
	return ""
}

func referenceCheckout(repository, revision string, needed bool) (string, func(), error) {
	cleanup := func() {}
	if !needed {
		return repository, cleanup, nil
	}

	head, err := gitOutput(repository, "rev-parse", "HEAD")
	if err != nil {
		return "", cleanup, err
	}
	status, err := gitOutput(repository, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", cleanup, err
	}
	if strings.TrimSpace(head) == revision && strings.TrimSpace(status) == "" {
		return repository, cleanup, nil
	}

	directory, err := os.MkdirTemp("", "vers-harvest-reference-")
	if err != nil {
		return "", cleanup, err
	}
	cleanup = func() {
		_ = os.RemoveAll(directory)
	}
	if err := runGit(directory, "init", "--quiet"); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := runGit(directory, "remote", "add", "origin", repository); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := runGit(directory, "fetch", "--quiet", "--depth=1", "origin", revision); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := runGit(directory, "checkout", "--quiet", "--detach", "FETCH_HEAD"); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return directory, cleanup, nil
}

func gitOutput(repository string, arguments ...string) (string, error) {
	arguments = append([]string{"-C", repository}, arguments...)
	command := exec.Command("git", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", command.String(), err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func crossContainmentQueries(assertions []nativeRangeAssertion, comparisons []comparison) []containmentQuery {
	ranges := make([]string, 0, len(assertions))
	seenRanges := make(map[string]bool, len(assertions))
	for _, assertion := range assertions {
		if !seenRanges[assertion.nativeRange] {
			seenRanges[assertion.nativeRange] = true
			ranges = append(ranges, assertion.nativeRange)
		}
	}

	versions := make([]string, 0, len(comparisons)*testsPerComparison)
	seenVersions := make(map[string]bool, len(comparisons)*testsPerComparison)
	appendVersion := func(version string) {
		if !seenVersions[version] {
			seenVersions[version] = true
			versions = append(versions, version)
		}
	}
	for _, item := range comparisons {
		appendVersion(item.left)
		appendVersion(item.right)
	}
	if len(versions) == 0 {
		for _, assertion := range assertions {
			appendVersion(assertion.version)
		}
	}

	queries := make([]containmentQuery, 0, len(ranges)*len(versions))
	for _, nativeRange := range ranges {
		for _, version := range versions {
			queries = append(queries, containmentQuery{Range: nativeRange, Version: version})
		}
	}
	return queries
}

func applyContainmentResults(queries []containmentQuery, results []bool) ([]nativeRangeAssertion, error) {
	if len(results) != len(queries) {
		return nil, fmt.Errorf("reference returned %d results for %d queries", len(results), len(queries))
	}
	assertions := make([]nativeRangeAssertion, 0, len(queries))
	for index, query := range queries {
		assertions = append(assertions, nativeRangeAssertion{
			nativeRange: query.Range,
			version:     query.Version,
			contains:    results[index],
		})
	}
	return assertions, nil
}

func evaluateNodeSemver(checkout string, queries []containmentQuery) ([]bool, error) {
	return evaluateJSONScript("node", ".js", nodeSemverEvaluator, []string{checkout}, queries)
}

func evaluatePyPI(checkout string, queries []containmentQuery) ([]bool, error) {
	return evaluateJSONScript("python3", ".py", pyPIEvaluator, []string{checkout}, queries)
}

func evaluateRubyGems(checkout string, queries []containmentQuery) ([]bool, error) {
	script, cleanup, err := writeEvaluatorScript(".rb", rubyGemsEvaluator)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return runJSONCommand("", "ruby", []string{"--disable-gems", "-I", filepath.Join(checkout, "lib"), script}, queries)
}

func evaluateComposer(checkout string, queries []containmentQuery) ([]bool, error) {
	return evaluateJSONScript("php", ".php", composerEvaluator, []string{checkout}, queries)
}

func evaluatePub(checkout string, queries []containmentQuery) ([]bool, error) {
	directory, err := os.MkdirTemp("", "vers-harvest-pub-")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(directory)
	}()

	packagePath := filepath.ToSlash(filepath.Join(checkout, "pkgs", "pub_semver"))
	pubspec := "name: vers_harvest_reference\nenvironment:\n  sdk: '>=3.4.0 <4.0.0'\ndependencies:\n  pub_semver:\n    path: '" + strings.ReplaceAll(packagePath, "'", "''") + "'\n"
	if err := os.WriteFile(filepath.Join(directory, "pubspec.yaml"), []byte(pubspec), fileMode); err != nil {
		return nil, err
	}
	binDirectory := filepath.Join(directory, "bin")
	if err := os.MkdirAll(binDirectory, directoryMode); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(binDirectory, "evaluate.dart"), []byte(pubEvaluator), fileMode); err != nil {
		return nil, err
	}
	if err := runReferenceCommand(directory, "dart", "pub", "get"); err != nil {
		return nil, err
	}
	return runJSONCommand(directory, "dart", []string{"run", "bin/evaluate.dart"}, queries)
}

func evaluateCargo(checkout string, queries []containmentQuery) ([]bool, error) {
	directory, err := os.MkdirTemp("", "vers-harvest-cargo-")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(directory)
	}()

	library := filepath.Join(directory, "libsemver.rlib")
	if err := runReferenceCommand("", "rustc", "--crate-name", "semver", "--crate-type", "lib", "--edition=2021", "--cfg", `feature="std"`, filepath.Join(checkout, "src", "lib.rs"), "-o", library); err != nil {
		return nil, err
	}

	var source strings.Builder
	source.WriteString("use semver::{Version, VersionReq};\nfn main() {\nlet queries = &[\n")
	for _, query := range queries {
		fmt.Fprintf(&source, "(%s, %s),\n", strconv.QuoteToASCII(query.Range), strconv.QuoteToASCII(query.Version))
	}
	source.WriteString("];\nprint!(\"[\");\nfor (index, (range, version)) in queries.iter().enumerate() {\nif index > 0 { print!(\",\"); }\nlet result = VersionReq::parse(range).unwrap().matches(&Version::parse(version).unwrap());\nprint!(\"{}\", result);\n}\nprintln!(\"]\");\n}\n")
	sourceFile := filepath.Join(directory, "evaluate.rs")
	if err := os.WriteFile(sourceFile, []byte(source.String()), fileMode); err != nil {
		return nil, err
	}
	executable := filepath.Join(directory, "evaluate")
	if err := runReferenceCommand("", "rustc", "--edition=2021", sourceFile, "--extern", "semver="+library, "-o", executable); err != nil {
		return nil, err
	}
	return runJSONCommand("", executable, nil, nil)
}

func evaluateMaven(checkout string, queries []containmentQuery) ([]bool, error) {
	directory, err := os.MkdirTemp("", "vers-harvest-maven-")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(directory)
	}()

	artifactDirectory := filepath.Join(directory, "org", "apache", "maven", "artifact")
	versioningDirectory := filepath.Join(artifactDirectory, "versioning")
	if err := os.MkdirAll(versioningDirectory, directoryMode); err != nil {
		return nil, err
	}
	artifactStub := "package org.apache.maven.artifact; public interface Artifact {}\n"
	defaultArtifactStub := "package org.apache.maven.artifact; public final class DefaultArtifact { public static boolean empty(String value) { return value == null || value.isBlank(); } }\n"
	overConstrainedStub := "package org.apache.maven.artifact.versioning; import org.apache.maven.artifact.Artifact; public class OverConstrainedVersionException extends Exception { public OverConstrainedVersionException(String message, Artifact artifact) { super(message); } }\n"
	stubFiles := []struct {
		filename string
		content  string
	}{
		{filepath.Join(artifactDirectory, "Artifact.java"), artifactStub},
		{filepath.Join(artifactDirectory, "DefaultArtifact.java"), defaultArtifactStub},
		{filepath.Join(versioningDirectory, "OverConstrainedVersionException.java"), overConstrainedStub},
	}
	for _, stub := range stubFiles {
		if err := os.WriteFile(stub.filename, []byte(stub.content), fileMode); err != nil {
			return nil, err
		}
	}

	var source strings.Builder
	source.WriteString("import org.apache.maven.artifact.versioning.DefaultArtifactVersion;\nimport org.apache.maven.artifact.versioning.VersionRange;\npublic final class Evaluate {\npublic static void main(String[] arguments) throws Exception {\nString[][] queries = {\n")
	for _, query := range queries {
		fmt.Fprintf(&source, "{%s, %s},\n", javaString(query.Range), javaString(query.Version))
	}
	source.WriteString("};\nSystem.out.print(\"[\");\nfor (int index = 0; index < queries.length; index++) {\nif (index > 0) System.out.print(\",\");\nboolean result = VersionRange.createFromVersionSpec(queries[index][0]).containsVersion(new DefaultArtifactVersion(queries[index][1]));\nSystem.out.print(result);\n}\nSystem.out.println(\"]\");\n}\n}\n")
	evaluatorFile := filepath.Join(directory, "Evaluate.java")
	if err := os.WriteFile(evaluatorFile, []byte(source.String()), fileMode); err != nil {
		return nil, err
	}

	versioningSource := filepath.Join(checkout, "compat", "maven-artifact", "src", "main", "java", "org", "apache", "maven", "artifact", "versioning")
	classes := filepath.Join(directory, "classes")
	if err := os.MkdirAll(classes, directoryMode); err != nil {
		return nil, err
	}
	sources := []string{
		filepath.Join(artifactDirectory, "Artifact.java"),
		filepath.Join(artifactDirectory, "DefaultArtifact.java"),
		filepath.Join(versioningDirectory, "OverConstrainedVersionException.java"),
		filepath.Join(versioningSource, "ArtifactVersion.java"),
		filepath.Join(versioningSource, "ComparableVersion.java"),
		filepath.Join(versioningSource, "DefaultArtifactVersion.java"),
		filepath.Join(versioningSource, "InvalidVersionSpecificationException.java"),
		filepath.Join(versioningSource, "Restriction.java"),
		filepath.Join(versioningSource, "VersionRange.java"),
		evaluatorFile,
	}
	arguments := append([]string{"-d", classes}, sources...)
	if err := runReferenceCommand("", "javac", arguments...); err != nil {
		return nil, err
	}
	return runJSONCommand("", "java", []string{"-cp", classes, "Evaluate"}, nil)
}

func evaluateJSONScript(runtime, extension, source string, arguments []string, queries []containmentQuery) ([]bool, error) {
	script, cleanup, err := writeEvaluatorScript(extension, source)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return runJSONCommand("", runtime, append([]string{script}, arguments...), queries)
}

func writeEvaluatorScript(extension, source string) (string, func(), error) {
	directory, err := os.MkdirTemp("", "vers-harvest-evaluator-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(directory)
	}
	filename := filepath.Join(directory, "evaluate"+extension)
	if err := os.WriteFile(filename, []byte(source), fileMode); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return filename, cleanup, nil
}

func runJSONCommand(directory, name string, arguments []string, queries []containmentQuery) ([]bool, error) {
	input, err := json.Marshal(queries)
	if err != nil {
		return nil, err
	}
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", command.String(), err, output)
	}
	var results []bool
	if err := json.Unmarshal(bytes.TrimSpace(output), &results); err != nil {
		return nil, fmt.Errorf("decode %s output: %w: %s", name, err, output)
	}
	return results, nil
}

func runReferenceCommand(directory, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", command.String(), err, output)
	}
	return nil
}

func javaString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
