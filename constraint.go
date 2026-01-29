package vers

import (
	"fmt"
	"regexp"
	"strings"
)

// Valid constraint operators.
var ValidOperators = []string{"=", "!=", "<", "<=", ">", ">="}

var operatorRegex = regexp.MustCompile(`^(!=|>=|<=|[<>=])`)

// Constraint represents a single version constraint (e.g., ">=1.2.3").
type Constraint struct {
	Operator string
	Version  string
}

// ParseConstraint parses a constraint string into a Constraint.
func ParseConstraint(s string) (*Constraint, error) {
	return parseConstraintWithScheme(s, "")
}

// parseConstraintWithScheme parses a constraint with scheme-specific handling.
// For Go/golang schemes, the v prefix is preserved.
func parseConstraintWithScheme(s, scheme string) (*Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty constraint")
	}

	// Go versions preserve the v prefix
	preserveVPrefix := scheme == "go" || scheme == "golang"

	matches := operatorRegex.FindStringSubmatch(s)
	if matches != nil {
		operator := matches[1]
		version := strings.TrimSpace(s[len(operator):])
		if version == "" {
			return nil, fmt.Errorf("invalid constraint format: %s", s)
		}
		if !preserveVPrefix {
			version = stripVPrefix(version)
		}
		return &Constraint{Operator: operator, Version: version}, nil
	}

	// No operator found, treat as exact match
	version := s
	if !preserveVPrefix {
		version = stripVPrefix(s)
	}
	return &Constraint{Operator: "=", Version: version}, nil
}

// stripVPrefix removes a leading 'v' or 'V' from version strings.
func stripVPrefix(version string) string {
	if len(version) > 1 && (version[0] == 'v' || version[0] == 'V') {
		return version[1:]
	}
	return version
}

// ToInterval converts this constraint to an interval.
// Returns nil for exclusion constraints (!=).
func (c *Constraint) ToInterval() (Interval, bool) {
	switch c.Operator {
	case "=":
		return ExactInterval(c.Version), true
	case "!=":
		// Exclusions need special handling in ranges
		return Interval{}, false
	case ">":
		return GreaterThanInterval(c.Version, false), true
	case ">=":
		return GreaterThanInterval(c.Version, true), true
	case "<":
		return LessThanInterval(c.Version, false), true
	case "<=":
		return LessThanInterval(c.Version, true), true
	default:
		return Interval{}, false
	}
}

// IsExclusion returns true if this is an exclusion constraint (!=).
func (c *Constraint) IsExclusion() bool {
	return c.Operator == "!="
}

// Satisfies checks if a version satisfies this constraint.
func (c *Constraint) Satisfies(version string) bool {
	cmp := CompareVersions(version, c.Version)

	switch c.Operator {
	case "=":
		return cmp == 0
	case "!=":
		return cmp != 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	default:
		return false
	}
}

// String returns the constraint as a string.
func (c *Constraint) String() string {
	return c.Operator + c.Version
}
