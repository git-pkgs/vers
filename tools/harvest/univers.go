package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	npmAdvisorySource        = "tests/data/npm_advisory.json"
	universAdditionalOutputs = 2
	alpineComparisonFields   = 3
)

func generateUnivers(files map[string]string, outputRoot string) ([]string, error) {
	testsDir := filepath.Join(outputRoot, "tests")
	comparisonFixtures := []struct {
		filename string
		name     string
		scheme   string
		extract  func(map[string]string) ([]comparison, error)
	}{
		{"deb_univers_version_cmp_test.json", "univers Debian", "deb", extractUniversDebian},
		{"rpm_univers_version_cmp_test.json", "univers RPM", "rpm", extractUniversRPM},
		{"gentoo_univers_version_cmp_test.json", "univers Gentoo", "gentoo", extractUniversGentoo},
		{"apk_univers_version_cmp_test.json", "univers Alpine", "apk", extractUniversAlpine},
		{"alpm_univers_version_cmp_test.json", "univers Pacman", "alpm", extractUniversPacman},
		{"conan_univers_version_cmp_test.json", "univers Conan", "conan", extractUniversConan},
	}
	generated := make([]string, 0, len(comparisonFixtures)+universAdditionalOutputs)
	for _, fixture := range comparisonFixtures {
		comparisons, err := fixture.extract(files)
		if err != nil {
			return nil, err
		}
		source := sourceSpec{name: fixture.name, scheme: fixture.scheme}
		if err := writeJSON(filepath.Join(testsDir, fixture.filename), buildTestFile(source, comparisons)); err != nil {
			return nil, err
		}
		generated = append(generated, filepath.ToSlash(filepath.Join("tests", fixture.filename)))
	}

	assertions, err := extractUniversConanRanges(files)
	if err != nil {
		return nil, err
	}
	conanSource := sourceSpec{name: "univers Conan", scheme: "conan"}
	conanFile, err := buildRangeTestFile(conanSource, assertions)
	if err != nil {
		return nil, err
	}
	const conanFilename = "conan_univers_range_test.json"
	if err := writeJSON(filepath.Join(testsDir, conanFilename), conanFile); err != nil {
		return nil, err
	}
	generated = append(generated, filepath.ToSlash(filepath.Join("tests", conanFilename)))

	dataDirectory := filepath.Join(outputRoot, "data")
	if err := os.MkdirAll(dataDirectory, directoryMode); err != nil {
		return nil, err
	}
	advisory := files[npmAdvisorySource]
	if !strings.HasSuffix(advisory, "\n") {
		advisory += "\n"
	}
	const advisoryFilename = "npm_advisory.json"
	if err := os.WriteFile(filepath.Join(dataDirectory, advisoryFilename), []byte(advisory), fileMode); err != nil {
		return nil, err
	}
	generated = append(generated, filepath.ToSlash(filepath.Join("data", advisoryFilename)))

	return generated, nil
}

func extractUniversDebian(files map[string]string) ([]comparison, error) {
	content := files["tests/test_debian_version.py"]
	pattern := regexp.MustCompile(`compare_versions\(\s*"([^"]*)",\s*"([^"]*)"\s*\)\s*==\s*(-?1|0)`)
	items := make([]comparison, 0)
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		result, _ := strconv.Atoi(match[3])
		items = append(items, comparison{left: match[1], right: match[2], result: result})
	}
	return requireComparisons(items)
}

func extractUniversRPM(files map[string]string) ([]comparison, error) {
	content := files["tests/data/rpmvercmp.at"]
	pattern := regexp.MustCompile(`(?m)^\s*(?:dnl\s+)?RPMVERCMP\(([^,]*),\s*([^,]*),\s*(-?1|0)\)`)
	items := make([]comparison, 0)
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		result, _ := strconv.Atoi(match[3])
		items = append(items, comparison{
			left: strings.TrimSpace(match[1]), right: strings.TrimSpace(match[2]), result: result,
		})
	}
	return requireComparisons(items)
}

func extractUniversGentoo(files map[string]string) ([]comparison, error) {
	content := files["tests/test_gentoo_pkgcore.py"]
	pattern := regexp.MustCompile(`assert\s+(older|newer|equal)\(f?"([^"{}]+)",\s*f?"([^"{}]+)"\)`)
	items := make([]comparison, 0)
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		result := 0
		switch match[1] {
		case "older":
			result = -1
		case "newer":
			result = 1
		}
		items = append(items, comparison{left: match[2], right: match[3], result: result})
	}
	notEqual := regexp.MustCompile(`assert\s+not\s+equal\(f?"([^"{}]+)",\s*f?"([^"{}]+)"\)`)
	for _, match := range notEqual.FindAllStringSubmatch(content, -1) {
		items = append(items, comparison{left: match[1], right: match[2], differentOnly: true})
	}
	return requireComparisons(items)
}

func extractUniversAlpine(files map[string]string) ([]comparison, error) {
	content := files["tests/data/alpine_test.txt"]
	items := make([]comparison, 0)
	for lineNumber, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != alpineComparisonFields {
			return nil, fmt.Errorf("parse Alpine comparison line %d: %q", lineNumber+1, line)
		}
		result, ok := strictOperatorResult(fields[1])
		if !ok {
			return nil, fmt.Errorf("parse Alpine comparison operator %q", fields[1])
		}
		items = append(items, comparison{left: fields[0], right: fields[2], result: result})
	}
	return requireComparisons(items)
}

func extractUniversPacman(files map[string]string) ([]comparison, error) {
	content := files["tests/test_pacman_vercmp.py"]
	pattern := regexp.MustCompile(`(?m)^\s*assert\s+ArchLinuxVersion\("([^"]+)"\)\s*(==|<|>)\s*ArchLinuxVersion\("([^"]+)"\)`)
	items := make([]comparison, 0)
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		result, _ := strictOperatorResult(match[2])
		items = append(items, comparison{left: match[1], right: match[3], result: result})
	}
	return requireComparisons(items)
}

func extractUniversConan(files map[string]string) ([]comparison, error) {
	content := files["tests/test_conan_version_comparison.py"]
	lessThan, err := pythonAssignedList(content, "v")
	if err != nil {
		return nil, err
	}
	equal, err := pythonAssignedList(content, "e")
	if err != nil {
		return nil, err
	}
	pair := regexp.MustCompile(`\(\s*"([^"]+)"\s*,\s*"([^"]+)"\s*\)`)
	items := make([]comparison, 0)
	for _, match := range pair.FindAllStringSubmatch(lessThan, -1) {
		items = append(items, comparison{left: match[1], right: match[2], result: -1})
	}
	for _, match := range pair.FindAllStringSubmatch(equal, -1) {
		items = append(items, comparison{left: match[1], right: match[2]})
	}
	return requireComparisons(items)
}

func extractUniversConanRanges(files map[string]string) ([]nativeRangeAssertion, error) {
	content := files["tests/test_conan_version_range.py"]
	list, err := pythonAssignedList(content, "values")
	if err != nil {
		return nil, err
	}
	records, err := splitPythonList(list)
	if err != nil {
		return nil, err
	}
	assertions := make([]nativeRangeAssertion, 0)
	for _, record := range records {
		fields, splitErr := splitPythonList(record)
		if splitErr != nil || len(fields) != 4 {
			return nil, fmt.Errorf("parse Conan range record %q", record)
		}
		nativeRange, stringErr := pythonString(fields[0])
		if stringErr != nil {
			return nil, stringErr
		}
		// Conan's empty range has implicit stable-only behavior, and the
		// include_prerelease option is configuration rather than range syntax.
		if nativeRange == "" || strings.Contains(nativeRange, "include_prerelease=") {
			continue
		}
		for _, group := range []struct {
			field    string
			contains bool
		}{{fields[2], true}, {fields[3], false}} {
			for _, match := range quotedValue.FindAllStringSubmatch(group.field, -1) {
				assertions = append(assertions, nativeRangeAssertion{
					nativeRange: nativeRange, version: match[1], contains: group.contains,
				})
			}
		}
	}
	return requireRangeAssertions(assertions)
}

func pythonAssignedList(content, name string) (string, error) {
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*=\s*\[`)
	location := pattern.FindStringIndex(content)
	if location == nil {
		return "", fmt.Errorf("python list %q not found", name)
	}
	start := location[1] - 1
	end, err := matchingPythonBracket(content, start)
	if err != nil {
		return "", fmt.Errorf("parse Python list %q: %w", name, err)
	}
	return content[start : end+1], nil
}

func matchingPythonBracket(content string, start int) (int, error) {
	depth := 0
	var quote byte
	escaped := false
	comment := false
	for index := start; index < len(content); index++ {
		character := content[index]
		if comment {
			if character == '\n' {
				comment = false
			}
			continue
		}
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case character == '\\':
				escaped = true
			case character == quote:
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
		case '#':
			comment = true
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
			if depth == 0 {
				return index, nil
			}
		}
	}
	return 0, fmt.Errorf("unclosed bracket at offset %d", start)
}

func splitPythonList(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil, fmt.Errorf("not a Python list: %q", value)
	}
	content := stripPythonComments(value[1 : len(value)-1])
	items := make([]string, 0)
	start := 0
	depth := 0
	var quote byte
	escaped := false
	comment := false
	for index := 0; index < len(content); index++ {
		character := content[index]
		if comment {
			if character == '\n' {
				comment = false
			}
			continue
		}
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case character == '\\':
				escaped = true
			case character == quote:
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
		case '#':
			comment = true
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
		case ',':
			if depth == 0 {
				if item := strings.TrimSpace(content[start:index]); item != "" {
					items = append(items, item)
				}
				start = index + 1
			}
		}
	}
	if quote != 0 || depth != 0 {
		return nil, fmt.Errorf("unbalanced Python list: %q", value)
	}
	if item := strings.TrimSpace(content[start:]); item != "" {
		items = append(items, item)
	}
	return items, nil
}

func stripPythonComments(content string) string {
	var result strings.Builder
	result.Grow(len(content))
	var quote byte
	escaped := false
	comment := false
	for index := 0; index < len(content); index++ {
		character := content[index]
		if comment {
			if character == '\n' {
				comment = false
				result.WriteByte(character)
			}
			continue
		}
		if quote != 0 {
			result.WriteByte(character)
			switch {
			case escaped:
				escaped = false
			case character == '\\':
				escaped = true
			case character == quote:
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
			result.WriteByte(character)
		case '#':
			comment = true
		default:
			result.WriteByte(character)
		}
	}
	return result.String()
}

func pythonString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != value[len(value)-1] || value[0] != '"' && value[0] != '\'' {
		return "", fmt.Errorf("not a Python string: %q", value)
	}
	if value[0] == '"' {
		return strconv.Unquote(value)
	}
	inner := value[1 : len(value)-1]
	inner = strings.ReplaceAll(inner, `\\`, `\`)
	inner = strings.ReplaceAll(inner, `\'`, `'`)
	return inner, nil
}
