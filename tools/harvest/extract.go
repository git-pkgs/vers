package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const minimumComparisonVersions = 2

var quotedValue = regexp.MustCompile(`['"]([^'"]+)['"]`)

func extractNodeSemver(files map[string]string) ([]comparison, error) {
	var items []comparison
	row := regexp.MustCompile(`^\s*\[\s*'([^']*)'\s*,\s*'([^']*)'(?:\s*,\s*(.*?))?\s*\],?\s*$`)
	for _, filename := range []string{"test/fixtures/comparisons.js", "test/fixtures/equality.js"} {
		for _, line := range strings.Split(files[filename], "\n") {
			match := row.FindStringSubmatch(line)
			if match == nil || strings.Contains(match[3], "true") {
				continue
			}
			result := 0
			if strings.Contains(filename, "comparisons") {
				result = 1
			}
			items = append(items, comparison{left: match[1], right: match[2], result: result})
		}
	}
	return requireComparisons(items)
}

func extractPyPI(files map[string]string) ([]comparison, error) {
	content := files["tests/test_version.py"]
	versions, err := quotedListAfter(content, "VERSIONS = [")
	if err != nil {
		return nil, err
	}
	items := orderedComparisons(versions)
	equality := regexp.MustCompile(`Version\("([^"]+)"\)\s*==\s*Version\("([^"]+)"\)`)
	for _, match := range equality.FindAllStringSubmatch(content, -1) {
		items = append(items, comparison{left: match[1], right: match[2]})
	}
	return requireComparisons(items)
}

func extractRubyGems(files map[string]string) ([]comparison, error) {
	content := files["test/rubygems/test_gem_version.rb"]
	spaceship := regexp.MustCompile(`assert_equal\(\s*(-?1|0),\s*v\("([^"]*)"\)\s*<=>\s*v\("([^"]*)"\)\)`)
	items := make([]comparison, 0)
	for _, match := range spaceship.FindAllStringSubmatch(content, -1) {
		result, _ := strconv.Atoi(match[1])
		items = append(items, comparison{left: match[2], right: match[3], result: result})
	}
	lessThan := regexp.MustCompile(`assert_less_than\s+v\("([^"]*)"\),\s*v\("([^"]*)"\)`)
	for _, match := range lessThan.FindAllStringSubmatch(content, -1) {
		items = append(items, comparison{left: match[1], right: match[2], result: -1})
	}
	return requireComparisons(items)
}

func extractComposer(files map[string]string) ([]comparison, error) {
	content := files["tests/ComparatorTest.php"]
	row := regexp.MustCompile(`array\('([^']*)',\s*'([<>=!]+)',\s*'([^']*)',\s*(true|false)\)`)
	items := make([]comparison, 0)
	for _, match := range row.FindAllStringSubmatch(content, -1) {
		if match[4] == "false" {
			if match[2] == "==" || match[2] == "=" {
				items = append(items, comparison{left: match[1], right: match[3], differentOnly: true})
			}
			continue
		}
		result, ok := strictOperatorResult(match[2])
		if ok {
			items = append(items, comparison{left: match[1], right: match[3], result: result})
		}
	}
	for _, provider := range []struct {
		name   string
		result int
	}{{"greaterThanProvider", 1}, {"lessThanProvider", -1}, {"equalToProvider", 0}} {
		block := functionBlock(content, "function "+provider.name+"()")
		providerRow := regexp.MustCompile(`array\('([^']*)',\s*'([^']*)',\s*(true|false)\)`)
		for _, match := range providerRow.FindAllStringSubmatch(block, -1) {
			if match[3] == "false" {
				if provider.name == "equalToProvider" {
					items = append(items, comparison{left: match[1], right: match[2], differentOnly: true})
				}
				continue
			}
			items = append(items, comparison{left: match[1], right: match[2], result: provider.result})
		}
	}
	return requireComparisons(items)
}

func extractPub(files map[string]string) ([]comparison, error) {
	content := files["pkgs/pub_semver/test/version_test.dart"]
	groupStart := strings.Index(content, "group('comparison'")
	if groupStart < 0 {
		return nil, fmt.Errorf("comparison group not found")
	}
	comparisonGroup := content[groupStart:]
	versions, err := quotedListAfter(comparisonGroup, "var versions = [")
	if err != nil {
		return nil, err
	}
	items := orderedComparisons(versions)
	equalityBlock := functionBlock(comparisonGroup, "test('equality'")
	equality := regexp.MustCompile(`(?s)Version\.parse\('([^']+)'\).*?equals\(Version\.parse\('([^']+)'\)\)`)
	for _, match := range equality.FindAllStringSubmatch(equalityBlock, -1) {
		if strings.Contains(match[0], "isNot") {
			items = append(items, comparison{left: match[1], right: match[2], differentOnly: true})
		} else {
			items = append(items, comparison{left: match[1], right: match[2]})
		}
	}
	return requireComparisons(items)
}

func extractCargo(files map[string]string) ([]comparison, error) {
	content := files["tests/test_version.rs"]
	versions, err := quotedListAfter(content, "fn test_spec_order()")
	if err != nil {
		return nil, err
	}
	items := orderedComparisons(versions)
	lessThan := regexp.MustCompile(`assert!\(version\("([^"]+)"\) < version\("([^"]+)"\)\);`)
	for _, match := range lessThan.FindAllStringSubmatch(content, -1) {
		items = append(items, comparison{left: match[1], right: match[2], result: -1})
	}
	notEqual := regexp.MustCompile(`assert_ne!\(version\("([^"]+)"\), version\("([^"]+)"\)\);`)
	for _, match := range notEqual.FindAllStringSubmatch(content, -1) {
		items = append(items, comparison{left: match[1], right: match[2], differentOnly: true})
	}
	return requireComparisons(items)
}

func extractDpkg(files map[string]string) ([]comparison, error) {
	content := files["lib/dpkg/t/t-version.c"]
	assignment := `DPKG_VERSION_OBJECT\((\d+),\s*"([^"]*)",\s*"([^"]*)"\)`
	pattern := regexp.MustCompile(`(?s)a = ` + assignment + `;\s*b = ` + assignment + `;\s*test_pass\(dpkg_version_compare\(&a, &b\)\s*(==|<|>)\s*0\);`)
	items := make([]comparison, 0)
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		left := debianVersion(match[1], match[2], match[3])
		right := debianVersion(match[4], match[5], match[6])
		result, _ := strictOperatorResult(match[7])
		items = append(items, comparison{left: left, right: right, result: result})
	}
	return requireComparisons(items)
}

func extractRPM(files map[string]string) ([]comparison, error) {
	content := files["tests/rpmvercmp.at"]
	row := regexp.MustCompile(`(?m)^RPMVERCMP\(([^,]*),\s*([^,]*),\s*(-?1|0)\)$`)
	items := make([]comparison, 0)
	for _, match := range row.FindAllStringSubmatch(content, -1) {
		result, _ := strconv.Atoi(match[3])
		items = append(items, comparison{left: strings.TrimSpace(match[1]), right: strings.TrimSpace(match[2]), result: result})
	}
	return requireComparisons(items)
}

func extractGo(files map[string]string) ([]comparison, error) {
	content := files["semver/semver_test.go"]
	start := strings.Index(content, "var tests = []struct")
	if start < 0 {
		return nil, fmt.Errorf("tests table not found")
	}
	end := strings.Index(content[start:], "\n}\n")
	if end < 0 {
		return nil, fmt.Errorf("tests table not found")
	}
	table := content[start : start+end]
	row := regexp.MustCompile(`\{"([^"]*)",\s*"([^"]*)"\}`)
	matches := row.FindAllStringSubmatch(table, -1)
	items := make([]comparison, 0, len(matches))
	for index := 1; index < len(matches); index++ {
		result := -1
		if matches[index-1][2] == matches[index][2] {
			result = 0
		}
		items = append(items, comparison{left: matches[index-1][1], right: matches[index][1], result: result})
	}
	return requireComparisons(items)
}

func quotedListAfter(content, marker string) ([]string, error) {
	start := strings.Index(content, marker)
	if start < 0 {
		return nil, fmt.Errorf("marker %q not found", marker)
	}
	start += len(marker)
	end := strings.IndexByte(content[start:], ']')
	if end < 0 {
		return nil, fmt.Errorf("list after %q is not closed", marker)
	}
	matches := quotedValue.FindAllStringSubmatch(content[start:start+end], -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	if len(values) < minimumComparisonVersions {
		return nil, fmt.Errorf("list after %q has fewer than two values", marker)
	}
	return values, nil
}

func orderedComparisons(versions []string) []comparison {
	items := make([]comparison, 0, len(versions)-1)
	for index := 1; index < len(versions); index++ {
		items = append(items, comparison{left: versions[index-1], right: versions[index], result: -1})
	}
	return items
}

func strictOperatorResult(operator string) (int, bool) {
	switch operator {
	case "<":
		return -1, true
	case ">":
		return 1, true
	case "==", "=":
		return 0, true
	default:
		return 0, false
	}
}

func functionBlock(content, marker string) string {
	start := strings.Index(content, marker)
	if start < 0 {
		return ""
	}
	rest := content[start+len(marker):]
	end := strings.Index(rest, "\n    }")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
}

func debianVersion(epoch, version, revision string) string {
	result := version
	if epoch != "0" {
		result = epoch + ":" + result
	}
	if revision != "" {
		result += "-" + revision
	}
	return result
}
