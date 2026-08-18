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

func extractNodeSemverRanges(files map[string]string) ([]nativeRangeAssertion, error) {
	row := regexp.MustCompile(`^\s*\[\s*'([^']*)'\s*,\s*'([^']*)'(?:\s*,\s*(.*?))?\s*\],?(?:\s*//.*)?$`)
	var assertions []nativeRangeAssertion
	for _, source := range []struct {
		filename string
		contains bool
	}{
		{"test/fixtures/range-include.js", true},
		{"test/fixtures/range-exclude.js", false},
	} {
		for _, line := range strings.Split(files[source.filename], "\n") {
			match := row.FindStringSubmatch(line)
			if match == nil || !supportedNodeSemverOptions(match[3]) {
				continue
			}
			assertions = append(assertions, nativeRangeAssertion{
				nativeRange: decodeJavaScriptString(match[1]),
				version:     decodeJavaScriptString(match[2]),
				contains:    source.contains,
			})
		}
	}
	return requireRangeAssertions(assertions)
}

func decodeJavaScriptString(value string) string {
	decoded, err := strconv.Unquote(`"` + strings.ReplaceAll(value, `"`, `\"`) + `"`)
	if err != nil {
		return value
	}
	return decoded
}

func supportedNodeSemverOptions(options string) bool {
	switch strings.TrimSpace(options) {
	case "", "{}", "{ loose: false }", "{ loose: null }", "{ loose: 0 }", "{ loose: undefined }":
		return true
	default:
		return false
	}
}

func extractPyPIRanges(files map[string]string) ([]nativeRangeAssertion, error) {
	content := files["tests/test_specifiers.py"]
	start := strings.Index(content, `("version", "spec_str", "expected")`)
	if start < 0 {
		return nil, fmt.Errorf("specifier containment table not found")
	}
	end := strings.Index(content[start:], "def test_specifiers(")
	if end < 0 {
		return nil, fmt.Errorf("specifier containment table is not closed")
	}
	block := content[start : start+end]
	falseStart := strings.Index(block, "(v, s, False)")
	if falseStart < 0 {
		return nil, fmt.Errorf("false specifier cases not found")
	}
	pair := regexp.MustCompile(`\("([^"]*)",\s*"([^"]*)"\)`)
	assertions := make([]nativeRangeAssertion, 0)
	for _, group := range []struct {
		content  string
		contains bool
	}{
		{block[:falseStart], true},
		{block[falseStart:], false},
	} {
		for _, match := range pair.FindAllStringSubmatch(group.content, -1) {
			assertions = append(assertions, nativeRangeAssertion{
				nativeRange: match[2], version: match[1], contains: group.contains,
			})
		}
	}
	return requireRangeAssertions(assertions)
}

func extractRubyGemsRanges(files map[string]string) ([]nativeRangeAssertion, error) {
	content := files["test/rubygems/test_gem_requirement.rb"]
	row := regexp.MustCompile(`(?m)^\s*(assert|refute)_satisfied_by\s+"([^"]*)",\s+"([^"]*)"`)
	assertions := make([]nativeRangeAssertion, 0)
	for _, match := range row.FindAllStringSubmatch(content, -1) {
		assertions = append(assertions, nativeRangeAssertion{
			nativeRange: match[3], version: match[2], contains: match[1] == "assert",
		})
	}
	return requireRangeAssertions(assertions)
}

func extractComposerRanges(files map[string]string) ([]nativeRangeAssertion, error) {
	content := files["tests/VersionParserTest.php"]
	rowStart := regexp.MustCompile(`array\('([^']*)'`)
	constraint := regexp.MustCompile(`new Constraint\('([^']+)',\s*'([^']+)'\)`)
	assertions := make([]nativeRangeAssertion, 0)
	for _, provider := range []string{
		"simpleConstraints", "wildcardConstraints", "tildeConstraints", "caretConstraints", "hyphenConstraints",
	} {
		block := functionBlock(content, "function "+provider+"()")
		for _, line := range strings.Split(block, "\n") {
			input := rowStart.FindStringSubmatch(line)
			if input == nil {
				continue
			}
			for _, match := range constraint.FindAllStringSubmatch(line, -1) {
				assertions = append(assertions, nativeRangeAssertion{
					nativeRange: input[1], version: match[2], contains: composerConstraintContainsBound(match[1]),
				})
			}
		}
	}
	return requireRangeAssertions(assertions)
}

func composerConstraintContainsBound(operator string) bool {
	switch operator {
	case "=", "==", ">=", "<=":
		return true
	default:
		return false
	}
}

func extractPubRanges(files map[string]string) ([]nativeRangeAssertion, error) {
	content := files["pkgs/pub_semver/test/version_constraint_test.dart"]
	assignment := regexp.MustCompile(`var constraint = VersionConstraint\.parse\('([^']+)'\);`)
	matches := assignment.FindAllStringSubmatchIndex(content, -1)
	assertions := make([]nativeRangeAssertion, 0)
	version := regexp.MustCompile(`Version\.parse\('([^']+)'\)`)
	for index, match := range matches {
		end := len(content)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		if testEnd := strings.Index(content[match[1]:end], "\n    test("); testEnd >= 0 {
			end = match[1] + testEnd
		}
		block := content[match[1]:end]
		nativeRange := content[match[2]:match[3]]
		for _, call := range balancedCalls(block, "allows(") {
			for _, parsed := range version.FindAllStringSubmatch(call, -1) {
				assertions = append(assertions, nativeRangeAssertion{
					nativeRange: nativeRange, version: parsed[1], contains: true,
				})
			}
		}
		for _, call := range balancedCalls(block, "doesNotAllow(") {
			for _, parsed := range version.FindAllStringSubmatch(call, -1) {
				assertions = append(assertions, nativeRangeAssertion{
					nativeRange: nativeRange, version: parsed[1], contains: false,
				})
			}
		}
	}
	constructorAssertions, err := extractPubVersionRangeAssertions(files)
	if err != nil {
		return nil, err
	}
	assertions = append(assertions, constructorAssertions...)
	return requireRangeAssertions(assertions)
}

func extractPubVersionRangeAssertions(files map[string]string) ([]nativeRangeAssertion, error) {
	content := files["pkgs/pub_semver/test/version_range_test.dart"]
	utilityContent := files["pkgs/pub_semver/test/utils.dart"]
	constant := regexp.MustCompile(`final (v\d+) = Version\.parse\('([^']+)'\);`)
	versions := make(map[string]string)
	for _, match := range constant.FindAllStringSubmatch(utilityContent, -1) {
		versions[match[1]] = match[2]
	}
	assignment := regexp.MustCompile(`(?s)var range = VersionRange\((.*?)\);`)
	matches := assignment.FindAllStringSubmatchIndex(content, -1)
	version := regexp.MustCompile(`Version\.parse\('([^']+)'\)`)
	assertions := make([]nativeRangeAssertion, 0)
	for index, match := range matches {
		end := len(content)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		if testEnd := strings.Index(content[match[1]:end], "\n    test("); testEnd >= 0 {
			end = match[1] + testEnd
		}
		nativeRange, err := pubConstructorRange(content[match[2]:match[3]], versions)
		if err != nil {
			return nil, err
		}
		block := content[match[1]:end]
		for _, call := range balancedCalls(block, "allows(") {
			for _, parsed := range version.FindAllStringSubmatch(call, -1) {
				assertions = append(assertions, nativeRangeAssertion{
					nativeRange: nativeRange, version: parsed[1], contains: true,
				})
			}
		}
		for _, call := range balancedCalls(block, "doesNotAllow(") {
			for _, parsed := range version.FindAllStringSubmatch(call, -1) {
				assertions = append(assertions, nativeRangeAssertion{
					nativeRange: nativeRange, version: parsed[1], contains: false,
				})
			}
		}
	}
	return assertions, nil
}

func pubConstructorRange(arguments string, versions map[string]string) (string, error) {
	bound := regexp.MustCompile(`(?:min|max):\s*(Version\.parse\('[^']+'\)|v\d+)`)
	var minimum, maximum string
	for _, match := range bound.FindAllStringSubmatch(arguments, -1) {
		value, err := pubVersionExpression(match[1], versions)
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(strings.TrimSpace(match[0]), "min:") {
			minimum = value
		} else {
			maximum = value
		}
	}
	var constraints []string
	if minimum != "" {
		operator := ">"
		if strings.Contains(arguments, "includeMin: true") {
			operator = ">="
		}
		constraints = append(constraints, operator+minimum)
	}
	if maximum != "" {
		operator := "<"
		if strings.Contains(arguments, "includeMax: true") {
			operator = "<="
		}
		constraints = append(constraints, operator+maximum)
	}
	if len(constraints) == 0 {
		return "any", nil
	}
	return strings.Join(constraints, " "), nil
}

func pubVersionExpression(expression string, versions map[string]string) (string, error) {
	expression = strings.TrimSpace(expression)
	if strings.HasPrefix(expression, "Version.parse('") {
		return strings.TrimSuffix(strings.TrimPrefix(expression, "Version.parse('"), "')"), nil
	}
	if version, ok := versions[expression]; ok {
		return version, nil
	}
	return "", fmt.Errorf("unknown Pub version expression %q", expression)
}

func extractCargoRanges(files map[string]string) ([]nativeRangeAssertion, error) {
	content := files["tests/test_version_req.rs"]
	rangeStart := regexp.MustCompile(`let ref r = req\("([^"]*)"\);`)
	assertion := regexp.MustCompile(`(?s)assert_match_(all|none)\(\s*r,\s*&\[(.*?)\]\s*,?\s*\);`)
	value := regexp.MustCompile(`"([^"]+)"`)
	assertions := make([]nativeRangeAssertion, 0)
	ranges := rangeStart.FindAllStringSubmatchIndex(content, -1)
	for index, item := range ranges {
		end := len(content)
		if index+1 < len(ranges) {
			end = ranges[index+1][0]
		}
		if functionEnd := strings.Index(content[item[1]:end], "\n}"); functionEnd >= 0 {
			end = item[1] + functionEnd
		}
		block := content[item[1]:end]
		nativeRange := content[item[2]:item[3]]
		for _, match := range assertion.FindAllStringSubmatch(block, -1) {
			for _, version := range value.FindAllStringSubmatch(match[2], -1) {
				assertions = append(assertions, nativeRangeAssertion{
					nativeRange: nativeRange, version: version[1], contains: match[1] == "all",
				})
			}
		}
	}
	return requireRangeAssertions(assertions)
}

func extractMavenRanges(files map[string]string) ([]nativeRangeAssertion, error) {
	content := files["compat/maven-artifact/src/test/java/org/apache/maven/artifact/versioning/VersionRangeTest.java"]
	assignment := regexp.MustCompile(`(?:VersionRange\s+)?range\s*=\s*VersionRange\.createFromVersionSpec\("([^"]+)"\);`)
	containment := regexp.MustCompile(`assert(True|False)\(range\.containsVersion\(new DefaultArtifactVersion\("([^"]+)"\)\)\);`)
	assertions := make([]nativeRangeAssertion, 0)
	currentRange := ""
	for _, line := range strings.Split(content, "\n") {
		if match := assignment.FindStringSubmatch(line); match != nil {
			currentRange = match[1]
		}
		if match := containment.FindStringSubmatch(line); match != nil && currentRange != "" {
			assertions = append(assertions, nativeRangeAssertion{
				nativeRange: currentRange, version: match[2], contains: match[1] == "True",
			})
		}
	}
	return requireRangeAssertions(assertions)
}

func balancedCalls(content, marker string) []string {
	var calls []string
	for offset := 0; offset < len(content); {
		relative := strings.Index(content[offset:], marker)
		if relative < 0 {
			break
		}
		start := offset + relative
		if start > 0 && content[start-1] == '.' {
			offset = start + len(marker)
			continue
		}
		open := start + len(marker) - 1
		depth := 0
		end := -1
		for index := open; index < len(content); index++ {
			switch content[index] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					end = index
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			break
		}
		calls = append(calls, content[open+1:end])
		offset = end + 1
	}
	return calls
}

func requireRangeAssertions(assertions []nativeRangeAssertion) ([]nativeRangeAssertion, error) {
	if len(assertions) == 0 {
		return nil, fmt.Errorf("no native range assertions found")
	}
	return assertions, nil
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
