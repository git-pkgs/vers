package vers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// SemanticVersionRegex matches semantic version strings.
var SemanticVersionRegex = regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([^+]+))?(?:\+(.+))?$`)

// simpleNumericRegex matches simple numeric versions like "1" or "42".
var simpleNumericRegex = regexp.MustCompile(`^\d+$`)

// versionCache caches parsed versions to avoid re-parsing the same strings.
var versionCache = &boundedCache{
	items: make(map[string]*VersionInfo),
	max:   10000,
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

	// Check cache first
	if cached, ok := versionCache.Load(s); ok {
		return cached, nil
	}

	v := &VersionInfo{Original: s}

	// Handle simple numeric versions
	if simpleNumericRegex.MatchString(s) {
		major, _ := strconv.Atoi(s)
		v.Major = major
		versionCache.Store(s, v)
		return v, nil
	}

	// Try semantic version parsing
	if matches := SemanticVersionRegex.FindStringSubmatch(s); matches != nil {
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
		versionCache.Store(s, v)
		return v, nil
	}

	// Handle dot-separated versions
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) >= 1 {
			v.Major, _ = strconv.Atoi(parts[0])
		}
		if len(parts) >= 2 && !strings.Contains(parts[1], "-") {
			v.Minor, _ = strconv.Atoi(parts[1])
		}
		if len(parts) >= 3 {
			if strings.Contains(parts[2], "-") {
				patchParts := strings.SplitN(parts[2], "-", 2)
				v.Patch, _ = strconv.Atoi(patchParts[0])
				if len(patchParts) > 1 {
					v.Prerelease = patchParts[1]
				}
			} else {
				v.Patch, _ = strconv.Atoi(parts[2])
			}
		}
		if len(parts) > 3 && v.Prerelease == "" {
			v.Prerelease = strings.Join(parts[3:], ".")
		}
		versionCache.Store(s, v)
		return v, nil
	}

	// Handle dash-separated versions
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		v.Major, _ = strconv.Atoi(parts[0])
		if len(parts) > 1 {
			v.Prerelease = parts[1]
		}
		versionCache.Store(s, v)
		return v, nil
	}

	return nil, fmt.Errorf("invalid version format: %s", s)
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
