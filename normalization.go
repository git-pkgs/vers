package vers

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	gemVersionRegex    = regexp.MustCompile(`^[0-9]+(?:\.[0-9A-Za-z]+)*(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?$`)
	nugetVersionRegex  = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){0,3}(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	intDotVersionRegex = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)
)

func validVersionForScheme(version, scheme string) bool { //nolint:gocyclo
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}

	switch scheme {
	case schemePyPI:
		_, ok := parsePEP440(version)
		return ok
	case schemeSemVer, schemeNPM, schemeCargo, schemeGo, schemeGolang, schemeHex, schemeElixir:
		return validSemverLike(version)
	case schemeGem, schemeRubyGems:
		return gemVersionRegex.MatchString(version)
	case schemeDeb, schemeDebian:
		return validDebianVersion(version)
	case schemeRPM:
		return validRPMVersion(version)
	case schemeNuGet:
		return nugetVersionRegex.MatchString(version)
	case schemeIntDot:
		return intDotVersionRegex.MatchString(version)
	case schemeOpenSSL:
		_, ok := parseOpenSSLVersion(version)
		return ok
	case schemeMaven, schemeLexicographic, schemeDatetime, schemeAPK, schemeAlpine, schemeGentoo, schemeALPM, schemeConan:
		return !strings.ContainsAny(version, " \t\r\n")
	default:
		return Valid(version)
	}
}

func normalizeVersionForScheme(version, scheme string) (string, error) {
	version = strings.TrimSpace(version)
	if scheme == "" {
		return Normalize(version)
	}
	if !validVersionForScheme(version, scheme) {
		return "", fmt.Errorf("invalid %s version: %s", scheme, version)
	}

	switch scheme {
	case schemePyPI:
		v, _ := parsePEP440(version)
		return formatPEP440(v), nil
	case schemeSemVer, schemeNPM, schemeCargo, schemeGo, schemeGolang, schemeHex, schemeElixir:
		return normalizeSemverLike(version, scheme == schemeGo || scheme == schemeGolang), nil
	case schemeGem, schemeRubyGems, schemeDeb, schemeDebian, schemeRPM, schemeNuGet, schemeIntDot, schemeOpenSSL,
		schemeMaven, schemeLexicographic, schemeDatetime, schemeAPK, schemeAlpine, schemeGentoo, schemeALPM, schemeConan:
		return version, nil
	default:
		return Normalize(version)
	}
}

func validSemverLike(s string) bool {
	m := SemanticVersionRegex.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	for _, field := range []string{m[4], m[5]} {
		if field == "" {
			continue
		}
		for _, part := range strings.Split(field, ".") {
			if part == "" || strings.Trim(part, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-") != "" {
				return false
			}
		}
	}
	return true
}

func normalizeSemverLike(s string, preserveV bool) string {
	m := SemanticVersionRegex.FindStringSubmatch(s)
	core := []string{trimLeadingZeros(m[1]), trimLeadingZeros(m[2]), trimLeadingZeros(m[3])}
	result := strings.Join(core, ".")
	if preserveV && (strings.HasPrefix(s, "v") || strings.HasPrefix(s, "V")) {
		result = "v" + result
	}
	if m[4] != "" {
		parts := strings.Split(m[4], ".")
		for i, part := range parts {
			if isDigits(part) {
				parts[i] = trimLeadingZeros(part)
			}
		}
		result += "-" + strings.Join(parts, ".")
	}
	if m[5] != "" {
		result += "+" + m[5]
	}
	return result
}

func formatPEP440(v pep440Version) string {
	var result strings.Builder
	if cmpNumStr(v.epoch, "0") != 0 {
		result.WriteString(trimLeadingZeros(v.epoch))
		result.WriteByte('!')
	}
	for i, part := range v.release {
		if i > 0 {
			result.WriteByte('.')
		}
		result.WriteString(trimLeadingZeros(part))
	}
	if v.hasPre {
		result.WriteString([]string{"a", "b", "rc"}[v.preTag])
		result.WriteString(trimLeadingZeros(v.preNum))
	}
	if v.hasPost {
		result.WriteString(".post")
		result.WriteString(trimLeadingZeros(v.post))
	}
	if v.hasDev {
		result.WriteString(".dev")
		result.WriteString(trimLeadingZeros(v.dev))
	}
	if len(v.local) > 0 {
		result.WriteByte('+')
		for i, part := range v.local {
			if i > 0 {
				result.WriteByte('.')
			}
			if part.isNum {
				result.WriteString(trimLeadingZeros(part.s))
			} else {
				result.WriteString(part.s)
			}
		}
	}
	return result.String()
}

func validDebianVersion(s string) bool {
	epoch, upstream, revision := splitDebianVersion(s)
	if epoch != "" && !isDigits(epoch) {
		return false
	}
	if upstream == "" || !isASCIIDigit(upstream[0]) || !containsOnly(upstream, isDebianUpstreamChar) {
		return false
	}
	return revision == "0" || (revision != "" && containsOnly(revision, isDebianRevisionChar))
}

func validRPMVersion(s string) bool {
	if strings.HasSuffix(s, "-") {
		return false
	}
	epoch, version, release := splitRPMVersion(s)
	if epoch != "" && !isDigits(epoch) {
		return false
	}
	if version == "" || !containsOnly(version, isRPMVersionChar) {
		return false
	}
	return release == "" || containsOnly(release, isRPMVersionChar)
}

func containsOnly(s string, valid func(byte) bool) bool {
	for i := 0; i < len(s); i++ {
		if !valid(s[i]) {
			return false
		}
	}
	return true
}

func isDebianUpstreamChar(c byte) bool {
	return isASCIIAlnum(c) || strings.ContainsRune(".+-~", rune(c))
}

func isDebianRevisionChar(c byte) bool {
	return isASCIIAlnum(c) || strings.ContainsRune(".+~", rune(c))
}

func isRPMVersionChar(c byte) bool {
	return isASCIIAlnum(c) || strings.ContainsRune("._+~^", rune(c))
}
