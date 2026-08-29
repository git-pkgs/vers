package vers

import (
	"fmt"
	"sort"
	"strings"
)

// Range represents a version range as a collection of intervals.
// Multiple intervals represent a union (OR) of ranges.
type Range struct {
	Intervals  []Interval
	Exclusions []string // Versions to exclude (from != constraints)
	// RawConstraints stores the original constraints for VERS output (not merged)
	RawConstraints []Interval
	// Scheme is the versioning scheme this range was parsed under.
	// It selects the comparison rules used by Contains. Empty means generic.
	Scheme string
}

// NewRange creates a new Range from intervals.
func NewRange(intervals []Interval) *Range {
	return &Range{Intervals: intervals}
}

// Contains checks if the range contains the given version.
func (r *Range) Contains(version string) bool {
	scheme := canonicalScheme(r.Scheme)
	cmp := compareFuncFor(r.Scheme)
	if scheme == schemeBazel && !validVersionForScheme(version, scheme) {
		return false
	}
	if scheme == schemeCargo {
		cmp = compareSemver
	}
	if scheme == schemeNPM || scheme == schemeCargo {
		if _, err := ParseVersion(version); err != nil {
			return false
		}
	}

	// Check exclusions first
	for _, exc := range r.Exclusions {
		excluded := cmp(version, exc) == 0
		if scheme == schemePyPI {
			excluded = pep440SpecifierEqual(version, exc)
		} else if scheme == schemeComposer && (isComposerBranchVersion(version) || isComposerBranchVersion(exc)) {
			excluded = version == exc
		}
		if excluded {
			return false
		}
	}

	// Check if version is in any interval
	for _, interval := range r.Intervals {
		contains := interval.containsCmp(version, cmp)
		switch scheme {
		case schemePyPI:
			contains = pypiIntervalContains(interval, version)
		case schemeComposer:
			contains = composerIntervalContains(interval, version)
		}
		if contains && (scheme == schemeNPM || scheme == schemeCargo) && !semverIntervalAllowsPrerelease(interval, version) {
			contains = false
		}
		if contains {
			return true
		}
	}

	return false
}

func composerIntervalContains(interval Interval, version string) bool {
	candidateIsBranch := isComposerBranchVersion(version)
	minimumIsBranch := isComposerBranchVersion(interval.Min)
	maximumIsBranch := isComposerBranchVersion(interval.Max)
	if !candidateIsBranch && !minimumIsBranch && !maximumIsBranch {
		return interval.containsCmp(version, compareComposer)
	}
	if interval.IsUnbounded() {
		return true
	}
	if candidateIsBranch && minimumIsBranch && maximumIsBranch &&
		interval.MinInclusive && interval.MaxInclusive && interval.Min == interval.Max {
		return version == interval.Min
	}
	return false
}

func semverIntervalAllowsPrerelease(interval Interval, version string) bool {
	candidate, err := ParseVersion(version)
	if err != nil || candidate.Prerelease == "" {
		return err == nil
	}
	for _, bound := range []string{interval.Min, interval.Max} {
		parsed, parseErr := ParseVersion(bound)
		if parseErr == nil && parsed.Prerelease != "" &&
			parsed.Major == candidate.Major && parsed.Minor == candidate.Minor && parsed.Patch == candidate.Patch {
			return true
		}
	}
	return false
}

func pypiIntervalContains(interval Interval, version string) bool {
	if interval.Min != "" && interval.Max != "" && interval.MinInclusive && interval.MaxInclusive &&
		comparePyPI(interval.Min, interval.Max) == 0 {
		return pep440SpecifierEqual(version, interval.Min)
	}
	if !interval.containsCmp(version, comparePyPI) {
		return false
	}
	candidate, candidateOK := parsePEP440(version)
	if !candidateOK {
		return false
	}
	if interval.Min != "" && !interval.MinInclusive {
		bound, boundOK := parsePEP440(interval.Min)
		if boundOK {
			if pep440SpecifierEqual(version, interval.Min) {
				return false
			}
			withoutPost := candidate
			withoutPost.hasPost = false
			withoutPost.post = ""
			withoutPost.hasDev = false
			withoutPost.dev = ""
			withoutPost.local = nil
			if candidate.hasPost && pep440VersionsEqual(withoutPost, bound, true) {
				return false
			}
		}
	}
	if interval.Max != "" && !interval.MaxInclusive {
		bound, boundOK := parsePEP440(interval.Max)
		if boundOK {
			if !bound.hasPre && !bound.hasPost && !bound.hasDev &&
				samePEP440Release(candidate, bound) && (candidate.hasPre || candidate.hasDev) {
				return false
			}
			withoutDev := candidate
			withoutDev.hasDev = false
			withoutDev.dev = ""
			withoutDev.local = nil
			if candidate.hasDev && pep440VersionsEqual(withoutDev, bound, true) {
				return false
			}
		}
	}
	return true
}

func pep440SpecifierEqual(version, specifier string) bool {
	candidate, candidateOK := parsePEP440(version)
	bound, boundOK := parsePEP440(specifier)
	if !candidateOK || !boundOK {
		return comparePyPI(version, specifier) == 0
	}
	return pep440VersionsEqual(candidate, bound, len(bound.local) == 0)
}

func pep440VersionsEqual(left, right pep440Version, ignoreLocal bool) bool {
	if ignoreLocal {
		left.local = nil
		right.local = nil
	}
	return cmpNumStr(left.epoch, right.epoch) == 0 &&
		cmpNumStrSlice(left.release, right.release) == 0 &&
		cmpPEP440Pre(left, right) == 0 &&
		cmpPEP440Post(left, right) == 0 &&
		cmpPEP440Dev(left, right) == 0 &&
		cmpPEP440Local(left.local, right.local) == 0
}

func samePEP440Release(left, right pep440Version) bool {
	return cmpNumStr(left.epoch, right.epoch) == 0 && cmpNumStrSlice(left.release, right.release) == 0
}

// IsEmpty returns true if this range matches no versions.
func (r *Range) IsEmpty() bool {
	if len(r.Intervals) == 0 {
		return true
	}
	cmp := compareFuncFor(r.Scheme)
	for _, interval := range r.Intervals {
		if !interval.isEmptyCmp(cmp) {
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

// ExactVersion reports whether the range matches exactly one version and
// returns the version used for its bounds. Bounds that compare equal under
// the range's scheme count as the same version. An exclusion only prevents a
// result when it excludes that version.
func (r *Range) ExactVersion() (string, bool) {
	if r == nil || len(r.Intervals) != 1 {
		return "", false
	}

	interval := r.Intervals[0]
	if interval.Min == "" || interval.Max == "" ||
		!interval.MinInclusive || !interval.MaxInclusive {
		return "", false
	}

	cmp := compareFuncFor(r.Scheme)
	if cmp(interval.Min, interval.Max) != 0 {
		return "", false
	}
	for _, exclusion := range r.Exclusions {
		if cmp(interval.Min, exclusion) == 0 {
			return "", false
		}
	}

	return interval.Min, true
}

// MinimumVersion returns the lowest version included by the range when that
// version is represented by an inclusive lower bound. It returns false for an
// unbounded range, an exclusive lower bound, or a lower bound excluded from the
// range.
func (r *Range) MinimumVersion() (string, bool) {
	if r == nil {
		return "", false
	}

	cmp := compareFuncFor(r.Scheme)
	minimum := ""
	found := false
	for _, interval := range r.Intervals {
		if interval.isEmptyCmp(cmp) {
			continue
		}
		if interval.Min == "" {
			return "", false
		}
		if interval.Max != "" && cmp(interval.Min, interval.Max) == 0 &&
			interval.MinInclusive && interval.MaxInclusive &&
			!r.Contains(interval.Min) {
			continue
		}
		if !found || cmp(interval.Min, minimum) < 0 {
			minimum = interval.Min
			found = true
		}
	}
	if !found || !r.Contains(minimum) {
		return "", false
	}
	return minimum, true
}

// Union returns a new Range that is the union of this range and another.
// The operands are assumed to use compatible schemes; use UnionChecked to
// have that verified.
func (r *Range) Union(other *Range) *Range {
	left, right := rangesWithCommonScheme(r, other)
	if left.IsEmpty() {
		return right
	}
	if right.IsEmpty() {
		return left
	}

	cmp := compareFuncFor(left.Scheme)

	// Combine all intervals
	allIntervals := make([]Interval, 0, len(left.Intervals)+len(right.Intervals))
	allIntervals = append(allIntervals, left.Intervals...)
	allIntervals = append(allIntervals, right.Intervals...)

	// Merge overlapping intervals for containment checking
	merged := mergeIntervals(allIntervals, cmp)

	// An exclusion survives the union only when the other operand does not
	// independently supply the excluded version.
	var exclusions []string
	for _, e := range left.Exclusions {
		if !right.Contains(e) {
			exclusions = append(exclusions, e)
		}
	}
	for _, e := range right.Exclusions {
		if left.Contains(e) || containsExclusion(exclusions, e, cmp) {
			continue
		}
		exclusions = append(exclusions, e)
	}

	// Combine raw constraints (unmerged) for VERS output
	rawConstraints := make([]Interval, 0, len(left.RawConstraints)+len(right.RawConstraints))
	if len(left.RawConstraints) > 0 {
		rawConstraints = append(rawConstraints, left.RawConstraints...)
	} else {
		rawConstraints = append(rawConstraints, left.Intervals...)
	}
	if len(right.RawConstraints) > 0 {
		rawConstraints = append(rawConstraints, right.RawConstraints...)
	} else {
		rawConstraints = append(rawConstraints, right.Intervals...)
	}

	return &Range{Intervals: merged, Exclusions: exclusions, RawConstraints: rawConstraints, Scheme: left.Scheme}
}

// UnionChecked returns the union of ranges that use compatible schemes.
func (r *Range) UnionChecked(other *Range) (*Range, error) {
	if err := checkRangeSchemes(r, other); err != nil {
		return nil, err
	}
	return r.Union(other), nil
}

// Intersect returns a new Range that is the intersection of this range and
// another. The operands are assumed to use compatible schemes; use
// IntersectChecked to have that verified.
func (r *Range) Intersect(other *Range) *Range {
	left, right := rangesWithCommonScheme(r, other)

	// Combine raw constraints for VERS output (preserved even if result is empty)
	rawConstraints := make([]Interval, 0, len(left.RawConstraints)+len(right.RawConstraints))
	if len(left.RawConstraints) > 0 {
		rawConstraints = append(rawConstraints, left.RawConstraints...)
	} else {
		rawConstraints = append(rawConstraints, left.Intervals...)
	}
	if len(right.RawConstraints) > 0 {
		rawConstraints = append(rawConstraints, right.RawConstraints...)
	} else {
		rawConstraints = append(rawConstraints, right.Intervals...)
	}

	if left.IsEmpty() || right.IsEmpty() {
		return &Range{RawConstraints: rawConstraints, Scheme: left.Scheme}
	}

	cmp := compareFuncFor(left.Scheme)

	// Intersect each pair of intervals
	var result []Interval
	for _, i1 := range left.Intervals {
		for _, i2 := range right.Intervals {
			intersection := i1.intersectCmp(i2, cmp)
			if !intersection.isEmptyCmp(cmp) {
				result = append(result, intersection)
			}
		}
	}

	// Merge overlapping intervals
	merged := mergeIntervals(result, cmp)

	// Combine exclusions (union of exclusions for intersection)
	exclusions := make([]string, 0, len(left.Exclusions)+len(right.Exclusions))
	exclusions = append(exclusions, left.Exclusions...)
	for _, e := range right.Exclusions {
		if !containsExclusion(exclusions, e, cmp) {
			exclusions = append(exclusions, e)
		}
	}

	return &Range{Intervals: merged, Exclusions: exclusions, RawConstraints: rawConstraints, Scheme: left.Scheme}
}

// IntersectChecked returns the intersection of ranges that use compatible schemes.
func (r *Range) IntersectChecked(other *Range) (*Range, error) {
	if err := checkRangeSchemes(r, other); err != nil {
		return nil, err
	}
	return r.Intersect(other), nil
}

func checkRangeSchemes(a, b *Range) error {
	if a == nil || b == nil {
		return fmt.Errorf("cannot combine a nil range")
	}
	sa, sb := canonicalScheme(a.Scheme), canonicalScheme(b.Scheme)
	if sa != "" && sb != "" && sa != sb {
		return fmt.Errorf("cannot combine %q and %q version schemes", a.Scheme, b.Scheme)
	}
	return nil
}

func containsExclusion(exclusions []string, version string, cmp func(a, b string) int) bool {
	for _, existing := range exclusions {
		if cmp(existing, version) == 0 {
			return true
		}
	}
	return false
}

func rangesWithCommonScheme(a, b *Range) (*Range, *Range) {
	scheme := a.Scheme
	if scheme == "" {
		scheme = b.Scheme
	}
	left, right := *a, *b
	left.Scheme, right.Scheme = scheme, scheme
	return &left, &right
}

// Exclude returns a Range that excludes the given version. If the range does
// not contain the version, the receiver is returned unchanged.
func (r *Range) Exclude(version string) *Range {
	if !r.Contains(version) {
		return r
	}

	exclusions := make([]string, len(r.Exclusions), len(r.Exclusions)+1)
	copy(exclusions, r.Exclusions)
	exclusions = append(exclusions, version)

	return &Range{
		Intervals:      r.Intervals,
		Exclusions:     exclusions,
		RawConstraints: r.RawConstraints,
		Scheme:         r.Scheme,
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
	cmp := compareFuncFor(r.Scheme)
	for _, interval := range r.Intervals {
		parts = append(parts, interval.stringCmp(cmp))
	}

	result := strings.Join(parts, " | ")

	if len(r.Exclusions) > 0 {
		result += " excluding " + strings.Join(r.Exclusions, ", ")
	}

	return result
}

// mergeIntervals merges overlapping intervals into a minimal set.
func mergeIntervals(intervals []Interval, cmp func(a, b string) int) []Interval {
	if len(intervals) <= 1 {
		return intervals
	}

	// Filter empty intervals and sort by lower bound
	sorted := make([]Interval, 0, len(intervals))
	for _, iv := range intervals {
		if !iv.isEmptyCmp(cmp) {
			sorted = append(sorted, iv)
		}
	}
	if len(sorted) == 0 {
		return nil
	}

	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Min == "" && b.Min != "" {
			return true // unbounded lower comes first
		}
		if a.Min != "" && b.Min == "" {
			return false
		}
		c := cmp(a.Min, b.Min)
		if c != 0 {
			return c < 0
		}
		return a.MinInclusive && !b.MinInclusive
	})

	result := []Interval{sorted[0]}
	for _, iv := range sorted[1:] {
		last := &result[len(result)-1]
		if union := last.unionCmp(iv, cmp); union != nil {
			*last = *union
		} else {
			result = append(result, iv)
		}
	}

	return result
}
