package vers

import "strings"

// Range represents a version range as a collection of intervals.
// Multiple intervals represent a union (OR) of ranges.
type Range struct {
	Intervals  []Interval
	Exclusions []string // Versions to exclude (from != constraints)
}

// NewRange creates a new Range from intervals.
func NewRange(intervals []Interval) *Range {
	return &Range{Intervals: intervals}
}

// Contains checks if the range contains the given version.
func (r *Range) Contains(version string) bool {
	// Check exclusions first
	for _, exc := range r.Exclusions {
		if CompareVersions(version, exc) == 0 {
			return false
		}
	}

	// Check if version is in any interval
	for _, interval := range r.Intervals {
		if interval.Contains(version) {
			return true
		}
	}

	return false
}

// IsEmpty returns true if this range matches no versions.
func (r *Range) IsEmpty() bool {
	if len(r.Intervals) == 0 {
		return true
	}
	for _, interval := range r.Intervals {
		if !interval.IsEmpty() {
			return false
		}
	}
	return true
}

// IsUnbounded returns true if this range matches all versions.
func (r *Range) IsUnbounded() bool {
	if len(r.Exclusions) > 0 {
		return false
	}
	for _, interval := range r.Intervals {
		if interval.IsUnbounded() {
			return true
		}
	}
	return false
}

// Union returns a new Range that is the union of this range and another.
func (r *Range) Union(other *Range) *Range {
	if r.IsEmpty() {
		return other
	}
	if other.IsEmpty() {
		return r
	}

	// Combine all intervals
	allIntervals := make([]Interval, 0, len(r.Intervals)+len(other.Intervals))
	allIntervals = append(allIntervals, r.Intervals...)
	allIntervals = append(allIntervals, other.Intervals...)

	// Merge overlapping intervals
	merged := mergeIntervals(allIntervals)

	// Combine exclusions (intersection of exclusions for union)
	exclusions := make([]string, 0)
	for _, e := range r.Exclusions {
		for _, oe := range other.Exclusions {
			if e == oe {
				exclusions = append(exclusions, e)
				break
			}
		}
	}

	return &Range{Intervals: merged, Exclusions: exclusions}
}

// Intersect returns a new Range that is the intersection of this range and another.
func (r *Range) Intersect(other *Range) *Range {
	if r.IsEmpty() || other.IsEmpty() {
		return &Range{}
	}

	// Intersect each pair of intervals
	var result []Interval
	for _, i1 := range r.Intervals {
		for _, i2 := range other.Intervals {
			intersection := i1.Intersect(i2)
			if !intersection.IsEmpty() {
				result = append(result, intersection)
			}
		}
	}

	// Merge overlapping intervals
	merged := mergeIntervals(result)

	// Combine exclusions (union of exclusions for intersection)
	exclusions := make([]string, 0, len(r.Exclusions)+len(other.Exclusions))
	exclusions = append(exclusions, r.Exclusions...)
	for _, e := range other.Exclusions {
		found := false
		for _, existing := range exclusions {
			if e == existing {
				found = true
				break
			}
		}
		if !found {
			exclusions = append(exclusions, e)
		}
	}

	return &Range{Intervals: merged, Exclusions: exclusions}
}

// Exclude returns a new Range that excludes the given version.
func (r *Range) Exclude(version string) *Range {
	exclusions := make([]string, len(r.Exclusions), len(r.Exclusions)+1)
	copy(exclusions, r.Exclusions)
	exclusions = append(exclusions, version)

	return &Range{
		Intervals:  r.Intervals,
		Exclusions: exclusions,
	}
}

// String returns a string representation of the range.
func (r *Range) String() string {
	if r.IsEmpty() {
		return "empty"
	}
	if r.IsUnbounded() && len(r.Exclusions) == 0 {
		return "*"
	}

	var parts []string
	for _, interval := range r.Intervals {
		parts = append(parts, interval.String())
	}

	result := strings.Join(parts, " | ")

	if len(r.Exclusions) > 0 {
		result += " excluding " + strings.Join(r.Exclusions, ", ")
	}

	return result
}

// mergeIntervals merges overlapping intervals into a minimal set.
func mergeIntervals(intervals []Interval) []Interval {
	if len(intervals) <= 1 {
		return intervals
	}

	// Simple implementation: try to merge each pair
	result := make([]Interval, 0, len(intervals))

	for _, interval := range intervals {
		if interval.IsEmpty() {
			continue
		}

		merged := false
		for i, existing := range result {
			if union := existing.Union(interval); union != nil {
				result[i] = *union
				merged = true
				break
			}
		}
		if !merged {
			result = append(result, interval)
		}
	}

	return result
}
