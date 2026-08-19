package vers

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var nginxRangeRegex = regexp.MustCompile(`^\d+(?:\.\d+)+-\d+(?:\.\d+)+$`)

// Parser handles parsing of vers URIs and native package manager syntax.
type Parser struct{}

// NewParser creates a new Parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a vers URI string into a Range.
func (p *Parser) Parse(versURI string) (*Range, error) {
	return p.parseVersURI(versURI, false)
}

func (p *Parser) parseVersURI(versURI string, requireCanonicalOrder bool) (*Range, error) {
	const prefix = "vers:"
	if !strings.HasPrefix(versURI, prefix) {
		return nil, fmt.Errorf("invalid vers URI format: %s", versURI)
	}
	if strings.ContainsAny(versURI, " \t\r\n") {
		return nil, fmt.Errorf("non-canonical VERS: whitespace is not permitted")
	}
	remainder := versURI[len(prefix):]
	slash := strings.IndexByte(remainder, '/')
	if slash <= 0 {
		return nil, fmt.Errorf("invalid vers URI format: %s", versURI)
	}

	scheme := remainder[:slash]
	constraintsStr := remainder[slash+1:]

	// Handle wildcard for unbounded range
	if constraintsStr == "*" || constraintsStr == "" {
		r := Unbounded()
		r.Scheme = scheme
		return r, nil
	}
	if err := validateVersConstraints(constraintsStr, scheme, requireCanonicalOrder); err != nil {
		return nil, err
	}

	return p.parseConstraints(constraintsStr, scheme)
}

func validateVersConstraints(constraints, scheme string, requireCanonicalOrder bool) error {
	if strings.HasPrefix(constraints, "|") {
		return fmt.Errorf("non-canonical VERS: leading pipe is not permitted")
	}
	if strings.HasSuffix(constraints, "|") {
		return fmt.Errorf("non-canonical VERS: trailing pipe is not permitted")
	}
	if strings.Contains(constraints, "||") {
		return fmt.Errorf("non-canonical VERS: consecutive pipes are not permitted")
	}

	var previous *Constraint
	previousRaw := ""
	for _, raw := range strings.Split(constraints, "|") {
		operator := constraintOperator(raw)
		version := raw[len(operator):]
		if err := validateVersVersion(version, scheme); err != nil {
			return err
		}
		constraint, err := parseConstraintWithScheme(raw, scheme)
		if err != nil {
			return err
		}
		if requireCanonicalOrder && previous != nil {
			order := CompareWithScheme(previous.Version, constraint.Version, scheme)
			if order > 0 || (order == 0 && previousRaw > raw) {
				return fmt.Errorf("non-canonical VERS: constraints are not sorted by version")
			}
		}
		previous, previousRaw = constraint, raw
	}
	return nil
}

func validateVersVersion(version, scheme string) error {
	for i := 0; i < len(version); i++ {
		if version[i] != '%' {
			continue
		}
		if i+2 >= len(version) || !isASCIIHex(version[i+1]) || !isASCIIHex(version[i+2]) {
			return fmt.Errorf("non-canonical VERS: invalid percent-encoding in version")
		}
		if isLowerASCIIHex(version[i+1]) || isLowerASCIIHex(version[i+2]) {
			return fmt.Errorf("non-canonical VERS: percent-encoding in version is not canonical")
		}
		i += 2
	}

	if scheme != schemeDatetime {
		return nil
	}
	if strings.Contains(version, "%3A") {
		return fmt.Errorf("non-canonical VERS: datetime time colons must be unencoded")
	}
	decoded, err := url.PathUnescape(version)
	if err != nil {
		return fmt.Errorf("non-canonical VERS: invalid percent-encoding in version")
	}
	if len(decoded) > len("2006-01-02") && decoded[len("2006-01-02")] == 't' || strings.HasSuffix(decoded, "z") {
		return fmt.Errorf("non-canonical VERS: datetime must use uppercase T and Z")
	}
	return nil
}

func isASCIIHex(c byte) bool {
	return isASCIIDigit(c) || c >= 'A' && c <= 'F' || isLowerASCIIHex(c)
}

func isLowerASCIIHex(c byte) bool {
	return c >= 'a' && c <= 'f'
}

// ParseNative parses a native package manager version range into a Range.
func (p *Parser) ParseNative(constraint string, scheme string) (*Range, error) {
	r, err := p.parseNative(constraint, scheme)
	if r != nil {
		r.Scheme = scheme
	}
	return r, err
}

func (p *Parser) parseNative(constraint string, scheme string) (*Range, error) {
	switch scheme {
	case schemeNPM:
		return p.parseNpmRange(constraint)
	case schemeComposer:
		return p.parseComposerRange(constraint)
	case schemeGem, schemeRubyGems:
		return p.parseGemRange(constraint)
	case schemePyPI:
		return p.parsePypiRange(constraint)
	case schemePub:
		return p.parsePubRange(constraint)
	case schemeMaven:
		return p.parseMavenRange(constraint)
	case schemeNuGet:
		return p.parseNugetRange(constraint)
	case schemeCargo:
		return p.parseCargoRange(constraint)
	case schemeGo, schemeGolang:
		return p.parseGoRange(constraint)
	case schemeHex, schemeElixir:
		return p.parseHexRange(constraint)
	case schemeDeb, schemeDebian:
		return p.parseDebianRange(constraint)
	case schemeRPM:
		return p.parseRpmRange(constraint)
	case schemeConan:
		return p.parseConanRange(constraint)
	case schemeOpenSSL:
		return p.parseOpenSSLRange(constraint)
	case schemeNginx:
		return p.parseNginxRange(constraint)
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
				str:     encodeVersVersion(normalizeVersion(interval.Min, scheme)),
				sortKey: interval.Min,
			})
		} else {
			if interval.Min != "" {
				op := ">"
				if interval.MinInclusive {
					op = ">="
				}
				constraints = append(constraints, constraintWithVersion{
					str:     op + encodeVersVersion(normalizeVersion(interval.Min, scheme)),
					sortKey: interval.Min,
				})
			}
			if interval.Max != "" {
				op := "<"
				if interval.MaxInclusive {
					op = "<="
				}
				constraints = append(constraints, constraintWithVersion{
					str:     op + encodeVersVersion(normalizeVersion(interval.Max, scheme)),
					sortKey: interval.Max,
				})
			}
		}
	}

	// Add exclusions
	for _, exc := range r.Exclusions {
		constraints = append(constraints, constraintWithVersion{
			str:     "!=" + encodeVersVersion(normalizeVersion(exc, scheme)),
			sortKey: exc,
		})
	}

	// Sort constraints by version
	sortConstraintsByVersion(constraints, scheme)

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
func sortConstraintsByVersion(constraints []constraintWithVersion, scheme string) {
	cmp := compareFuncFor(scheme)
	// Simple bubble sort to avoid import
	for i := 0; i < len(constraints); i++ {
		for j := i + 1; j < len(constraints); j++ {
			order := cmp(constraints[i].sortKey, constraints[j].sortKey)
			if order > 0 || (order == 0 && constraints[i].str > constraints[j].str) {
				constraints[i], constraints[j] = constraints[j], constraints[i]
			}
		}
	}
}

var versMetaEncoder = strings.NewReplacer(
	"%", "%25",
	"|", "%7C",
	">", "%3E",
	"<", "%3C",
	"=", "%3D",
	"!", "%21",
	"/", "%2F",
	"*", "%2A",
	" ", "%20",
)

func encodeVersVersion(v string) string {
	return versMetaEncoder.Replace(v)
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
	case schemeNPM, schemeCargo, schemeNuGet, schemeComposer, schemePub:
		if !validSemverLike(version) {
			return version
		}
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
	result := intersectConsecutiveIntervals(intervals, scheme)

	// If we only have exclusions and no other constraints, start with unbounded range
	if result == nil {
		if len(exclusions) > 0 {
			result = Unbounded()
		} else {
			result = &Range{}
		}
	}
	result.Exclusions = exclusions
	result.Scheme = scheme
	return result, nil
}

// intersectConsecutiveIntervals handles VERS constraint semantics:
// - Consecutive unbounded intervals (like >=X followed by <Y) are intersected to form a range
// - Bounded intervals (exact versions) are unioned
func intersectConsecutiveIntervals(intervals []Interval, scheme string) *Range {
	if len(intervals) == 0 {
		return nil
	}
	if len(intervals) == 1 {
		return NewRange(intervals)
	}

	cmp := compareFuncFor(scheme)

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
				if !intersection.isEmptyCmp(cmp) {
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

	// Handle || (OR) -- collect all parts, then merge once
	if strings.Contains(s, "||") {
		parts := strings.Split(s, "||")
		var allIntervals []Interval
		var allExclusions []string
		var allRaw []Interval
		for _, part := range parts {
			r, err := p.parseNpmRange(strings.TrimSpace(part))
			if err != nil {
				return nil, err
			}
			allIntervals = append(allIntervals, r.Intervals...)
			allExclusions = append(allExclusions, r.Exclusions...)
			if len(r.RawConstraints) > 0 {
				allRaw = append(allRaw, r.RawConstraints...)
			} else {
				allRaw = append(allRaw, r.Intervals...)
			}
		}
		return &Range{
			Intervals:      mergeIntervals(allIntervals, CompareVersions),
			Exclusions:     allExclusions,
			RawConstraints: allRaw,
		}, nil
	}

	// Handle space-separated AND constraints
	if strings.ContainsAny(s, " \t\r\n") && !strings.Contains(s, " - ") {
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
	case ">=", "<=", ">", "<", "=", "!=", "~", "~>", "^":
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
	s = strings.TrimSpace(s)
	// Caret range: ^1.2.3
	if strings.HasPrefix(s, "^") {
		return p.parseCaretRange(s[1:])
	}

	// Tilde range: ~1.2.3 or ~>1.2.3
	if strings.HasPrefix(s, "~>") {
		return p.parseTildeRange(strings.TrimSpace(s[2:]))
	}
	if strings.HasPrefix(s, "~") {
		return p.parseTildeRange(strings.TrimSpace(s[1:]))
	}

	// Hyphen range: 1.2.3 - 2.0.0
	if strings.Contains(s, " - ") {
		parts := strings.SplitN(s, " - ", 2) //nolint:mnd
		return p.parseNpmHyphenRange(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	// X-range: 1.x, 1.2.x (also handle operator + x-range like >=1.x)
	if strings.HasSuffix(s, ".x") || strings.HasSuffix(s, ".X") || strings.HasSuffix(s, ".*") {
		// Check if there's an operator prefix
		op, version := extractOperator(s)
		if op != "" {
			return p.parseNpmPartialRange(version, op)
		}
		return p.parseXRange(s)
	}
	if op, version := extractOperator(s); isPartialNpmVersion(version) {
		return p.parseNpmPartialRange(version, op)
	}
	if isPartialNpmVersion(s) {
		return p.parseNpmPartialRange(s, "")
	}
	if op, version := extractOperator(s); op != "" {
		s = op + strings.TrimSpace(version)
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
	if version == "" || version == "*" || version == "x" || version == "X" {
		return Unbounded(), nil
	}
	if strings.HasSuffix(version, ".x") || strings.HasSuffix(version, ".X") || strings.HasSuffix(version, ".*") {
		return p.parseXRange(version)
	}

	v, err := ParseVersion(version)
	if err != nil {
		return nil, err
	}

	base := strings.SplitN(version, "+", 2)[0] //nolint:mnd
	base = strings.SplitN(base, "-", 2)[0]     //nolint:mnd
	segments := strings.Count(base, ".") + 1
	var upper string
	switch {
	case segments == 1 || v.Major > 0:
		upper = fmt.Sprintf("%d.0.0", v.Major+1)
	case segments == 2 || v.Minor > 0: //nolint:mnd
		upper = fmt.Sprintf("0.%d.0", v.Minor+1)
	default:
		upper = fmt.Sprintf("0.0.%d", v.Patch+1)
	}

	return NewRange([]Interval{
		NewInterval(version, upper, true, false),
	}), nil
}

// ~1.2.3 := >=1.2.3 <1.3.0
// ~1.2.3-pre := >=1.2.3-pre <1.2.3 OR >=1.2.3 <1.2.4 (for prerelease handling)
func (p *Parser) parseTildeRange(version string) (*Range, error) {
	if version == "" || version == "*" || version == "x" || version == "X" {
		return Unbounded(), nil
	}
	if strings.HasSuffix(version, ".x") || strings.HasSuffix(version, ".X") || strings.HasSuffix(version, ".*") {
		return p.parseXRange(version)
	}

	v, err := ParseVersion(version)
	if err != nil {
		return nil, err
	}

	base := strings.SplitN(version, "+", 2)[0] //nolint:mnd
	base = strings.SplitN(base, "-", 2)[0]     //nolint:mnd
	segments := strings.Count(base, ".") + 1

	var upper string
	if segments >= 2 { //nolint:mnd
		// ~1.2.3 := >=1.2.3 <1.3.0
		// ~1.0.0 := >=1.0.0 <1.1.0
		// ~1.0   := >=1.0.0 <1.1.0
		upper = fmt.Sprintf("%d.%d.0", v.Major, v.Minor+1)
	} else {
		// ~1 := >=1.0.0 <2.0.0
		upper = fmt.Sprintf("%d.0.0", v.Major+1)
	}

	r := NewRange([]Interval{
		NewInterval(version, upper, true, false),
	})
	if v.Prerelease != "" {
		baseVersion := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
		nextPatch := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch+1)
		r.RawConstraints = []Interval{
			NewInterval(version, baseVersion, true, false),
			NewInterval(baseVersion, nextPatch, true, false),
		}
	}
	return r, nil
}

// 1.x := >=1.0.0 <2.0.0
func (p *Parser) parseXRange(s string) (*Range, error) {
	return p.parseNpmPartialRange(s, "")
}

func (p *Parser) parseNpmPartialRange(version, operator string) (*Range, error) {
	lower, upper, err := npmPartialBounds(version)
	if err != nil {
		return nil, err
	}
	if lower == "" {
		return Unbounded(), nil
	}
	var r *Range
	switch operator {
	case "", "=":
		r = NewRange([]Interval{NewInterval(lower, upper, true, false)})
	case ">=":
		r = NewRange([]Interval{NewInterval(lower, "", true, false)})
	case ">":
		r = NewRange([]Interval{NewInterval(upper, "", true, false)})
	case "<=":
		r = NewRange([]Interval{NewInterval("", upper, false, false)})
	case "<":
		r = NewRange([]Interval{NewInterval("", lower, false, false)})
	default:
		return nil, fmt.Errorf("invalid operator for npm partial range: %s", operator)
	}
	// Keep the established native-to-VERS rendering while containment uses
	// the expanded interval above.
	hasWildcard := strings.ContainsAny(version, "xX*")
	switch {
	case hasWildcard:
		r.RawConstraints = []Interval{NewInterval(lower, upper, true, false)}
	case operator == "" || operator == "=":
		r.RawConstraints = []Interval{ExactInterval(lower)}
	case operator == ">=":
		r.RawConstraints = []Interval{NewInterval(lower, "", true, false)}
	case operator == ">":
		r.RawConstraints = []Interval{NewInterval(lower, "", false, false)}
	case operator == "<=":
		r.RawConstraints = []Interval{NewInterval("", lower, false, true)}
	case operator == "<":
		r.RawConstraints = []Interval{NewInterval("", lower, false, false)}
	}
	return r, nil
}

func npmPartialBounds(version string) (string, string, error) {
	version = strings.TrimSpace(version)
	if version == "" || version == "*" || version == "x" || version == "X" {
		return "", "", nil
	}
	version = strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
	segments := strings.Split(version, ".")
	if len(segments) > 3 { //nolint:mnd
		return "", "", fmt.Errorf("invalid npm partial version: %s", version)
	}
	parts := make([]int, 0, len(segments))
	wildcard := false
	for _, segment := range segments {
		if segment == "*" || strings.EqualFold(segment, "x") {
			wildcard = true
			continue
		}
		if wildcard || !isDigits(segment) {
			return "", "", fmt.Errorf("invalid npm partial version: %s", version)
		}
		value, err := strconv.Atoi(segment)
		if err != nil {
			return "", "", fmt.Errorf("invalid npm partial version: %s", version)
		}
		parts = append(parts, value)
	}
	if len(parts) == 0 {
		return "", "", nil
	}
	lowerParts := append([]int(nil), parts...)
	for len(lowerParts) < 3 { //nolint:mnd
		lowerParts = append(lowerParts, 0)
	}
	upperParts := append([]int(nil), lowerParts...)
	upperParts[len(parts)-1]++
	for index := len(parts); index < len(upperParts); index++ {
		upperParts[index] = 0
	}
	return fmt.Sprintf("%d.%d.%d", lowerParts[0], lowerParts[1], lowerParts[2]),
		fmt.Sprintf("%d.%d.%d", upperParts[0], upperParts[1], upperParts[2]), nil
}

func isPartialNpmVersion(version string) bool {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
	segments := strings.Split(version, ".")
	if len(segments) > 3 { //nolint:mnd
		return false
	}
	partial := len(segments) < 3 //nolint:mnd
	for _, segment := range segments {
		if segment == "*" || strings.EqualFold(segment, "x") {
			partial = true
			continue
		}
		if !isDigits(segment) {
			return false
		}
	}
	return partial
}

func (p *Parser) parseNpmHyphenRange(lower, upper string) (*Range, error) {
	min := lower
	if isPartialNpmVersion(lower) {
		var err error
		min, _, err = npmPartialBounds(lower)
		if err != nil {
			return nil, err
		}
	}
	max := upper
	maxInclusive := true
	if isPartialNpmVersion(upper) {
		var err error
		_, max, err = npmPartialBounds(upper)
		if err != nil {
			return nil, err
		}
		maxInclusive = false
	}
	return NewRange([]Interval{NewInterval(min, max, true, maxInclusive)}), nil
}

// gem: ~> 1.2, >= 1.0, < 2.0
func (p *Parser) parseGemRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)

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

	// Pessimistic operator: ~> 1.2.3
	if strings.HasPrefix(s, "~>") {
		version := strings.TrimSpace(s[2:])
		return p.parseGemPessimisticRange(version)
	}

	// Standard constraint
	return p.parseConstraints(s, schemeGem)
}

func (p *Parser) parseGemPessimisticRange(version string) (*Range, error) {
	if !validVersionForScheme(version, schemeGem) {
		return nil, fmt.Errorf("invalid gem version: %s", version)
	}
	segments := parseGemRawSegments(version)
	var release []string
	for _, segment := range segments {
		if !segment.num {
			break
		}
		release = append(release, segment.value)
	}
	if len(release) == 0 {
		return nil, fmt.Errorf("invalid gem version: %s", version)
	}
	var upper string
	if len(release) == 1 {
		upper = incNumStr(release[0])
	} else {
		head := append([]string(nil), release[:len(release)-1]...)
		head[len(head)-1] = incNumStr(head[len(head)-1])
		upper = strings.Join(head, ".")
	}
	upper += ".a"
	return &Range{
		Intervals: []Interval{NewInterval(version, upper, true, false)},
		// RubyGems applies the upper bound to Version#release for ~>, while
		// the established VERS rendering uses the bare bumped version.
		RawConstraints: []Interval{NewInterval(version, strings.TrimSuffix(upper, ".a"), true, false)},
		Scheme:         schemeGem,
	}, nil
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
	if segments >= 3 { //nolint:mnd
		// ~> 1.2.3 bumps minor: < 1.3
		upper = fmt.Sprintf("%d.%d", v.Major, v.Minor+1)
	} else {
		// ~> 1.2 or ~> 1 bumps major: < 2.0
		upper = fmt.Sprintf("%d.0", v.Major+1)
	}

	return NewRange([]Interval{
		NewInterval(version, upper, true, false),
	}), nil
}

// pypi: >=1.0,<2.0, ~=1.4.2, !=1.5.0
func (p *Parser) parsePypiRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)

	// Comma is AND in PEP 440: parse each specifier and intersect.
	if strings.Contains(s, ",") {
		var result *Range
		for _, part := range strings.Split(s, ",") {
			r, err := p.parsePypiRange(part)
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

	// Compatible release: ~=1.4.2
	if strings.HasPrefix(s, "~=") {
		version := strings.TrimSpace(s[2:])
		return p.parsePypiCompatibleRelease(version)
	}
	if strings.HasPrefix(s, "===") {
		return nil, fmt.Errorf("pypi arbitrary equality constraints are not supported: %s", s)
	}
	if strings.HasPrefix(s, "==") && strings.HasSuffix(strings.TrimSpace(s[2:]), ".*") {
		return parsePypiPrefixRange(strings.TrimSpace(s[2:]), false)
	}
	if strings.HasPrefix(s, "!=") && strings.HasSuffix(strings.TrimSpace(s[2:]), ".*") {
		return parsePypiPrefixRange(strings.TrimSpace(s[2:]), true)
	}
	if strings.HasPrefix(s, "==") {
		s = "=" + strings.TrimSpace(s[2:])
	}

	return p.parseConstraints(s, schemePyPI)
}

func parsePypiPrefixRange(version string, exclude bool) (*Range, error) {
	prefix := strings.TrimSuffix(version, ".*")
	v, ok := parsePEP440(prefix)
	if !ok || v.hasPre || v.hasPost || v.hasDev || len(v.local) > 0 {
		return nil, fmt.Errorf("invalid pypi prefix constraint: %s", version)
	}
	lower := prefix + ".dev0"
	upperRelease := append([]string(nil), v.release...)
	upperRelease[len(upperRelease)-1] = incNumStr(upperRelease[len(upperRelease)-1])
	upper := strings.Join(upperRelease, ".") + ".dev0"
	if bang := strings.IndexByte(prefix, '!'); bang >= 0 {
		epoch := trimLeadingZeros(v.epoch)
		lower = epoch + "!" + prefix[bang+1:] + ".dev0"
		upper = epoch + "!" + upper
	}
	if !exclude {
		return &Range{
			Intervals: []Interval{NewInterval(lower, upper, true, false)}, Scheme: schemePyPI,
		}, nil
	}
	return &Range{
		Intervals: []Interval{
			NewInterval("", lower, false, false),
			NewInterval(upper, "", true, false),
		},
		Scheme: schemePyPI,
	}, nil
}

// parsePypiCompatibleRelease handles PEP 440 ~= by deriving the upper bound
// from release segments (ignoring pre/post/dev) and preserving the epoch.
func (p *Parser) parsePypiCompatibleRelease(version string) (*Range, error) {
	upper, ok := pep440CompatibleUpper(version)
	var r *Range
	if ok {
		r = NewRange([]Interval{NewInterval(version, upper, true, false)})
	} else {
		// Fall back to the generic pessimistic algorithm when the
		// version is not valid PEP 440 or has fewer than two release
		// segments.
		var err error
		r, err = p.parsePessimisticRange(version)
		if err != nil {
			return nil, err
		}
	}
	r.Scheme = schemePyPI
	return r, nil
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

	return p.parseConstraints(s, schemeMaven)
}

func (p *Parser) parseBracketRange(s string) (*Range, error) {
	minInclusive := s[0] == '['
	maxInclusive := s[len(s)-1] == ']'

	inner := s[1 : len(s)-1]
	parts := strings.SplitN(inner, ",", 2) //nolint:mnd

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
	s = strings.TrimSpace(s)
	if strings.Contains(s, ",") {
		var result *Range
		for _, part := range strings.Split(s, ",") {
			r, err := p.parseCargoRange(part)
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
	if s != "" && !strings.ContainsAny(s, "^~*<>=") {
		return p.parseCaretRange(s)
	}
	if operator, version := extractOperator(s); operator == "<" && isPartialNpmVersion(version) {
		lower, _, err := npmPartialBounds(version)
		if err != nil {
			return nil, err
		}
		return NewRange([]Interval{NewInterval("", lower+"-0", false, false)}), nil
	}
	return p.parseNpmRange(s)
}

// go: >=1.0.0, <2.0.0
func (p *Parser) parseGoRange(s string) (*Range, error) {
	// Go uses comma-separated constraints
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		var result *Range
		for _, part := range parts {
			constraint, err := parseConstraintWithScheme(strings.TrimSpace(part), schemeGo)
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

	return p.parseConstraints(s, schemeGo)
}

// hex/elixir: ~> 1.2.3, >= 1.0.0 and < 2.0.0, ~> 1.0 or ~> 2.0
func (p *Parser) parseHexRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)

	// Handle "or" disjunction -- collect all parts, then merge once
	if strings.Contains(s, " or ") {
		parts := strings.Split(s, " or ")
		var allIntervals []Interval
		var allRaw []Interval
		for _, part := range parts {
			r, err := p.parseHexSingleRange(strings.TrimSpace(part))
			if err != nil {
				return nil, err
			}
			allIntervals = append(allIntervals, r.Intervals...)
			if len(r.RawConstraints) > 0 {
				allRaw = append(allRaw, r.RawConstraints...)
			} else {
				allRaw = append(allRaw, r.Intervals...)
			}
		}
		return &Range{
			Intervals:      mergeIntervals(allIntervals, CompareVersions),
			RawConstraints: allRaw,
		}, nil
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
	return p.parseConstraints(s, schemeDeb)
}

// rpm: >= 1.0, <= 2.0
func (p *Parser) parseRpmRange(s string) (*Range, error) {
	return p.parseConstraints(s, schemeRPM)
}

// conan: >1 <2, ~1.2, ^1.2.3, ||
func (p *Parser) parseConanRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ","); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if s == "*" || s == "*-" {
		return NewRange([]Interval{GreaterThanInterval("0.0.0", true)}), nil
	}
	if strings.Contains(s, "||") {
		var intervals, raw []Interval
		for _, part := range strings.Split(s, "||") {
			r, err := p.parseConanRange(part)
			if err != nil {
				return nil, err
			}
			intervals = append(intervals, r.Intervals...)
			if len(r.RawConstraints) > 0 {
				raw = append(raw, r.RawConstraints...)
			} else {
				raw = append(raw, r.Intervals...)
			}
		}
		return &Range{Intervals: mergeIntervals(intervals, compareConan), RawConstraints: raw}, nil
	}
	if strings.Contains(s, " ") {
		var result *Range
		for _, part := range tokenizeNpmConstraints(s) {
			r, err := p.parseConanRange(part)
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

	s = strings.TrimSuffix(s, "-")
	if strings.HasPrefix(s, "~") || strings.HasPrefix(s, "^") {
		operator, version := s[0], strings.TrimSpace(s[1:])
		upper, err := conanUpperBound(version, operator)
		if err != nil {
			return nil, err
		}
		return NewRange([]Interval{NewInterval(version, upper, true, false)}), nil
	}
	return p.parseConstraints(s, schemeConan)
}

func conanUpperBound(version string, operator byte) (string, error) {
	parts := strings.Split(version, ".")
	for _, part := range parts {
		if !isDigits(part) {
			return "", fmt.Errorf("invalid conan version range: %c%s", operator, version)
		}
	}

	index := 0
	if operator == '~' && len(parts) > 1 {
		index = 1
	} else if operator == '^' {
		for index < len(parts)-1 && cmpNumStr(parts[index], "0") == 0 {
			index++
		}
	}
	upper := append([]string(nil), parts[:index+1]...)
	upper[index] = incNumStr(upper[index])
	return strings.Join(upper, ".") + "-", nil
}

// openssl native ranges are comma-separated exact releases.
func (p *Parser) parseOpenSSLRange(s string) (*Range, error) {
	return p.parseExactList(s, schemeOpenSSL)
}

func (p *Parser) parseExactList(s, scheme string) (*Range, error) {
	parts := strings.Split(s, ",")
	intervals := make([]Interval, 0, len(parts))
	for _, part := range parts {
		version := strings.TrimSpace(part)
		if version == "" {
			return nil, fmt.Errorf("empty %s version", scheme)
		}
		if !validVersionForScheme(version, scheme) {
			return nil, fmt.Errorf("invalid %s version: %s", scheme, version)
		}
		intervals = append(intervals, ExactInterval(version))
	}
	return NewRange(intervals), nil
}

// nginx: 0.8.40+, 0.7.52-0.8.39, comma-separated unions.
func (p *Parser) parseNginxRange(s string) (*Range, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, ",") {
		var intervals, raw []Interval
		for _, part := range strings.Split(s, ",") {
			r, err := p.parseNginxRange(part)
			if err != nil {
				return nil, err
			}
			intervals = append(intervals, r.Intervals...)
			raw = append(raw, r.Intervals...)
		}
		return &Range{Intervals: mergeIntervals(intervals, compareSemver), RawConstraints: raw}, nil
	}
	if strings.HasSuffix(s, "+") {
		version := strings.TrimSuffix(s, "+")
		if !validSemverLike(version) {
			return nil, fmt.Errorf("invalid nginx version range: %s", s)
		}
		parts := strings.Split(version, ".")
		if len(parts) < 2 || !isDigits(parts[1]) {
			return nil, fmt.Errorf("invalid nginx version range: %s", s)
		}
		interval := GreaterThanInterval(version, true)
		minor := trimLeadingZeros(parts[1])
		if minor[len(minor)-1]%2 == 0 {
			interval.Max = strings.Join([]string{parts[0], incNumStr(parts[1]), "0"}, ".")
		}
		return NewRange([]Interval{interval}), nil
	}
	if nginxRangeRegex.MatchString(s) {
		parts := strings.SplitN(s, "-", 2) //nolint:mnd
		return NewRange([]Interval{NewInterval(parts[0], parts[1], true, true)}), nil
	}
	return p.parseConstraints(s, schemeNginx)
}
