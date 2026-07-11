package vers

import "fmt"

// Interval represents a mathematical interval of versions.
// For example, [1.0.0, 2.0.0) represents versions from 1.0.0 (inclusive) to 2.0.0 (exclusive).
type Interval struct {
	Min          string
	Max          string
	MinInclusive bool
	MaxInclusive bool
}

// NewInterval creates a new interval with the given bounds.
func NewInterval(min, max string, minInclusive, maxInclusive bool) Interval {
	return Interval{
		Min:          min,
		Max:          max,
		MinInclusive: minInclusive,
		MaxInclusive: maxInclusive,
	}
}

// EmptyInterval creates an interval that matches no versions.
func EmptyInterval() Interval {
	return Interval{Min: "1", Max: "0", MinInclusive: true, MaxInclusive: true}
}

// UnboundedInterval creates an interval that matches all versions.
func UnboundedInterval() Interval {
	return Interval{}
}

// ExactInterval creates an interval that matches exactly one version.
func ExactInterval(version string) Interval {
	return Interval{Min: version, Max: version, MinInclusive: true, MaxInclusive: true}
}

// GreaterThanInterval creates an interval for versions greater than the given version.
func GreaterThanInterval(version string, inclusive bool) Interval {
	return Interval{Min: version, MinInclusive: inclusive}
}

// LessThanInterval creates an interval for versions less than the given version.
func LessThanInterval(version string, inclusive bool) Interval {
	return Interval{Max: version, MaxInclusive: inclusive}
}

// IsEmpty returns true if this interval matches no versions.
func (i Interval) IsEmpty() bool {
	return i.isEmptyCmp(CompareVersions)
}

// IsEmptyWithScheme reports whether the interval is empty under a version scheme.
func (i Interval) IsEmptyWithScheme(scheme string) bool {
	return i.isEmptyCmp(compareFuncFor(scheme))
}

func (i Interval) isEmptyCmp(cmp func(a, b string) int) bool {
	if i.Min != "" && i.Max != "" {
		c := cmp(i.Min, i.Max)
		if c > 0 {
			return true
		}
		if c == 0 && (!i.MinInclusive || !i.MaxInclusive) {
			return true
		}
	}
	return false
}

// IsUnbounded returns true if this interval matches all versions.
func (i Interval) IsUnbounded() bool {
	return i.Min == "" && i.Max == ""
}

// Contains checks if the interval contains the given version.
func (i Interval) Contains(version string) bool {
	return i.containsCmp(version, CompareVersions)
}

// ContainsWithScheme checks containment under a version scheme.
func (i Interval) ContainsWithScheme(version, scheme string) bool {
	return i.containsCmp(version, compareFuncFor(scheme))
}

func (i Interval) containsCmp(version string, cmp func(a, b string) int) bool {
	if i.isEmptyCmp(cmp) {
		return false
	}
	if i.IsUnbounded() {
		return true
	}

	// Check minimum bound
	if i.Min != "" {
		c := cmp(version, i.Min)
		if i.MinInclusive {
			if c < 0 {
				return false
			}
		} else {
			if c <= 0 {
				return false
			}
		}
	}

	// Check maximum bound
	if i.Max != "" {
		c := cmp(version, i.Max)
		if i.MaxInclusive {
			if c > 0 {
				return false
			}
		} else {
			if c >= 0 {
				return false
			}
		}
	}

	return true
}

// Intersect returns the intersection of two intervals.
func (i Interval) Intersect(other Interval) Interval {
	return i.intersectCmp(other, CompareVersions)
}

// IntersectWithScheme returns the intersection under a version scheme.
func (i Interval) IntersectWithScheme(other Interval, scheme string) Interval {
	return i.intersectCmp(other, compareFuncFor(scheme))
}

func (i Interval) intersectCmp(other Interval, cmp func(a, b string) int) Interval {
	if i.isEmptyCmp(cmp) || other.isEmptyCmp(cmp) {
		return EmptyInterval()
	}

	result := Interval{}

	// Determine new minimum
	switch {
	case i.Min != "" && other.Min != "":
		c := cmp(i.Min, other.Min)
		switch {
		case c > 0:
			result.Min = i.Min
			result.MinInclusive = i.MinInclusive
		case c < 0:
			result.Min = other.Min
			result.MinInclusive = other.MinInclusive
		default:
			result.Min = i.Min
			result.MinInclusive = i.MinInclusive && other.MinInclusive
		}
	case i.Min != "":
		result.Min = i.Min
		result.MinInclusive = i.MinInclusive
	case other.Min != "":
		result.Min = other.Min
		result.MinInclusive = other.MinInclusive
	}

	// Determine new maximum
	switch {
	case i.Max != "" && other.Max != "":
		c := cmp(i.Max, other.Max)
		switch {
		case c < 0:
			result.Max = i.Max
			result.MaxInclusive = i.MaxInclusive
		case c > 0:
			result.Max = other.Max
			result.MaxInclusive = other.MaxInclusive
		default:
			result.Max = i.Max
			result.MaxInclusive = i.MaxInclusive && other.MaxInclusive
		}
	case i.Max != "":
		result.Max = i.Max
		result.MaxInclusive = i.MaxInclusive
	case other.Max != "":
		result.Max = other.Max
		result.MaxInclusive = other.MaxInclusive
	}

	return result
}

// Overlaps returns true if the two intervals overlap.
func (i Interval) Overlaps(other Interval) bool {
	return i.overlapsCmp(other, CompareVersions)
}

// OverlapsWithScheme reports overlap under a version scheme.
func (i Interval) OverlapsWithScheme(other Interval, scheme string) bool {
	return i.overlapsCmp(other, compareFuncFor(scheme))
}

func (i Interval) overlapsCmp(other Interval, cmp func(a, b string) int) bool {
	if i.isEmptyCmp(cmp) || other.isEmptyCmp(cmp) {
		return false
	}
	return !i.intersectCmp(other, cmp).isEmptyCmp(cmp)
}

// Adjacent returns true if the two intervals are adjacent (can be merged).
func (i Interval) Adjacent(other Interval) bool {
	return i.adjacentCmp(other, CompareVersions)
}

// AdjacentWithScheme reports adjacency under a version scheme.
func (i Interval) AdjacentWithScheme(other Interval, scheme string) bool {
	return i.adjacentCmp(other, compareFuncFor(scheme))
}

func (i Interval) adjacentCmp(other Interval, cmp func(a, b string) int) bool {
	if i.isEmptyCmp(cmp) || other.isEmptyCmp(cmp) {
		return false
	}

	if i.Max != "" && other.Min != "" && cmp(i.Max, other.Min) == 0 {
		return (i.MaxInclusive && !other.MinInclusive) || (!i.MaxInclusive && other.MinInclusive)
	}

	if i.Min != "" && other.Max != "" && cmp(i.Min, other.Max) == 0 {
		return (i.MinInclusive && !other.MaxInclusive) || (!i.MinInclusive && other.MaxInclusive)
	}

	return false
}

// Union returns the union of two intervals, or nil if they cannot be merged.
func (i Interval) Union(other Interval) *Interval {
	return i.unionCmp(other, CompareVersions)
}

// UnionWithScheme returns the union under a version scheme.
func (i Interval) UnionWithScheme(other Interval, scheme string) *Interval {
	return i.unionCmp(other, compareFuncFor(scheme))
}

func (i Interval) unionCmp(other Interval, cmp func(a, b string) int) *Interval {
	if i.isEmptyCmp(cmp) {
		return &other
	}
	if other.isEmptyCmp(cmp) {
		return &i
	}

	if !i.overlapsCmp(other, cmp) && !i.adjacentCmp(other, cmp) {
		return nil
	}

	result := Interval{}

	// Determine new minimum (take the smaller one, "" means unbounded)
	if i.Min == "" || other.Min == "" {
		result.Min = ""
		result.MinInclusive = false
	} else {
		c := cmp(i.Min, other.Min)
		switch {
		case c < 0:
			result.Min = i.Min
			result.MinInclusive = i.MinInclusive
		case c > 0:
			result.Min = other.Min
			result.MinInclusive = other.MinInclusive
		default:
			result.Min = i.Min
			result.MinInclusive = i.MinInclusive || other.MinInclusive
		}
	}

	// Determine new maximum (take the larger one, "" means unbounded)
	if i.Max == "" || other.Max == "" {
		result.Max = ""
		result.MaxInclusive = false
	} else {
		c := cmp(i.Max, other.Max)
		switch {
		case c > 0:
			result.Max = i.Max
			result.MaxInclusive = i.MaxInclusive
		case c < 0:
			result.Max = other.Max
			result.MaxInclusive = other.MaxInclusive
		default:
			result.Max = i.Max
			result.MaxInclusive = i.MaxInclusive || other.MaxInclusive
		}
	}

	return &result
}

// String returns a string representation of the interval.
func (i Interval) String() string {
	return i.stringCmp(CompareVersions)
}

// StringWithScheme formats the interval under a version scheme.
func (i Interval) StringWithScheme(scheme string) string {
	return i.stringCmp(compareFuncFor(scheme))
}

func (i Interval) stringCmp(cmp func(a, b string) int) string {
	if i.isEmptyCmp(cmp) {
		return "empty"
	}
	if i.IsUnbounded() {
		return "(-inf,+inf)"
	}

	minBracket := "("
	if i.MinInclusive {
		minBracket = "["
	}
	maxBracket := ")"
	if i.MaxInclusive {
		maxBracket = "]"
	}

	minStr := "-inf"
	if i.Min != "" {
		minStr = i.Min
	}
	maxStr := "+inf"
	if i.Max != "" {
		maxStr = i.Max
	}

	return fmt.Sprintf("%s%s,%s%s", minBracket, minStr, maxStr, maxBracket)
}
