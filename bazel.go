package vers

import (
	"regexp"
	"strconv"
	"strings"
)

var bazelVersionRegex = regexp.MustCompile(`^([A-Za-z0-9.]+)(-([A-Za-z0-9.-]+))?(\+([A-Za-z0-9.-]+))?$`)

type bazelVersion struct {
	release    []bazelIdentifier
	prerelease []bazelIdentifier
	normalized string
	empty      bool
}

type bazelIdentifier struct {
	text    string
	number  uint64
	numeric bool
}

func parseBazelVersion(version string) (bazelVersion, bool) {
	if version == "" {
		return bazelVersion{empty: true}, true
	}

	match := bazelVersionRegex.FindStringSubmatch(version)
	if match == nil {
		return bazelVersion{}, false
	}

	release, ok := parseBazelIdentifiers(match[1], true)
	if !ok {
		return bazelVersion{}, false
	}
	prerelease, ok := parseBazelIdentifiers(match[3], true)
	if !ok {
		return bazelVersion{}, false
	}
	if _, ok := parseBazelIdentifiers(match[5], false); !ok {
		return bazelVersion{}, false
	}

	normalized := match[1]
	if match[3] != "" {
		normalized += "-" + match[3]
	}
	return bazelVersion{release: release, prerelease: prerelease, normalized: normalized}, true
}

func parseBazelIdentifiers(value string, numericLimit bool) ([]bazelIdentifier, bool) {
	if value == "" {
		return nil, true
	}

	parts := strings.Split(value, ".")
	identifiers := make([]bazelIdentifier, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		identifier := bazelIdentifier{text: part, numeric: isDigits(part)}
		if identifier.numeric && numericLimit {
			number, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				return nil, false
			}
			identifier.number = number
		}
		identifiers = append(identifiers, identifier)
	}
	return identifiers, true
}

func compareBazel(a, b string) int {
	left, leftOK := parseBazelVersion(a)
	right, rightOK := parseBazelVersion(b)
	if !leftOK || !rightOK {
		return cmpString(a, b)
	}
	if left.empty {
		if right.empty {
			return 0
		}
		return 1
	}
	if right.empty {
		return -1
	}
	if result := compareBazelIdentifiers(left.release, right.release); result != 0 {
		return result
	}
	if len(left.prerelease) == 0 && len(right.prerelease) != 0 {
		return 1
	}
	if len(left.prerelease) != 0 && len(right.prerelease) == 0 {
		return -1
	}
	return compareBazelIdentifiers(left.prerelease, right.prerelease)
}

func compareBazelIdentifiers(a, b []bazelIdentifier) int {
	limit := min(len(a), len(b))
	for i := 0; i < limit; i++ {
		if result := compareBazelIdentifier(a[i], b[i]); result != 0 {
			return result
		}
	}
	return cmpInt(len(a), len(b))
}

func compareBazelIdentifier(a, b bazelIdentifier) int {
	if a.numeric != b.numeric {
		if a.numeric {
			return -1
		}
		return 1
	}
	if a.numeric {
		if a.number < b.number {
			return -1
		}
		if a.number > b.number {
			return 1
		}
	}
	return cmpString(a.text, b.text)
}
