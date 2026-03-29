package vers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// SemanticVersionRegex matches semantic version strings (with optional v prefix).
var SemanticVersionRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([^+]+))?(?:\+(.+))?$`)

// simpleNumericRegex matches simple numeric versions like "1" or "42".
var simpleNumericRegex = regexp.MustCompile(`^\d+$`)

// versionCache caches parsed versions to avoid re-parsing the same strings.
var versionCache = &boundedCache{
	items: make(map[string]*VersionInfo),
	max:   10000, //nolint:mnd
}

type boundedCache struct {
	mu    sync.RWMutex
	items map[string]*VersionInfo
	max   int
}

func (c *boundedCache) Load(key string) (*VersionInfo, bool) {
	c.mu.RLock()
	v, ok := c.items[key]
	c.mu.RUnlock()
	return v, ok
}

func (c *boundedCache) Store(key string, value *VersionInfo) {
	c.mu.Lock()
	if len(c.items) >= c.max {
		c.items = make(map[string]*VersionInfo)
	}
	c.items[key] = value
	c.mu.Unlock()
}

// VersionInfo represents a parsed version with its components.
type VersionInfo struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
	Original   string
}

// ParseVersion parses a version string into its components.
func ParseVersion(s string) (*VersionInfo, error) {
	if s == "" {
		return nil, fmt.Errorf("empty version string")
	}

	if cached, ok := versionCache.Load(s); ok {
		return cached, nil
	}

	v, err := parseVersionUncached(s)
	if err != nil {
		return nil, err
	}
	versionCache.Store(s, v)
	return v, nil
}

func parseVersionUncached(s string) (*VersionInfo, error) {
	v := &VersionInfo{Original: s}

	if simpleNumericRegex.MatchString(s) {
		v.Major, _ = strconv.Atoi(s)
		return v, nil
	}

	if matches := SemanticVersionRegex.FindStringSubmatch(s); matches != nil {
		return parseSemverMatches(v, matches), nil
	}

	if strings.Contains(s, ".") {
		return parseDotSeparated(v, s), nil
	}

	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2) //nolint:mnd
		v.Major, _ = strconv.Atoi(parts[0])
		if len(parts) > 1 {
			v.Prerelease = parts[1]
		}
		return v, nil
	}

	return nil, fmt.Errorf("invalid version format: %s", s)
}

func parseSemverMatches(v *VersionInfo, matches []string) *VersionInfo {
	if matches[1] != "" {
		v.Major, _ = strconv.Atoi(matches[1])
	}
	if matches[2] != "" {
		v.Minor, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		v.Patch, _ = strconv.Atoi(matches[3])
	}
	v.Prerelease = matches[4]
	v.Build = matches[5]
	return v
}

func parseDotSeparated(v *VersionInfo, s string) *VersionInfo {
	parts := strings.Split(s, ".")
	if len(parts) >= 1 {
		v.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 && !strings.Contains(parts[1], "-") { //nolint:mnd
		v.Minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 { //nolint:mnd
		if strings.Contains(parts[2], "-") {
			patchParts := strings.SplitN(parts[2], "-", 2) //nolint:mnd
			v.Patch, _ = strconv.Atoi(patchParts[0])
			if len(patchParts) > 1 {
				v.Prerelease = patchParts[1]
			}
		} else {
			v.Patch, _ = strconv.Atoi(parts[2])
		}
	}
	if len(parts) > 3 && v.Prerelease == "" { //nolint:mnd
		v.Prerelease = strings.Join(parts[3:], ".")
	}
	return v
}

// String returns the normalized version string.
func (v *VersionInfo) String() string {
	result := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		result += "-" + v.Prerelease
	}
	return result
}

// Compare compares this version to another.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v *VersionInfo) Compare(other *VersionInfo) int {
	// Compare major
	if v.Major < other.Major {
		return -1
	}
	if v.Major > other.Major {
		return 1
	}

	// Compare minor
	if v.Minor < other.Minor {
		return -1
	}
	if v.Minor > other.Minor {
		return 1
	}

	// Compare patch
	if v.Patch < other.Patch {
		return -1
	}
	if v.Patch > other.Patch {
		return 1
	}

	// Handle prerelease comparison
	// No prerelease > has prerelease (1.0.0 > 1.0.0-alpha)
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease == "" && other.Prerelease == "" {
		return 0
	}

	return comparePrerelease(v.Prerelease, other.Prerelease)
}

// IsStable returns true if this is a stable release (no prerelease).
func (v *VersionInfo) IsStable() bool {
	return v.Prerelease == ""
}

// IsPrerelease returns true if this is a prerelease version.
func (v *VersionInfo) IsPrerelease() bool {
	return v.Prerelease != ""
}

// IncrementMajor returns a new version with major incremented.
func (v *VersionInfo) IncrementMajor() *VersionInfo {
	return &VersionInfo{
		Major: v.Major + 1,
		Minor: 0,
		Patch: 0,
	}
}

// IncrementMinor returns a new version with minor incremented.
func (v *VersionInfo) IncrementMinor() *VersionInfo {
	return &VersionInfo{
		Major: v.Major,
		Minor: v.Minor + 1,
		Patch: 0,
	}
}

// IncrementPatch returns a new version with patch incremented.
func (v *VersionInfo) IncrementPatch() *VersionInfo {
	return &VersionInfo{
		Major: v.Major,
		Minor: v.Minor,
		Patch: v.Patch + 1,
	}
}

// CompareVersions compares two version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareVersions(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}

	va, errA := ParseVersion(a)
	vb, errB := ParseVersion(b)

	if errA != nil && errB != nil {
		// Fall back to string comparison
		if a < b {
			return -1
		}
		return 1
	}
	if errA != nil {
		return -1
	}
	if errB != nil {
		return 1
	}

	return va.Compare(vb)
}

// CompareWithScheme compares two version strings using scheme-specific rules.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareWithScheme(a, b, scheme string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}

	switch scheme {
	case "nuget": //nolint:goconst
		return compareNuGet(a, b)
	case "maven":
		return compareMaven(a, b)
	default:
		return CompareVersions(a, b)
	}
}

// compareNuGet compares two NuGet version strings.
// NuGet uses 4-part versions, trailing zeros are equivalent, and prereleases are case-insensitive.
func compareNuGet(a, b string) int {
	partsA := parseNuGetVersion(a)
	partsB := parseNuGetVersion(b)

	// Compare numeric parts (up to 4)
	for i := 0; i < 4; i++ {
		if partsA.numeric[i] < partsB.numeric[i] {
			return -1
		}
		if partsA.numeric[i] > partsB.numeric[i] {
			return 1
		}
	}

	// Handle prerelease comparison
	// No prerelease > has prerelease (1.0.0 > 1.0.0-alpha)
	if partsA.prerelease == "" && partsB.prerelease != "" {
		return 1
	}
	if partsA.prerelease != "" && partsB.prerelease == "" {
		return -1
	}
	if partsA.prerelease == "" && partsB.prerelease == "" {
		return 0
	}

	// NuGet prerelease comparison is case-insensitive
	return compareNuGetPrerelease(partsA.prerelease, partsB.prerelease)
}

type nugetVersion struct {
	numeric    [4]int // major, minor, patch, revision
	prerelease string
}

func parseNuGetVersion(s string) nugetVersion {
	result := nugetVersion{}

	// Split off build metadata (ignored in comparison)
	if idx := strings.Index(s, "+"); idx != -1 {
		s = s[:idx]
	}

	// Split off prerelease
	if idx := strings.Index(s, "-"); idx != -1 {
		result.prerelease = s[idx+1:]
		s = s[:idx]
	}

	// Parse numeric parts
	parts := strings.Split(s, ".")
	for i := 0; i < len(parts) && i < 4; i++ {
		result.numeric[i], _ = strconv.Atoi(parts[i])
	}

	return result
}

func compareNuGetPrerelease(a, b string) int {
	// NuGet prerelease comparison: case-insensitive, dot-separated, numeric parts compared numerically
	partsA := strings.Split(strings.ToLower(a), ".")
	partsB := strings.Split(strings.ToLower(b), ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var partA, partB string
		if i < len(partsA) {
			partA = partsA[i]
		}
		if i < len(partsB) {
			partB = partsB[i]
		}

		if partA == "" {
			return -1
		}
		if partB == "" {
			return 1
		}

		// Try numeric comparison
		numA, errA := strconv.Atoi(partA)
		numB, errB := strconv.Atoi(partB)

		if errA == nil && errB == nil {
			if numA < numB {
				return -1
			}
			if numA > numB {
				return 1
			}
		} else {
			// String comparison (already lowercased)
			if partA < partB {
				return -1
			}
			if partA > partB {
				return 1
			}
		}
	}

	return 0
}

// compareMaven compares two Maven version strings.
// Maven has special qualifier ordering: alpha < beta < milestone < rc < snapshot < "" (release) < sp
// Key rules:
// - A sublist (afterDash) item is LESS than a direct numeric item (list < int)
// - A sublist item is GREATER than a direct string item (list > string)
// - Trailing zeros are removed from the base version (before any sublist starts)
func compareMaven(a, b string) int {
	partsA := parseMavenVersion(a)
	partsB := parseMavenVersion(b)

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var compA, compB mavenComponent
		if i < len(partsA) {
			compA = partsA[i]
		} else {
			compA = mavenComponent{isNull: true}
		}
		if i < len(partsB) {
			compB = partsB[i]
		} else {
			compB = mavenComponent{isNull: true}
		}

		cmp := compareMavenComponentsNew(compA, compB)
		if cmp != 0 {
			return cmp
		}
	}

	return 0
}

// compareMavenComponentsNew compares two Maven components with proper handling of sublist vs direct items.
func compareMavenComponentsNew(a, b mavenComponent) int {
	if a.isNull && b.isNull {
		return 0
	}
	if a.isNull {
		return -compareMavenToNull(b)
	}
	if b.isNull {
		return compareMavenToNull(a)
	}

	if a.afterDash != b.afterDash {
		return compareMavenDifferentLevels(a, b)
	}

	return compareMavenSameLevel(a, b)
}

func compareMavenDifferentLevels(a, b mavenComponent) int {
	if a.afterDash {
		if b.isNumeric {
			return -1 // sublist < direct numeric
		}
		return 1 // sublist > direct string
	}
	if a.isNumeric {
		return 1 // direct numeric > sublist
	}
	return -1 // direct string < sublist
}

func compareMavenSameLevel(a, b mavenComponent) int {
	if a.isNumeric && b.isNumeric {
		return cmpInt(a.numeric, b.numeric)
	}
	if a.isNumeric {
		return 1 // numeric > any qualifier
	}
	if b.isNumeric {
		return -1 // qualifier < numeric
	}

	orderA, okA := getMavenQualifierOrder(a.qualifier)
	orderB, okB := getMavenQualifierOrder(b.qualifier)
	if orderA != orderB {
		return cmpInt(orderA, orderB)
	}
	if !okA && !okB {
		return cmpString(a.qualifier, b.qualifier)
	}
	return 0
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func cmpString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// compareMavenToNull compares a component to null (missing component).
func compareMavenToNull(comp mavenComponent) int {
	if comp.isNumeric {
		return compareMavenNumericToNull(comp)
	}
	return compareMavenQualifierToNull(comp.qualifier)
}

func compareMavenNumericToNull(comp mavenComponent) int {
	if comp.numeric == 0 {
		return 0
	}
	if comp.afterDash && comp.numeric < 0 {
		return -1
	}
	return 1
}

func compareMavenQualifierToNull(qualifier string) int {
	orderComp, _ := getMavenQualifierOrder(qualifier)
	orderNull := mavenQualifierOrder[""]
	return cmpInt(orderComp, orderNull)
}

type mavenComponent struct {
	isNumeric bool
	numeric   int
	qualifier string
	isNull    bool
	afterDash bool // true if this component came after a dash (or digit-letter transition)
}

// Maven qualifier ordering
// Order: alpha < beta < milestone < rc < snapshot < "" (release) < sp < unknown < numbers
//
//nolint:mnd
var mavenQualifierOrder = map[string]int{
	"alpha":     1,
	"beta":      2,
	"milestone": 3,
	"rc":        4,
	"snapshot":  5,
	"":          6, // release
	"sp":        7, // sp comes after release but before unknown qualifiers
}

func getMavenQualifierOrder(q string) (int, bool) {
	order, ok := mavenQualifierOrder[strings.ToLower(q)]
	if ok {
		return order, true
	}
	return 8, false //nolint:mnd
}

func parseMavenVersion(s string) []mavenComponent {
	var result []mavenComponent

	// Maven versions are split by . and - AND on transitions between digits and letters
	s = strings.ToLower(s)

	// Split on delimiters and digit/letter transitions, tracking separators
	parts, afterDashFlags := splitMavenVersionWithSeparators(s)

	for i, part := range parts {
		if part == "" {
			continue
		}
		// Check if next part is a digit (for single-letter qualifier normalization)
		nextIsDigit := false
		if i+1 < len(parts) {
			if _, err := strconv.Atoi(parts[i+1]); err == nil {
				nextIsDigit = true
			}
		}
		// Normalize qualifier aliases
		normalized := normalizeMavenQualifierWithNext(part, nextIsDigit)
		if normalized == "" {
			// Skip empty qualifiers (ga, final, release are equivalent to nothing)
			continue
		}
		afterDash := false
		if i < len(afterDashFlags) {
			afterDash = afterDashFlags[i]
		}
		if num, err := strconv.Atoi(normalized); err == nil {
			result = append(result, mavenComponent{isNumeric: true, numeric: num, afterDash: afterDash})
		} else {
			result = append(result, mavenComponent{qualifier: normalized, afterDash: afterDash})
		}
	}

	// Maven normalization: remove trailing null-equivalent components
	// A null component is: 0 for numeric, "" for string (already handled above)
	// Also remove trailing zeros BEFORE the first qualifier (if any)
	result = normalizeMavenComponents(result)

	return result
}

// normalizeMavenComponents removes trailing null-equivalent components.
// In Maven, trailing zeros are removed from each "level":
// - From the base version (before any sublist)
// - From the end of the version
// This makes "1.0-a" normalize to [1, a] (the 0 is trailing in the base)
func normalizeMavenComponents(components []mavenComponent) []mavenComponent {
	if len(components) == 0 {
		return components
	}

	// Find the first sublist component (afterDash=true)
	firstSublistIdx := -1
	for i, c := range components {
		if c.afterDash {
			firstSublistIdx = i
			break
		}
	}

	// Remove trailing zeros from the base portion (before first sublist)
	if firstSublistIdx > 0 {
		// Trim trailing zeros from base (indices 0 to firstSublistIdx-1)
		baseEnd := firstSublistIdx
		for baseEnd > 1 && components[baseEnd-1].isNumeric && components[baseEnd-1].numeric == 0 {
			baseEnd--
		}
		if baseEnd < firstSublistIdx {
			// Rebuild: base without trailing zeros + sublist portion
			newComponents := make([]mavenComponent, baseEnd)
			copy(newComponents, components[:baseEnd])
			newComponents = append(newComponents, components[firstSublistIdx:]...)
			components = newComponents
		}
	} else if firstSublistIdx == -1 {
		// No sublist - just remove trailing zeros from the end
		for len(components) > 0 {
			last := components[len(components)-1]
			if last.isNumeric && last.numeric == 0 {
				components = components[:len(components)-1]
			} else {
				break
			}
		}
	}
	// If firstSublistIdx == 0, the first component is a sublist, nothing to trim from base

	return components
}

// normalizeMavenQualifier normalizes Maven qualifier aliases
// In Maven, single-letter qualifiers (a, b, m) only become their full forms when followed by a digit
// This is handled by normalizeMavenQualifierWithNext
func normalizeMavenQualifier(q string) string {
	switch q {
	case "cr":
		return "rc"
	case "ga", "final", "release":
		return "" // These are equivalent to release (no qualifier)
	default:
		return q
	}
}

// normalizeMavenQualifierWithNext normalizes a qualifier considering the next part
func normalizeMavenQualifierWithNext(q string, nextIsDigit bool) string {
	// Single-letter qualifiers become their full form only when followed by a digit
	if nextIsDigit && len(q) == 1 {
		switch q {
		case "a":
			return "alpha"
		case "b":
			return "beta"
		case "m":
			return "milestone"
		}
	}
	return normalizeMavenQualifier(q)
}

// splitMavenVersionWithSeparators splits a Maven version string and tracks which parts came after a dash.
// Returns the parts and a parallel slice of booleans indicating if each part is "after dash" (sublist).
// Both '-' separator AND digit-letter transitions create sublist context.
func splitMavenVersionWithSeparators(s string) ([]string, []bool) {
	var parts []string
	var afterDash []bool
	var current strings.Builder
	var lastWasDigit bool
	firstChar := true
	currentAfterDash := false // first component is never after dash

	for _, c := range s {
		if c == '.' || c == '-' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				afterDash = append(afterDash, currentAfterDash)
				current.Reset()
			}
			// '-' creates afterDash context (sublist in Maven's model)
			// '.' does NOT create afterDash context
			currentAfterDash = (c == '-')
			firstChar = true
			continue
		}

		isDigit := c >= '0' && c <= '9'

		// Split on digit/letter transitions (but not at the start)
		// In Maven, digit-letter transitions also create sublist context
		if !firstChar && isDigit != lastWasDigit && current.Len() > 0 {
			parts = append(parts, current.String())
			afterDash = append(afterDash, currentAfterDash)
			current.Reset()
			// Digit-letter transition creates sublist for the new component
			currentAfterDash = true
		}

		current.WriteRune(c)
		lastWasDigit = isDigit
		firstChar = false
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
		afterDash = append(afterDash, currentAfterDash)
	}

	return parts, afterDash
}

func comparePrerelease(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var partA, partB string
		if i < len(partsA) {
			partA = partsA[i]
		}
		if i < len(partsB) {
			partB = partsB[i]
		}

		if partA == "" {
			return -1
		}
		if partB == "" {
			return 1
		}

		// Try numeric comparison
		numA, errA := strconv.Atoi(partA)
		numB, errB := strconv.Atoi(partB)

		if errA == nil && errB == nil {
			if numA < numB {
				return -1
			}
			if numA > numB {
				return 1
			}
		} else {
			// String comparison
			if partA < partB {
				return -1
			}
			if partA > partB {
				return 1
			}
		}
	}

	return 0
}
