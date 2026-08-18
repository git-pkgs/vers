package vers

import "strings"

// TagCandidates returns possible repository tag spellings for a version. It
// trims surrounding whitespace, preserves the remaining spelling, adds its
// scheme-normalized form, and adds or removes a leading v for numeric versions.
// Duplicate candidates are omitted.
func TagCandidates(version, scheme string) ([]string, error) {
	version = strings.TrimSpace(version)
	normalized, err := normalizeTagVersion(version, scheme)
	if err != nil {
		return nil, err
	}

	var candidates []string
	seen := make(map[string]bool)
	add := func(candidate string) {
		if candidate == "" || seen[candidate] {
			return
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	addSpellings := func(candidate string) {
		add(candidate)
		switch {
		case hasNumericVPrefix(candidate):
			add(candidate[1:])
		case startsWithDigit(candidate):
			add("v" + candidate)
		}
	}

	addSpellings(version)
	addSpellings(normalized)
	return candidates, nil
}

func normalizeTagVersion(version, scheme string) (string, error) {
	normalized, err := NormalizeWithScheme(version, scheme)
	if err == nil || !hasNumericVPrefix(version) {
		return normalized, err
	}

	base := version[1:]
	if version[0] == 'V' {
		if normalized, lowerErr := NormalizeWithScheme("v"+base, scheme); lowerErr == nil {
			return normalized, nil
		}
	}
	return NormalizeWithScheme(base, scheme)
}

func hasNumericVPrefix(version string) bool {
	return len(version) > 1 && (version[0] == 'v' || version[0] == 'V') && isASCIIDigit(version[1])
}

func startsWithDigit(version string) bool {
	return version != "" && isASCIIDigit(version[0])
}
