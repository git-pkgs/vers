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
	if i.Min != "" && i.Max != "" {
		cmp := CompareVersions(i.Min, i.Max)
		if cmp > 0 {
			return true
		}
		if cmp == 0 && (!i.MinInclusive || !i.MaxInclusive) {
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
	if i.IsEmpty() {
		return false
	}
	if i.IsUnbounded() {
		return true
	}

	// Check minimum bound
	if i.Min != "" {
		cmp := CompareVersions(version, i.Min)
		if i.MinInclusive {
			if cmp < 0 {
				return false
			}
		} else {
			if cmp <= 0 {
				return false
			}
		}
	}

	// Check maximum bound
	if i.Max != "" {
		cmp := CompareVersions(version, i.Max)
		if i.MaxInclusive {
			if cmp > 0 {
				return false
			}
		} else {
			if cmp >= 0 {
				return false
			}
		}
	}

	return true
}

// Intersect returns the intersection of two intervals.
func (i Interval) Intersect(other Interval) Interval {
	if i.IsEmpty() || other.IsEmpty() {
		return EmptyInterval()
	}

	result := Interval{}

	// Determine new minimum
	if i.Min != "" && other.Min != "" {
		cmp := CompareVersions(i.Min, other.Min)
		if cmp > 0 {
			result.Min = i.Min
			result.MinInclusive = i.MinInclusive
		} else if cmp < 0 {
			result.Min = other.Min
			result.MinInclusive = other.MinInclusive
		} else {
			result.Min = i.Min
			result.MinInclusive = i.MinInclusive && other.MinInclusive
		}
	} else if i.Min != "" {
		result.Min = i.Min
		result.MinInclusive = i.MinInclusive
	} else if other.Min != "" {
		result.Min = other.Min
		result.MinInclusive = other.MinInclusive
	}

	// Determine new maximum
	if i.Max != "" && other.Max != "" {
		cmp := CompareVersions(i.Max, other.Max)
		if cmp < 0 {
			result.Max = i.Max
			result.MaxInclusive = i.MaxInclusive
		} else if cmp > 0 {
			result.Max = other.Max
			result.MaxInclusive = other.MaxInclusive
		} else {
			result.Max = i.Max
			result.MaxInclusive = i.MaxInclusive && other.MaxInclusive
		}
	} else if i.Max != "" {
		result.Max = i.Max
		result.MaxInclusive = i.MaxInclusive
	} else if other.Max != "" {
		result.Max = other.Max
		result.MaxInclusive = other.MaxInclusive
	}

	return result
}

// Overlaps returns true if the two intervals overlap.
func (i Interval) Overlaps(other Interval) bool {
	if i.IsEmpty() || other.IsEmpty() {
		return false
	}
	return !i.Intersect(other).IsEmpty()
}

// Adjacent returns true if the two intervals are adjacent (can be merged).
func (i Interval) Adjacent(other Interval) bool {
	if i.IsEmpty() || other.IsEmpty() {
		return false
	}

	if i.Max != "" && other.Min != "" && CompareVersions(i.Max, other.Min) == 0 {
		return (i.MaxInclusive && !other.MinInclusive) || (!i.MaxInclusive && other.MinInclusive)
	}

	if i.Min != "" && other.Max != "" && CompareVersions(i.Min, other.Max) == 0 {
		return (i.MinInclusive && !other.MaxInclusive) || (!i.MinInclusive && other.MaxInclusive)
	}

	return false
}

// Union returns the union of two intervals, or nil if they cannot be merged.
func (i Interval) Union(other Interval) *Interval {
	if i.IsEmpty() {
		return &other
	}
	if other.IsEmpty() {
		return &i
	}

	if !i.Overlaps(other) && !i.Adjacent(other) {
		return nil
	}

	result := Interval{}

	// Determine new minimum (take the smaller one, "" means unbounded)
	if i.Min == "" || other.Min == "" {
		// Either side is unbounded below, so the union is too
		result.Min = ""
		result.MinInclusive = false
	} else {
		cmp := CompareVersions(i.Min, other.Min)
		if cmp < 0 {
			result.Min = i.Min
			result.MinInclusive = i.MinInclusive
		} else if cmp > 0 {
			result.Min = other.Min
			result.MinInclusive = other.MinInclusive
		} else {
			result.Min = i.Min
			result.MinInclusive = i.MinInclusive || other.MinInclusive
		}
	}

	// Determine new maximum (take the larger one, "" means unbounded)
	if i.Max == "" || other.Max == "" {
		// Either side is unbounded above, so the union is too
		result.Max = ""
		result.MaxInclusive = false
	} else {
		cmp := CompareVersions(i.Max, other.Max)
		if cmp > 0 {
			result.Max = i.Max
			result.MaxInclusive = i.MaxInclusive
		} else if cmp < 0 {
			result.Max = other.Max
			result.MaxInclusive = other.MaxInclusive
		} else {
			result.Max = i.Max
			result.MaxInclusive = i.MaxInclusive || other.MaxInclusive
		}
	}

	return &result
}

// String returns a string representation of the interval.
func (i Interval) String() string {
	if i.IsEmpty() {
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
