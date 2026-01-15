// Package vers provides version range parsing and comparison according to the VERS specification.
//
// VERS (Version Range Specification) is a universal format for expressing version ranges
// across different package ecosystems. This package supports parsing vers URIs, native
// package manager syntax, and provides version comparison functionality.
//
// Quick Start:
//
//	// Parse a vers URI
//	r, _ := vers.Parse("vers:npm/>=1.2.3|<2.0.0")
//	r.Contains("1.5.0") // true
//
//	// Parse native package manager syntax
//	r, _ = vers.ParseNative("^1.2.3", "npm")
//
//	// Check if version satisfies constraint
//	vers.Satisfies("1.5.0", ">=1.0.0,<2.0.0", "npm") // true
//
//	// Compare versions
//	vers.Compare("1.2.3", "1.2.4") // -1
//
// See https://github.com/package-url/purl-spec/blob/main/VERSION-RANGE-SPEC.rst
package vers

// Version is the library version.
const Version = "0.1.0"

// Parse parses a vers URI string into a Range.
//
// The vers URI format is: vers:<scheme>/<constraints>
// For example: vers:npm/>=1.2.3|<2.0.0
//
// Use vers:<scheme>/* for an unbounded range that matches all versions.
func Parse(versURI string) (*Range, error) {
	return defaultParser.Parse(versURI)
}

// ParseNative parses a native package manager version range into a Range.
//
// Supported schemes:
//   - npm: ^1.2.3, ~1.2.3, 1.2.3 - 2.0.0, >=1.0.0 <2.0.0, ||
//   - gem/rubygems: ~> 1.2, >= 1.0, < 2.0
//   - pypi: >=1.0,<2.0, ~=1.4.2, !=1.5.0
//   - maven: [1.0,2.0), (1.0,2.0], [1.0,)
//   - nuget: [1.0,2.0), (1.0,2.0]
//   - cargo: ^1.2.3, ~1.2.3, >=1.0.0, <2.0.0
//   - go: >=1.0.0, <2.0.0
//   - deb/debian: >= 1.0, << 2.0
//   - rpm: >= 1.0, <= 2.0
func ParseNative(constraint string, scheme string) (*Range, error) {
	return defaultParser.ParseNative(constraint, scheme)
}

// Satisfies checks if a version satisfies a constraint.
//
// If scheme is empty, constraint is parsed as a vers URI.
// Otherwise, constraint is parsed as native package manager syntax.
func Satisfies(version, constraint, scheme string) (bool, error) {
	var r *Range
	var err error

	if scheme == "" {
		r, err = Parse(constraint)
	} else {
		r, err = ParseNative(constraint, scheme)
	}
	if err != nil {
		return false, err
	}

	return r.Contains(version), nil
}

// Compare compares two version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func Compare(a, b string) int {
	return CompareVersions(a, b)
}

// Valid checks if a version string is valid.
func Valid(version string) bool {
	_, err := ParseVersion(version)
	return err == nil
}

// Normalize normalizes a version string to a consistent format.
func Normalize(version string) (string, error) {
	v, err := ParseVersion(version)
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

// Exact creates a range that matches only the specified version.
func Exact(version string) *Range {
	return NewRange([]Interval{ExactInterval(version)})
}

// GreaterThan creates a range for versions greater than (or equal to) the specified version.
func GreaterThan(version string, inclusive bool) *Range {
	return NewRange([]Interval{GreaterThanInterval(version, inclusive)})
}

// LessThan creates a range for versions less than (or equal to) the specified version.
func LessThan(version string, inclusive bool) *Range {
	return NewRange([]Interval{LessThanInterval(version, inclusive)})
}

// Unbounded creates a range that matches all versions.
func Unbounded() *Range {
	return NewRange([]Interval{UnboundedInterval()})
}

// Empty creates a range that matches no versions.
func Empty() *Range {
	return NewRange([]Interval{EmptyInterval()})
}

// ToVersString converts a Range back to a vers URI string.
func ToVersString(r *Range, scheme string) string {
	return defaultParser.ToVersString(r, scheme)
}

var defaultParser = NewParser()
