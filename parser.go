package vers

import (
	"fmt"
	"regexp"
	"strings"
)

var versURIRegex = regexp.MustCompile(`^vers:([^/]+)/(.*)$`)

// Parser handles parsing of vers URIs and native package manager syntax.
type Parser struct{}

// NewParser creates a new Parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a vers URI string into a Range.
func (p *Parser) Parse(versURI string) (*Range, error) {
	matches := versURIRegex.FindStringSubmatch(versURI)
	if matches == nil {
		return nil, fmt.Errorf("invalid vers URI format: %s", versURI)
	}

	scheme := matches[1]
	constraintsStr := matches[2]

	// Handle wildcard for unbounded range
	if constraintsStr == "*" || constraintsStr == "" {
		return Unbounded(), nil
	}

	return p.parseConstraints(constraintsStr, scheme)
}

// ParseNative parses a native package manager version range into a Range.
func (p *Parser) ParseNative(constraint string, scheme string) (*Range, error) {
	switch scheme {
	case "npm":
		return p.parseNpmRange(constraint)
	case "gem", "rubygems":
		return p.parseGemRange(constraint)
	case "pypi":
		return p.parsePypiRange(constraint)
	case "maven":
		return p.parseMavenRange(constraint)
	case "nuget":
		return p.parseNugetRange(constraint)
	case "cargo":
		return p.parseCargoRange(constraint)
	case "go", "golang":
		return p.parseGoRange(constraint)
	case "hex", "elixir":
		return p.parseHexRange(constraint)
	case "deb", "debian":
		return p.parseDebianRange(constraint)
	case "rpm":
		return p.parseRpmRange(constraint)
	default:
		return p.parseConstraints(constraint, scheme)
	}
}

// ToVersString converts a Range back to a vers URI string.
func (p *Parser) ToVersString(r *Range, scheme string) string {
	if r.IsUnbounded() && len(r.Exclusions) == 0 && len(r.RawConstraints) == 0 {
		return fmt.Sprintf("vers:%s/*", scheme)
	}
	// Check if empty but has raw constraints (preserve them for output)
	if r.IsEmpty() && len(r.RawConstraints) == 0 {
		return fmt.Sprintf("vers:%s/", scheme)
	}

	// Use RawConstraints if available (for preserving original structure)
	intervals := r.Intervals
	if len(r.RawConstraints) > 0 {
		intervals = r.RawConstraints
	}

	var constraints []constraintWithVersion
	for _, interval := range intervals {
		if interval.Min == interval.Max && interval.MinInclusive && interval.MaxInclusive && interval.Min != "" {
			// Exact version - no operator needed per VERS spec
			constraints = append(constraints, constraintWithVersion{
				str:     normalizeVersion(interval.Min, scheme),
				sortKey: interval.Min,
			})
		} else {
			if interval.Min != "" {
				op := ">"
				if interval.MinInclusive {
					op = ">="
				}
				constraints = append(constraints, constraintWithVersion{
					str:     op + normalizeVersion(interval.Min, scheme),
					sortKey: interval.Min,
				})
			}
			if interval.Max != "" {
				op := "<"
				if interval.MaxInclusive {
					op = "<="
				}
				constraints = append(constraints, constraintWithVersion{
					str:     op + normalizeVersion(interval.Max, scheme),
					sortKey: interval.Max,
				})
			}
		}
	}

	// Add exclusions
	for _, exc := range r.Exclusions {
		constraints = append(constraints, constraintWithVersion{
			str:     "!=" + normalizeVersion(exc, scheme),
			sortKey: exc,
		})
	}

	// Sort constraints by version
	sortConstraintsByVersion(constraints)

	var strs []string
	for _, c := range constraints {
		strs = append(strs, c.str)
	}

	return fmt.Sprintf("vers:%s/%s", scheme, strings.Join(strs, "|"))
}

// constraintWithVersion holds a constraint string and its sort key.
type constraintWithVersion struct {
	str     string
	sortKey string
}

// sortConstraintsByVersion sorts constraints by their version in ascending order.
func sortConstraintsByVersion(constraints []constraintWithVersion) {
	// Simple bubble sort to avoid import
	for i := 0; i < len(constraints); i++ {
		for j := i + 1; j < len(constraints); j++ {
			if CompareVersions(constraints[i].sortKey, constraints[j].sortKey) > 0 {
				constraints[i], constraints[j] = constraints[j], constraints[i]
			}
		}
	}
}

// normalizeVersion normalizes a version string for output.
// For semver-based schemes, this ensures 3-part versions (1.1 -> 1.1.0).
func normalizeVersion(version, scheme string) string {
	// Don't normalize if it already has prerelease info
	if strings.Contains(version, "-") {
		return version
	}

	// Count the number of dots
	dots := strings.Count(version, ".")

	switch scheme {
	case "npm", "cargo", "nuget":
		// These schemes use semver, normalize to 3 parts
		switch dots {
		case 0:
			return version + ".0.0"
		case 1:
			return version + ".0"
		}
	}

	return version
}

func (p *Parser) parseConstraints(constraintsStr, scheme string) (*Range, error) {
	parts := strings.Split(constraintsStr, "|")
	var intervals []Interval
	var exclusions []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		constraint, err := parseConstraintWithScheme(part, scheme)
		if err != nil {
			return nil, err
		}

		if constraint.IsExclusion() {
			exclusions = append(exclusions, constraint.Version)
		} else {
			interval, ok := constraint.ToInterval()
			if ok {
				intervals = append(intervals, interval)
			}
		}
	}

	// Collect all intervals - they form a union
	// Then intersect overlapping intervals to form proper ranges
	result := intersectConsecutiveIntervals(intervals)

	// If we only have exclusions and no other constraints, start with unbounded range
	if result == nil {
		if len(exclusions) > 0 {
			result = Unbounded()
		} else {
			result = &Range{}
		}
	}
	result.Exclusions = exclusions
	return result, nil
}

// intersectConsecutiveIntervals handles VERS constraint semantics:
// - Consecutive unbounded intervals (like >=X followed by <Y) are intersected to form a range
// - Bounded intervals (exact versions) are unioned
func intersectConsecutiveIntervals(intervals []Interval) *Range {
	if len(intervals) == 0 {
		return nil
	}
	if len(intervals) == 1 {
		return NewRange(intervals)
	}

	var resultIntervals []Interval
	i := 0
	for i < len(intervals) {
		current := intervals[i]

		// Check if current and next can be intersected to form a bounded range
		if i+1 < len(intervals) {
			next := intervals[i+1]
			// If one has only min and other has only max, intersect them
			if (current.Min != "" && current.Max == "" && next.Max != "" && next.Min == "") ||
				(current.Max != "" && current.Min == "" && next.Min != "" && next.Max == "") {
				intersection := current.Intersect(next)
				if !intersection.IsEmpty() {
					resultIntervals = append(resultIntervals, intersection)
					i += 2
					continue
				}
			}
		}

		// Otherwise just add the interval (union semantics)
		resultIntervals = append(resultIntervals, current)
		i++
	}

	return NewRange(resultIntervals)
}

// npm: ^1.2.3, ~1.2.3, >=1.0.0 <2.0.0, ||
func (p *Parser) parseNpmRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" || s == "x" || s == "X" {
		return Unbounded(), nil
	}

	// Handle || (OR)
	if strings.Contains(s, "||") {
		parts := strings.Split(s, "||")
		var result *Range
		for _, part := range parts {
			// Each OR part may contain AND constraints, so recurse
			r, err := p.parseNpmRange(strings.TrimSpace(part))
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

	// Handle space-separated AND constraints
	if strings.Contains(s, " ") && !strings.Contains(s, " - ") {
		parts := tokenizeNpmConstraints(s)
		var result *Range
		for _, part := range parts {
			r, err := p.parseNpmSingleRange(part)
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

	return p.parseNpmSingleRange(s)
}

// tokenizeNpmConstraints splits an npm constraint string into individual constraints,
// properly handling operators followed by spaces (e.g., ">= 1.0.0" stays as one token).
func tokenizeNpmConstraints(s string) []string {
	tokens := strings.Fields(s)
	if len(tokens) <= 1 {
		return tokens
	}

	// Merge operator-only tokens with the following version token
	var result []string
	i := 0
	for i < len(tokens) {
		token := tokens[i]
		// Check if this token is just an operator
		if isOperatorOnly(token) && i+1 < len(tokens) {
			// Merge with next token
			result = append(result, token+tokens[i+1])
			i += 2
		} else {
			result = append(result, token)
			i++
		}
	}
	return result
}

// isOperatorOnly checks if a string is just an operator without a version.
func isOperatorOnly(s string) bool {
	switch s {
	case ">=", "<=", ">", "<", "=", "!=":
		return true
	}
	return false
}

// extractOperator extracts an operator prefix from a constraint string.
// Returns the operator and the remaining version string.
func extractOperator(s string) (string, string) {
	for _, op := range []string{">=", "<=", "!=", ">", "<", "="} {
		if strings.HasPrefix(s, op) {
			return op, s[len(op):]
		}
	}
	return "", s
}

func (p *Parser) parseNpmSingleRange(s string) (*Range, error) {
	// Caret range: ^1.2.3
	if strings.HasPrefix(s, "^") {
		return p.parseCaretRange(s[1:])
	}

	// Tilde range: ~1.2.3
	if strings.HasPrefix(s, "~") {
		return p.parseTildeRange(s[1:])
	}

	// Hyphen range: 1.2.3 - 2.0.0
	if strings.Contains(s, " - ") {
		parts := strings.SplitN(s, " - ", 2)
		return NewRange([]Interval{
			NewInterval(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true, true),
		}), nil
	}

	// X-range: 1.x, 1.2.x (also handle operator + x-range like >=1.x)
	if strings.HasSuffix(s, ".x") || strings.HasSuffix(s, ".X") || strings.HasSuffix(s, ".*") {
		// Check if there's an operator prefix
		op, version := extractOperator(s)
		if op != "" {
			// For >=X.x or >X.x, the x-range defines the minimum
			xRange, err := p.parseXRange(version)
			if err != nil {
				return nil, err
			}
			// >=2.2.x means >=2.2.0 (start of the x-range)
			// The x-range itself is the answer for >= with x-range
			return xRange, nil
		}
		return p.parseXRange(s)
	}

	// Standard constraint
	constraint, err := ParseConstraint(s)
	if err != nil {
		return nil, err
	}
	interval, ok := constraint.ToInterval()
	if !ok {
		if constraint.IsExclusion() {
			return Unbounded().Exclude(constraint.Version), nil
		}
		return nil, fmt.Errorf("invalid constraint: %s", s)
	}
	return NewRange([]Interval{interval}), nil
}

// ^1.2.3 := >=1.2.3 <2.0.0
func (p *Parser) parseCaretRange(version string) (*Range, error) {
	v, err := ParseVersion(version)
	if err != nil {
		return nil, err
	}

	var upper string
	if v.Major > 0 {
		upper = fmt.Sprintf("%d.0.0", v.Major+1)
	} else if v.Minor > 0 {
		upper = fmt.Sprintf("0.%d.0", v.Minor+1)
	} else {
		upper = fmt.Sprintf("0.0.%d", v.Patch+1)
	}

	return NewRange([]Interval{
		NewInterval(version, upper, true, false),
	}), nil
}

// ~1.2.3 := >=1.2.3 <1.3.0
// ~1.2.3-pre := >=1.2.3-pre <1.2.3 OR >=1.2.3 <1.2.4 (for prerelease handling)
func (p *Parser) parseTildeRange(version string) (*Range, error) {
	v, err := ParseVersion(version)
	if err != nil {
		return nil, err
	}

	// If there's a prerelease, we need special handling
	// npm semver only matches prereleases if they're on the same major.minor.patch
	if v.Prerelease != "" {
		// Create two intervals:
		// 1. Prereleases from the specified version to the release version
		// 2. Release versions for patch updates
		baseVersion := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
		nextPatch := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch+1)

		return NewRange([]Interval{
			// Prerelease interval: >=version <baseVersion
			NewInterval(version, baseVersion, true, false),
			// Release interval: >=baseVersion <nextPatch
			NewInterval(baseVersion, nextPatch, true, false),
		}), nil
	}

	var upper string
	if v.Minor > 0 || v.Patch > 0 {
		upper = fmt.Sprintf("%d.%d.0", v.Major, v.Minor+1)
	} else {
		upper = fmt.Sprintf("%d.0.0", v.Major+1)
	}

	return NewRange([]Interval{
		NewInterval(version, upper, true, false),
	}), nil
}

// 1.x := >=1.0.0 <2.0.0
func (p *Parser) parseXRange(s string) (*Range, error) {
	s = strings.TrimSuffix(s, ".x")
	s = strings.TrimSuffix(s, ".X")
	s = strings.TrimSuffix(s, ".*")

	parts := strings.Split(s, ".")
	if len(parts) == 1 {
		major := parts[0]
		v, err := ParseVersion(major)
		if err != nil {
			return nil, err
		}
		return NewRange([]Interval{
			NewInterval(fmt.Sprintf("%d.0.0", v.Major), fmt.Sprintf("%d.0.0", v.Major+1), true, false),
		}), nil
	}

	v, err := ParseVersion(s)
	if err != nil {
		return nil, err
	}
	return NewRange([]Interval{
		NewInterval(fmt.Sprintf("%d.%d.0", v.Major, v.Minor), fmt.Sprintf("%d.%d.0", v.Major, v.Minor+1), true, false),
	}), nil
}

// gem: ~> 1.2, >= 1.0, < 2.0
func (p *Parser) parseGemRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)

	// Pessimistic operator: ~> 1.2.3
	if strings.HasPrefix(s, "~>") {
		version := strings.TrimSpace(s[2:])
		return p.parsePessimisticRange(version)
	}

	// Comma-separated constraints
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		var result *Range
		for _, part := range parts {
			r, err := p.parseGemRange(strings.TrimSpace(part))
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

	// Standard constraint
	return p.parseConstraints(s, "gem")
}

// ~> 1.2.3 := >= 1.2.3, < 1.3
// ~> 1.2   := >= 1.2,   < 2.0
func (p *Parser) parsePessimisticRange(version string) (*Range, error) {
	v, err := ParseVersion(version)
	if err != nil {
		return nil, err
	}

	// Count segments in original version string to preserve precision
	segments := strings.Count(version, ".") + 1

	var upper string
	if segments >= 3 {
		// ~> 1.2.3 bumps minor: < 1.3
		upper = fmt.Sprintf("%d.%d", v.Major, v.Minor+1)
	} else if segments == 2 {
		// ~> 1.2 bumps major: < 2.0
		upper = fmt.Sprintf("%d.0", v.Major+1)
	} else {
		// ~> 1 bumps major: < 2.0
		upper = fmt.Sprintf("%d.0", v.Major+1)
	}

	return NewRange([]Interval{
		NewInterval(version, upper, true, false),
	}), nil
}

// pypi: >=1.0,<2.0, ~=1.4.2, !=1.5.0
func (p *Parser) parsePypiRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)

	// Compatible release: ~=1.4.2
	if strings.HasPrefix(s, "~=") {
		version := strings.TrimSpace(s[2:])
		return p.parsePessimisticRange(version)
	}

	// Comma-separated constraints
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		constraintStr := strings.Join(parts, "|")
		return p.parseConstraints(constraintStr, "pypi")
	}

	return p.parseConstraints(s, "pypi")
}

// maven: [1.0,2.0), (1.0,2.0], [1.0,)
func (p *Parser) parseMavenRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)

	// Bracket notation
	if (strings.HasPrefix(s, "[") || strings.HasPrefix(s, "(")) &&
		(strings.HasSuffix(s, "]") || strings.HasSuffix(s, ")")) {
		return p.parseBracketRange(s)
	}

	// Simple version (minimum version in Maven)
	if matched, _ := regexp.MatchString(`^[0-9]`, s); matched {
		return NewRange([]Interval{
			GreaterThanInterval(s, true),
		}), nil
	}

	return p.parseConstraints(s, "maven")
}

func (p *Parser) parseBracketRange(s string) (*Range, error) {
	minInclusive := s[0] == '['
	maxInclusive := s[len(s)-1] == ']'

	inner := s[1 : len(s)-1]
	parts := strings.SplitN(inner, ",", 2)

	if len(parts) == 1 {
		// Exact version: [1.0]
		return Exact(strings.TrimSpace(parts[0])), nil
	}

	min := strings.TrimSpace(parts[0])
	max := strings.TrimSpace(parts[1])

	interval := Interval{
		Min:          min,
		Max:          max,
		MinInclusive: minInclusive,
		MaxInclusive: maxInclusive,
	}

	if min == "" {
		interval.Min = ""
	}
	if max == "" {
		interval.Max = ""
	}

	return NewRange([]Interval{interval}), nil
}

// nuget: same as maven
func (p *Parser) parseNugetRange(s string) (*Range, error) {
	return p.parseMavenRange(s)
}

// cargo: ^1.2.3, ~1.2.3, >=1.0.0
func (p *Parser) parseCargoRange(s string) (*Range, error) {
	// Cargo uses similar syntax to npm
	return p.parseNpmRange(s)
}

// go: >=1.0.0, <2.0.0
func (p *Parser) parseGoRange(s string) (*Range, error) {
	// Go uses comma-separated constraints
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		var result *Range
		for _, part := range parts {
			constraint, err := parseConstraintWithScheme(strings.TrimSpace(part), "go")
			if err != nil {
				return nil, err
			}
			interval, ok := constraint.ToInterval()
			if !ok {
				continue
			}
			r := NewRange([]Interval{interval})
			if result == nil {
				result = r
			} else {
				result = result.Intersect(r)
			}
		}
		return result, nil
	}

	return p.parseConstraints(s, "go")
}

// hex/elixir: ~> 1.2.3, >= 1.0.0 and < 2.0.0, ~> 1.0 or ~> 2.0
func (p *Parser) parseHexRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)

	// Handle "or" disjunction first
	if strings.Contains(s, " or ") {
		parts := strings.Split(s, " or ")
		var result *Range
		for _, part := range parts {
			r, err := p.parseHexSingleRange(strings.TrimSpace(part))
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

	return p.parseHexSingleRange(s)
}

func (p *Parser) parseHexSingleRange(s string) (*Range, error) {
	// Handle "and" conjunction
	if strings.Contains(s, " and ") {
		parts := strings.Split(s, " and ")
		var result *Range
		for _, part := range parts {
			r, err := p.parseHexConstraint(strings.TrimSpace(part))
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

	return p.parseHexConstraint(s)
}

func (p *Parser) parseHexConstraint(s string) (*Range, error) {
	// Pessimistic operator: ~> 1.2.3
	if strings.HasPrefix(s, "~>") {
		version := strings.TrimSpace(s[2:])
		return p.parsePessimisticRange(version)
	}

	// Normalize == to = for internal constraint parsing
	normalized := strings.Replace(s, "==", "=", 1)
	constraint, err := ParseConstraint(normalized)
	if err != nil {
		return nil, err
	}

	if constraint.IsExclusion() {
		return Unbounded().Exclude(constraint.Version), nil
	}

	interval, ok := constraint.ToInterval()
	if !ok {
		return nil, fmt.Errorf("invalid hex constraint: %s", s)
	}
	return NewRange([]Interval{interval}), nil
}

// debian: >= 1.0, << 2.0
func (p *Parser) parseDebianRange(s string) (*Range, error) {
	// Convert Debian operators to standard
	s = strings.ReplaceAll(s, ">>", ">")
	s = strings.ReplaceAll(s, "<<", "<")
	return p.parseConstraints(s, "deb")
}

// rpm: >= 1.0, <= 2.0
func (p *Parser) parseRpmRange(s string) (*Range, error) {
	return p.parseConstraints(s, "rpm")
}
