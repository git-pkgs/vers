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
	if r.IsUnbounded() && len(r.Exclusions) == 0 {
		return fmt.Sprintf("vers:%s/*", scheme)
	}
	if r.IsEmpty() {
		return fmt.Sprintf("vers:%s/", scheme)
	}

	var constraints []string
	for _, interval := range r.Intervals {
		if interval.Min == interval.Max && interval.MinInclusive && interval.MaxInclusive && interval.Min != "" {
			constraints = append(constraints, "="+interval.Min)
		} else {
			if interval.Min != "" {
				op := ">"
				if interval.MinInclusive {
					op = ">="
				}
				constraints = append(constraints, op+interval.Min)
			}
			if interval.Max != "" {
				op := "<"
				if interval.MaxInclusive {
					op = "<="
				}
				constraints = append(constraints, op+interval.Max)
			}
		}
	}

	return fmt.Sprintf("vers:%s/%s", scheme, strings.Join(constraints, "|"))
}

func (p *Parser) parseConstraints(constraintsStr, scheme string) (*Range, error) {
	parts := strings.Split(constraintsStr, "|")
	var result *Range
	var exclusions []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		constraint, err := ParseConstraint(part)
		if err != nil {
			return nil, err
		}

		if constraint.IsExclusion() {
			exclusions = append(exclusions, constraint.Version)
		} else {
			interval, ok := constraint.ToInterval()
			if ok {
				r := NewRange([]Interval{interval})
				if result == nil {
					result = r
				} else {
					result = result.Intersect(r)
				}
			}
		}
	}

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
			r, err := p.parseNpmSingleRange(strings.TrimSpace(part))
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
		parts := strings.Fields(s)
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

	// X-range: 1.x, 1.2.x
	if strings.HasSuffix(s, ".x") || strings.HasSuffix(s, ".X") || strings.HasSuffix(s, ".*") {
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
func (p *Parser) parseTildeRange(version string) (*Range, error) {
	v, err := ParseVersion(version)
	if err != nil {
		return nil, err
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

// ~> 1.2.3 := >= 1.2.3, < 1.3.0
func (p *Parser) parsePessimisticRange(version string) (*Range, error) {
	v, err := ParseVersion(version)
	if err != nil {
		return nil, err
	}

	var upper string
	if v.Patch > 0 {
		upper = fmt.Sprintf("%d.%d.0", v.Major, v.Minor+1)
	} else if v.Minor > 0 {
		upper = fmt.Sprintf("%d.0.0", v.Major+1)
	} else {
		upper = fmt.Sprintf("%d.0.0", v.Major+1)
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
			constraint, err := ParseConstraint(strings.TrimSpace(part))
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
