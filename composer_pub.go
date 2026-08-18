package vers

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	composerAliasRegex         = regexp.MustCompile(`(?i)^(.+?)\s+as\s+.+$`)
	composerHyphenRangeRegex   = regexp.MustCompile(`^\s*(\S+)\s+-\s+(\S+)\s*$`)
	composerNumericBranchRegex = regexp.MustCompile(`(?i)^v?\d+(?:\.\d+)*\.x-dev$`)
	composerOrRegex            = regexp.MustCompile(`\s*\|\|?\s*`)
	composerStabilityFlagRegex = regexp.MustCompile(`(?i)^(.*?)@(dev|alpha|beta|rc|stable)$`)
	composerVersionRegex       = regexp.MustCompile(`(?i)^v?([0-9]+(?:\.[0-9]+){0,3})(?:[-._]?([a-z]+)(?:[.-]?([0-9]+(?:[.-][0-9]+)*))?)?(?:\+[^\s]+)?$`)
	pubVersionPrefixRegex      = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?`)
)

const (
	composerStabilityDev = iota
	composerStabilityAlpha
	composerStabilityBeta
	composerStabilityRC
	composerStabilityStable
	composerStabilityPatch
)

const (
	composerTildeBumpOffset = 2
	composerVersionParts    = 4
	composerQualifierDev    = "dev"
	composerQualifierPatch  = "patch"
	composerQualifierStable = "stable"
)

type composerVersion struct {
	core      [4]string
	stability int
	number    string
}

// parseComposerRange parses Composer's AND, OR, alias and stability syntax.
func (p *Parser) parseComposerRange(constraint string) (*Range, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "*" {
		return rangeWithScheme(Unbounded(), schemeComposer), nil
	}
	if constraint == "" {
		return nil, fmt.Errorf("empty composer constraint")
	}

	parts := composerOrRegex.Split(constraint, -1)
	if len(parts) == 1 {
		return p.parseComposerConjunction(parts[0])
	}
	var result *Range
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("invalid composer OR constraint: %s", constraint)
		}
		r, err := p.parseComposerConjunction(part)
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = r
		} else {
			result = result.Union(r)
		}
	}
	return result, nil
}

// parseComposerConjunction parses a single Composer AND expression.
func (p *Parser) parseComposerConjunction(constraint string) (*Range, error) {
	constraint = strings.TrimSpace(constraint)
	if match := composerAliasRegex.FindStringSubmatch(constraint); match != nil {
		constraint = strings.TrimSpace(match[1])
	}
	if match := composerHyphenRangeRegex.FindStringSubmatch(constraint); match != nil {
		return parseComposerHyphenRange(match[1], match[2])
	}

	tokens := tokenizeNpmConstraints(strings.ReplaceAll(constraint, ",", " "))
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty composer constraint")
	}
	var result *Range
	for _, token := range tokens {
		r, err := p.parseComposerConstraint(token)
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = r
		} else {
			result = result.Intersect(r)
		}
	}
	return result, nil
}

// parseComposerConstraint parses one Composer constraint without boolean
// operators.
func (p *Parser) parseComposerConstraint(constraint string) (*Range, error) {
	constraint, stability := splitComposerStabilityFlag(constraint)
	if constraint == "" {
		constraint = "*"
	}
	if strings.HasPrefix(constraint, "==") {
		constraint = constraint[1:]
	} else if strings.HasPrefix(constraint, "<>") {
		constraint = "!=" + constraint[2:]
	}
	if operator, version := extractOperator(constraint); operator != "" && isComposerWildcard(strings.TrimSpace(version)) {
		return nil, fmt.Errorf("composer wildcard cannot be combined with %s", operator)
	}
	switch {
	case constraint == "*":
		return rangeWithScheme(Unbounded(), schemeComposer), nil
	case strings.HasPrefix(constraint, "^"):
		return parseComposerCaretRange(strings.TrimSpace(constraint[1:]))
	case strings.HasPrefix(constraint, "~"):
		return parseComposerTildeRange(strings.TrimSpace(constraint[1:]))
	case isComposerWildcard(constraint):
		return parseComposerWildcardRange(constraint)
	}

	parsed, err := parseConstraintWithScheme(constraint, schemeComposer)
	if err != nil {
		return nil, err
	}
	if !validComposerVersion(parsed.Version) {
		return nil, fmt.Errorf("invalid composer version: %s", parsed.Version)
	}
	implicitStability := ""
	if parsed.Operator != "=" && parsed.Operator != "!=" {
		implicitStability = stability
	}
	if implicitStability == "" && (parsed.Operator == "<" || parsed.Operator == ">=") {
		implicitStability = composerQualifierDev
	}
	if normalized, ok := canonicalComposerNumericVersion(parsed.Version, implicitStability); ok {
		parsed.Version = normalized
	}
	if parsed.IsExclusion() {
		return rangeWithScheme(Unbounded().Exclude(parsed.Version), schemeComposer), nil
	}
	interval, ok := parsed.ToInterval()
	if !ok {
		return nil, fmt.Errorf("invalid composer constraint: %s", constraint)
	}
	return rangeWithScheme(NewRange([]Interval{interval}), schemeComposer), nil
}

// splitComposerStabilityFlag removes a trailing stability flag and returns
// its value. Stable is the default, so an explicit stable flag is empty.
func splitComposerStabilityFlag(constraint string) (string, string) {
	constraint = strings.TrimSpace(constraint)
	match := composerStabilityFlagRegex.FindStringSubmatch(constraint)
	if match == nil {
		return constraint, ""
	}
	stability := strings.ToLower(match[2])
	if stability == composerQualifierStable {
		stability = ""
	}
	return strings.TrimSpace(match[1]), stability
}

// parseComposerCaretRange expands a Composer caret constraint and uses a dev
// upper bound so prereleases of the next breaking version are excluded.
func parseComposerCaretRange(version string) (*Range, error) {
	if lower, parts, ok := composerDevWildcardBound(version); ok {
		upper := incrementComposerRelease(parts[:1], 0)
		return rangeWithScheme(NewRange([]Interval{
			NewInterval(lower, upper, true, false),
		}), schemeComposer), nil
	}
	parts, ok := composerReleaseParts(version)
	if !ok {
		return nil, fmt.Errorf("invalid composer caret version: %s", version)
	}
	bump := 0
	if cmpNumStr(parts[0], "0") == 0 && len(parts) > 1 {
		bump = 1
		if cmpNumStr(parts[1], "0") == 0 && len(parts) > 2 {
			bump = 2
		}
	}
	upper := incrementComposerRelease(parts, bump)
	lower, _ := canonicalComposerNumericVersion(version, composerQualifierDev)
	return rangeWithScheme(NewRange([]Interval{
		NewInterval(lower, upper, true, false),
	}), schemeComposer), nil
}

// parseComposerTildeRange expands Composer's tilde syntax. One or two release
// components allow the next major; longer versions allow the penultimate
// component to increase.
func parseComposerTildeRange(version string) (*Range, error) {
	if lower, parts, ok := composerDevWildcardBound(version); ok {
		prefixLength := 0
		for prefixLength < len(parts) && parts[prefixLength] != "9999999" {
			prefixLength++
		}
		upperParts := parts[:prefixLength]
		upper := incrementComposerRelease(upperParts, len(upperParts)-1)
		return rangeWithScheme(NewRange([]Interval{
			NewInterval(lower, upper, true, false),
		}), schemeComposer), nil
	}
	parts, ok := composerReleaseParts(version)
	if !ok {
		return nil, fmt.Errorf("invalid composer tilde version: %s", version)
	}
	bump := 0
	if len(parts) > composerTildeBumpOffset {
		bump = len(parts) - composerTildeBumpOffset
	}
	upper := incrementComposerRelease(parts, bump)
	lower, _ := canonicalComposerNumericVersion(version, composerQualifierDev)
	return rangeWithScheme(NewRange([]Interval{
		NewInterval(lower, upper, true, false),
	}), schemeComposer), nil
}

// parseComposerWildcardRange expands a Composer wildcard into inclusive and
// exclusive bounds.
func parseComposerWildcardRange(constraint string) (*Range, error) {
	parts, ok := composerWildcardReleaseParts(constraint)
	if !ok {
		return nil, fmt.Errorf("invalid composer wildcard: %s", constraint)
	}
	if len(parts) == 0 {
		return rangeWithScheme(Unbounded(), schemeComposer), nil
	}
	lower := completeComposerRelease(parts) + "-dev"
	upper := incrementComposerRelease(parts, len(parts)-1)
	return rangeWithScheme(NewRange([]Interval{NewInterval(lower, upper, true, false)}), schemeComposer), nil
}

// isComposerWildcard reports whether a constraint consists of numeric and
// trailing x or asterisk components.
func isComposerWildcard(constraint string) bool {
	_, ok := composerWildcardReleaseParts(constraint)
	return ok
}

// composerWildcardReleaseParts returns the numeric prefix before one or more
// trailing Composer wildcard components.
func composerWildcardReleaseParts(constraint string) ([]string, bool) {
	constraint = normalizeComposerLower(constraint)
	if strings.HasPrefix(constraint, "v") || strings.HasPrefix(constraint, "V") {
		constraint = constraint[1:]
	}
	segments := strings.Split(constraint, ".")
	parts := make([]string, 0, len(segments))
	wildcard := false
	for _, segment := range segments {
		if segment == "*" || strings.EqualFold(segment, "x") {
			wildcard = true
			continue
		}
		if wildcard || !isDigits(segment) {
			return nil, false
		}
		parts = append(parts, segment)
	}
	return parts, wildcard
}

// parseComposerHyphenRange expands a Composer hyphen range. A partial upper
// version behaves as a wildcard, while a complete upper version is inclusive.
func parseComposerHyphenRange(lower, upper string) (*Range, error) {
	_, lowerOK := composerReleaseParts(lower)
	min, lowerWildcardParts, lowerWildcard := composerDevWildcardBound(lower)
	if lowerWildcard {
		lowerOK = len(lowerWildcardParts) > 0
	} else {
		min, _ = canonicalComposerNumericVersion(lower, composerQualifierDev)
	}
	upperParts, upperOK := composerReleaseParts(upper)
	max, _, upperWildcard := composerDevWildcardBound(upper)
	if upperWildcard {
		upperOK = true
	}
	if !lowerOK || !upperOK {
		return nil, fmt.Errorf("invalid composer hyphen range: %s - %s", lower, upper)
	}
	if upperWildcard {
		return rangeWithScheme(NewRange([]Interval{NewInterval(min, max, true, true)}), schemeComposer), nil
	}
	if len(upperParts) < 3 && !hasComposerStability(upper) { //nolint:mnd
		max = incrementComposerRelease(upperParts, len(upperParts)-1)
		return rangeWithScheme(NewRange([]Interval{NewInterval(min, max, true, false)}), schemeComposer), nil
	}
	max, _ = canonicalComposerNumericVersion(upper, "")
	return rangeWithScheme(NewRange([]Interval{NewInterval(min, max, true, true)}), schemeComposer), nil
}

func composerDevWildcardBound(version string) (string, []string, bool) {
	version = normalizeComposerLower(version)
	if !strings.HasSuffix(strings.ToLower(version), "-dev") {
		return "", nil, false
	}
	release := version[:len(version)-len("-dev")]
	segments := strings.Split(release, ".")
	parts := make([]string, 0, composerVersionParts)
	wildcard := false
	for _, segment := range segments {
		if segment == "*" || strings.EqualFold(segment, "x") {
			if len(parts) == 0 {
				return "", nil, false
			}
			wildcard = true
			parts = append(parts, "9999999")
			continue
		}
		if wildcard || !isDigits(segment) {
			return "", nil, false
		}
		parts = append(parts, trimLeadingZeros(segment))
	}
	if !wildcard || len(parts) > composerVersionParts {
		return "", nil, false
	}
	for len(parts) < composerVersionParts {
		parts = append(parts, "9999999")
	}
	return strings.Join(parts, ".") + "-dev", parts, true
}

// hasComposerStability reports whether a numeric Composer version has an
// explicit stability suffix.
func hasComposerStability(version string) bool {
	match := composerVersionRegex.FindStringSubmatch(normalizeComposerLower(version))
	return match != nil && match[2] != ""
}

// composerReleaseParts returns the numeric release components of a Composer
// version or constraint bound.
func composerReleaseParts(version string) ([]string, bool) {
	if _, ok := canonicalComposerNumericVersion(version, ""); !ok {
		return nil, false
	}
	match := composerVersionRegex.FindStringSubmatch(normalizeComposerLower(version))
	parts := strings.Split(match[1], ".")
	if len(parts) == 0 || len(parts) > 4 { //nolint:mnd
		return nil, false
	}
	for _, part := range parts {
		if !isDigits(part) {
			return nil, false
		}
	}
	return parts, true
}

// completeComposerRelease pads a numeric Composer release to at least three
// components.
func completeComposerRelease(parts []string) string {
	completed := append([]string(nil), parts...)
	for len(completed) < 3 { //nolint:mnd
		completed = append(completed, "0")
	}
	for i := range completed {
		completed[i] = trimLeadingZeros(completed[i])
	}
	return strings.Join(completed, ".")
}

// incrementComposerRelease increments one component, clears following
// components and returns the exclusive dev boundary used by Composer.
func incrementComposerRelease(parts []string, index int) string {
	length := len(parts)
	if length < 3 { //nolint:mnd
		length = 3
	}
	upper := make([]string, length)
	copy(upper, parts)
	for i := len(parts); i < len(upper); i++ {
		upper[i] = "0"
	}
	upper[index] = incNumStr(upper[index])
	for i := index + 1; i < len(upper); i++ {
		upper[i] = "0"
	}
	return strings.Join(upper, ".") + "-dev"
}

// normalizeComposerLower removes a numeric tag's optional v prefix while
// preserving prerelease and build suffixes.
func normalizeComposerLower(version string) string {
	version = strings.TrimSpace(version)
	if len(version) > 1 && (version[0] == 'v' || version[0] == 'V') && isASCIIDigit(version[1]) {
		return version[1:]
	}
	return version
}

// canonicalComposerNumericVersion formats a numeric Composer version with at
// least three release components and an optional implicit stability.
func canonicalComposerNumericVersion(version, implicitStability string) (string, bool) {
	match := composerVersionRegex.FindStringSubmatch(normalizeComposerLower(version))
	if match == nil {
		return "", false
	}
	result := completeComposerRelease(strings.Split(match[1], "."))
	stability := strings.ToLower(match[2])
	if stability == "" {
		stability = strings.ToLower(implicitStability)
	}
	switch stability {
	case "":
		return result, true
	case "a":
		stability = qualifierAlpha
	case "b":
		stability = qualifierBeta
	case "rc":
		stability = "RC"
	case "p", "pl":
		stability = composerQualifierPatch
	case composerQualifierStable:
		return result, true
	case composerQualifierDev, qualifierAlpha, qualifierBeta, composerQualifierPatch:
	default:
		return "", false
	}
	return result + "-" + stability + match[3], true
}

// parseComposerVersion parses numeric Composer versions and their recognized
// stability suffixes.
func parseComposerVersion(version string) (composerVersion, bool) {
	match := composerVersionRegex.FindStringSubmatch(strings.TrimSpace(version))
	if match == nil {
		return composerVersion{}, false
	}
	parsed := composerVersion{stability: composerStabilityStable}
	parts := strings.Split(match[1], ".")
	for i := range parsed.core {
		parsed.core[i] = "0"
		if i < len(parts) {
			parsed.core[i] = parts[i]
		}
	}
	if match[2] == "" {
		return parsed, true
	}
	switch strings.ToLower(match[2]) {
	case composerQualifierDev:
		parsed.stability = composerStabilityDev
	case "a", qualifierAlpha:
		parsed.stability = composerStabilityAlpha
	case "b", qualifierBeta:
		parsed.stability = composerStabilityBeta
	case "rc":
		parsed.stability = composerStabilityRC
	case composerQualifierStable:
		parsed.stability = composerStabilityStable
	case "p", "pl", composerQualifierPatch:
		parsed.stability = composerStabilityPatch
	default:
		return composerVersion{}, false
	}
	parsed.number = match[3]
	return parsed, true
}

// compareComposer compares numeric Composer versions using Composer's
// stability order.
func compareComposer(a, b string) int {
	left, leftOK := parseComposerVersion(a)
	right, rightOK := parseComposerVersion(b)
	leftBranch := strings.HasPrefix(strings.ToLower(a), "dev-")
	rightBranch := strings.HasPrefix(strings.ToLower(b), "dev-")
	if leftBranch != rightBranch {
		if leftBranch {
			return -1
		}
		return 1
	}
	if !leftOK || !rightOK {
		return cmpString(strings.ToLower(a), strings.ToLower(b))
	}
	for i := range left.core {
		if comparison := cmpNumStr(left.core[i], right.core[i]); comparison != 0 {
			return comparison
		}
	}
	if left.stability != right.stability {
		return cmpInt(left.stability, right.stability)
	}
	return compareComposerNumbers(left.number, right.number)
}

func compareComposerNumbers(a, b string) int {
	left := strings.FieldsFunc(a, func(character rune) bool { return character == '.' || character == '-' })
	right := strings.FieldsFunc(b, func(character rune) bool { return character == '.' || character == '-' })
	for index := 0; index < len(left) || index < len(right); index++ {
		var leftPart, rightPart string
		if index < len(left) {
			leftPart = left[index]
		}
		if index < len(right) {
			rightPart = right[index]
		}
		if comparison := cmpNumStr(leftPart, rightPart); comparison != 0 {
			return comparison
		}
	}
	return 0
}

// validComposerVersion reports whether a version is a numeric Composer version
// or one of Composer's branch version forms.
func validComposerVersion(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" || strings.ContainsAny(version, " \t\r\n") {
		return false
	}
	if _, ok := parseComposerVersion(version); ok {
		return true
	}
	lower := strings.ToLower(version)
	return (strings.HasPrefix(lower, "dev-") && len(version) > len("dev-")) ||
		composerNumericBranchRegex.MatchString(version)
}

// normalizeComposerVersion normalizes SemVer-shaped Composer tags while
// leaving branch and four-component versions intact.
func normalizeComposerVersion(version string) string {
	version = normalizeComposerLower(version)
	if normalized, ok := canonicalComposerNumericVersion(version, ""); ok {
		return normalized
	}
	return version
}

// parsePubRange parses Pub's any, caret and traditional intersection syntax.
func (p *Parser) parsePubRange(constraint string) (*Range, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "any" {
		return rangeWithScheme(Unbounded(), schemePub), nil
	}
	if constraint == "" {
		return nil, fmt.Errorf("empty pub constraint")
	}
	if strings.ContainsAny(constraint, ",|*") || strings.Contains(constraint, "!=") {
		return nil, fmt.Errorf("unsupported pub constraint: %s", constraint)
	}
	if strings.HasPrefix(constraint, "^") {
		version := strings.TrimSpace(constraint[1:])
		if !validPubVersion(version) {
			return nil, fmt.Errorf("invalid pub caret version: %s", version)
		}
		return parsePubCaretRange(version)
	}

	tokens, err := tokenizePubConstraints(constraint)
	if err != nil {
		return nil, err
	}
	var result *Range
	for _, token := range tokens {
		r, err := parsePubConstraint(token)
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = r
		} else {
			result = result.Intersect(r)
		}
	}
	if result == nil {
		return nil, fmt.Errorf("empty pub constraint")
	}
	return adjustPubExclusiveUpperBounds(result), nil
}

// adjustPubExclusiveUpperBounds excludes prereleases of a stable upper bound
// unless the range begins at a prerelease of that same version.
func adjustPubExclusiveUpperBounds(r *Range) *Range {
	for i := range r.Intervals {
		interval := &r.Intervals[i]
		if !pubUpperNeedsFirstPrerelease(*interval) {
			continue
		}
		original := interval.Max
		interval.Max += "-0"
		for j := range r.RawConstraints {
			raw := &r.RawConstraints[j]
			if raw.Max == original && !raw.MaxInclusive {
				raw.Max = interval.Max
			}
		}
	}
	return r
}

// pubUpperNeedsFirstPrerelease reports whether an exclusive stable upper
// bound should move to its first prerelease.
func pubUpperNeedsFirstPrerelease(interval Interval) bool {
	if interval.Max == "" || interval.MaxInclusive || !validPubVersion(interval.Max) {
		return false
	}
	maximum, _ := parseSemverValue(interval.Max)
	if maximum.pre != "" || pubBuildIdentifier(interval.Max) != "" {
		return false
	}
	if interval.Min == "" || !validPubVersion(interval.Min) {
		return true
	}
	minimum, _ := parseSemverValue(interval.Min)
	if minimum.pre == "" {
		return true
	}
	for i := range minimum.core {
		if cmpNumStr(minimum.core[i], maximum.core[i]) != 0 {
			return true
		}
	}
	return false
}

// tokenizePubConstraints splits Pub constraints while allowing whitespace on
// either side of an operator, including no whitespace between constraints.
func tokenizePubConstraints(constraint string) ([]string, error) {
	remaining := strings.TrimSpace(constraint)
	var tokens []string
	for remaining != "" {
		operator, rest := extractOperator(remaining)
		if operator == "=" || operator == "!=" {
			return nil, fmt.Errorf("unsupported pub operator: %s", operator)
		}
		if operator != "" {
			remaining = strings.TrimSpace(rest)
		}
		match := pubVersionPrefixRegex.FindString(remaining)
		if match == "" {
			return nil, fmt.Errorf("invalid pub constraint: %s", constraint)
		}
		tokens = append(tokens, operator+match)
		remaining = strings.TrimSpace(remaining[len(match):])
	}
	return tokens, nil
}

// parsePubCaretRange expands Pub's compatible-with operator. Pub treats all
// pre-1.0 releases in the same minor series as compatible.
func parsePubCaretRange(version string) (*Range, error) {
	parsed, ok := parseSemverValue(version)
	if !ok {
		return nil, fmt.Errorf("invalid pub caret version: %s", version)
	}
	var upper string
	if cmpNumStr(parsed.core[0], "0") == 0 {
		upper = "0." + incNumStr(parsed.core[1]) + ".0-0"
	} else {
		upper = incNumStr(parsed.core[0]) + ".0.0-0"
	}
	return rangeWithScheme(NewRange([]Interval{
		NewInterval(version, upper, true, false),
	}), schemePub), nil
}

// parsePubConstraint parses one exact or comparator-based Pub constraint.
func parsePubConstraint(constraint string) (*Range, error) {
	operator, version := extractOperator(strings.TrimSpace(constraint))
	if operator == "=" || operator == "!=" {
		return nil, fmt.Errorf("unsupported pub operator: %s", operator)
	}
	if operator == "" {
		version = strings.TrimSpace(constraint)
	}
	if !validPubVersion(strings.TrimSpace(version)) {
		return nil, fmt.Errorf("invalid pub version: %s", version)
	}
	parsed, err := parseConstraintWithScheme(constraint, schemePub)
	if err != nil {
		return nil, err
	}
	interval, ok := parsed.ToInterval()
	if !ok {
		return nil, fmt.Errorf("invalid pub constraint: %s", constraint)
	}
	return rangeWithScheme(NewRange([]Interval{interval}), schemePub), nil
}

// validPubVersion reports whether a version is a complete Pub semantic
// version without a v prefix.
func validPubVersion(version string) bool {
	if strings.HasPrefix(version, "v") || strings.HasPrefix(version, "V") {
		return false
	}
	return validSemverLike(version) && pubVersionPrefixRegex.FindString(version) == version
}

// comparePub compares Pub versions, including build identifiers in version
// precedence as required by pub_semver.
func comparePub(a, b string) int {
	if !validPubVersion(a) || !validPubVersion(b) {
		return CompareVersions(a, b)
	}
	left, _ := parseSemverValue(a)
	right, _ := parseSemverValue(b)
	for i := range left.core {
		if comparison := cmpNumStr(left.core[i], right.core[i]); comparison != 0 {
			return comparison
		}
	}
	if comparison := compareSemverPrereleaseStrings(left.pre, right.pre); comparison != 0 {
		return comparison
	}
	leftBuild := pubBuildIdentifier(a)
	rightBuild := pubBuildIdentifier(b)
	switch {
	case leftBuild == "" && rightBuild == "":
		return 0
	case leftBuild == "":
		return -1
	case rightBuild == "":
		return 1
	default:
		return compareSemverPrereleaseStrings(leftBuild, rightBuild)
	}
}

// pubBuildIdentifier returns the build portion of a Pub version.
func pubBuildIdentifier(version string) string {
	if index := strings.IndexByte(version, '+'); index >= 0 {
		return version[index+1:]
	}
	return ""
}

// normalizePubVersion returns Pub's canonical version representation,
// including normalized numeric prerelease and build identifiers.
func normalizePubVersion(version string) string {
	normalized := normalizeSemverLike(version, false)
	index := strings.IndexByte(normalized, '+')
	if index < 0 {
		return normalized
	}
	build := strings.Split(normalized[index+1:], ".")
	for i, part := range build {
		if isDigits(part) {
			build[i] = trimLeadingZeros(part)
		}
	}
	return normalized[:index+1] + strings.Join(build, ".")
}

// rangeWithScheme assigns a comparison scheme to a parsed range.
func rangeWithScheme(r *Range, scheme string) *Range {
	if r != nil {
		r.Scheme = scheme
	}
	return r
}
