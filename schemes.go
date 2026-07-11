package vers

import (
	"regexp"
	"strings"
)

type semverValue struct {
	core []string
	pre  []string
}

func compareSemver(a, b string) int {
	va, okA := parseSemverValue(a)
	vb, okB := parseSemverValue(b)
	if !okA || !okB {
		return CompareVersions(a, b)
	}
	for i := range va.core {
		if c := cmpNumStr(va.core[i], vb.core[i]); c != 0 {
			return c
		}
	}
	return compareSemverPrerelease(va.pre, vb.pre)
}

func parseSemverValue(s string) (semverValue, bool) {
	m := SemanticVersionRegex.FindStringSubmatch(s)
	if m == nil {
		return semverValue{}, false
	}
	v := semverValue{core: []string{m[1], m[2], m[3]}}
	if m[4] != "" {
		v.pre = strings.Split(m[4], ".")
	}
	return v, true
}

func compareSemverPrerelease(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		aNum, bNum := isDigits(a[i]), isDigits(b[i])
		if aNum != bNum {
			if aNum {
				return -1
			}
			return 1
		}
		var c int
		if aNum {
			c = cmpNumStr(a[i], b[i])
		} else {
			c = cmpString(a[i], b[i])
		}
		if c != 0 {
			return c
		}
	}
	return cmpInt(len(a), len(b))
}

type gemSegment struct {
	value string
	num   bool
}

var gemSegmentRegex = regexp.MustCompile(`[0-9]+|[A-Za-z]+`)

func compareGem(a, b string) int {
	return compareGemSegments(parseGemSegments(a), parseGemSegments(b))
}

func parseGemRawSegments(s string) []gemSegment {
	s = strings.ReplaceAll(strings.TrimSpace(s), "-", ".pre.")
	raw := gemSegmentRegex.FindAllString(s, -1)
	parts := make([]gemSegment, 0, len(raw))
	for _, part := range raw {
		parts = append(parts, gemSegment{value: part, num: isDigits(part)})
	}
	return parts
}

func parseGemSegments(s string) []gemSegment {
	parts := parseGemRawSegments(s)

	firstAlpha := -1
	for i, part := range parts {
		if !part.num {
			firstAlpha = i
			break
		}
	}
	if firstAlpha > 0 {
		start := firstAlpha
		for start > 0 && parts[start-1].num && cmpNumStr(parts[start-1].value, "0") == 0 {
			start--
		}
		if start < firstAlpha {
			parts = append(parts[:start], parts[firstAlpha:]...)
		}
	}
	for len(parts) > 0 && parts[len(parts)-1].num && cmpNumStr(parts[len(parts)-1].value, "0") == 0 {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func compareGemSegments(a, b []gemSegment) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i].num != b[i].num {
			if a[i].num {
				return 1
			}
			return -1
		}
		var c int
		if a[i].num {
			c = cmpNumStr(a[i].value, b[i].value)
		} else {
			c = cmpString(a[i].value, b[i].value)
		}
		if c != 0 {
			return c
		}
	}
	if len(a) == len(b) {
		return 0
	}
	if len(a) < len(b) {
		return compareMissingGemSegments(b[len(a):])
	}
	return -compareMissingGemSegments(a[len(b):])
}

func compareMissingGemSegments(remaining []gemSegment) int {
	for _, part := range remaining {
		if !part.num {
			return 1
		}
		if cmpNumStr(part.value, "0") != 0 {
			return -1
		}
	}
	return 0
}

func compareDebian(a, b string) int {
	ea, ua, ra := splitDebianVersion(a)
	eb, ub, rb := splitDebianVersion(b)
	if c := cmpNumStr(ea, eb); c != 0 {
		return c
	}
	if c := compareDebianPart(ua, ub); c != 0 {
		return c
	}
	return compareDebianPart(ra, rb)
}

func splitDebianVersion(s string) (epoch, upstream, revision string) {
	upstream = s
	if i := strings.IndexByte(upstream, ':'); i >= 0 {
		epoch, upstream = upstream[:i], upstream[i+1:]
	}
	if i := strings.LastIndexByte(upstream, '-'); i >= 0 {
		upstream, revision = upstream[:i], upstream[i+1:]
	} else {
		revision = "0"
	}
	return epoch, upstream, revision
}

func compareDebianPart(a, b string) int { //nolint:gocognit
	for ia, ib := 0, 0; ia < len(a) || ib < len(b); {
		for (ia < len(a) && !isASCIIDigit(a[ia])) || (ib < len(b) && !isASCIIDigit(b[ib])) {
			var ca, cb byte
			if ia < len(a) && !isASCIIDigit(a[ia]) {
				ca = a[ia]
			}
			if ib < len(b) && !isASCIIDigit(b[ib]) {
				cb = b[ib]
			}
			if oa, ob := debianCharOrder(ca), debianCharOrder(cb); oa != ob {
				return cmpInt(oa, ob)
			}
			if ca != 0 {
				ia++
			}
			if cb != 0 {
				ib++
			}
		}

		za, zb := ia, ib
		for za < len(a) && a[za] == '0' {
			za++
		}
		for zb < len(b) && b[zb] == '0' {
			zb++
		}
		ea, eb := za, zb
		for ea < len(a) && isASCIIDigit(a[ea]) {
			ea++
		}
		for eb < len(b) && isASCIIDigit(b[eb]) {
			eb++
		}
		if ea-za != eb-zb {
			return cmpInt(ea-za, eb-zb)
		}
		if c := cmpString(a[za:ea], b[zb:eb]); c != 0 {
			return c
		}
		ia, ib = ea, eb
	}
	return 0
}

func debianCharOrder(c byte) int {
	switch {
	case c == '~':
		return -1
	case c == 0:
		return 0
	case isASCIIAlpha(c):
		return int(c)
	default:
		return int(c) + 256 //nolint:mnd // Debian orders non-letters above the ASCII letter range.
	}
}

func compareRPM(a, b string) int {
	ea, va, ra := splitRPMVersion(a)
	eb, vb, rb := splitRPMVersion(b)
	if c := cmpNumStr(ea, eb); c != 0 {
		return c
	}
	if c := compareRPMPart(va, vb); c != 0 {
		return c
	}
	return compareRPMPart(ra, rb)
}

func splitRPMVersion(s string) (epoch, version, release string) {
	version = s
	if i := strings.IndexByte(version, ':'); i >= 0 {
		epoch, version = version[:i], version[i+1:]
	}
	if i := strings.LastIndexByte(version, '-'); i >= 0 {
		version, release = version[:i], version[i+1:]
	}
	return epoch, version, release
}

func compareRPMPart(a, b string) int { //nolint:gocyclo,gocognit
	ia, ib := 0, 0
	for ia < len(a) || ib < len(b) {
		for ia < len(a) && !isASCIIAlnum(a[ia]) && a[ia] != '~' && a[ia] != '^' {
			ia++
		}
		for ib < len(b) && !isASCIIAlnum(b[ib]) && b[ib] != '~' && b[ib] != '^' {
			ib++
		}

		if (ia < len(a) && a[ia] == '~') || (ib < len(b) && b[ib] == '~') {
			if ia >= len(a) || a[ia] != '~' {
				return 1
			}
			if ib >= len(b) || b[ib] != '~' {
				return -1
			}
			ia++
			ib++
			continue
		}
		if (ia < len(a) && a[ia] == '^') || (ib < len(b) && b[ib] == '^') {
			if ia >= len(a) {
				return -1
			}
			if ib >= len(b) {
				return 1
			}
			if a[ia] != '^' {
				return 1
			}
			if b[ib] != '^' {
				return -1
			}
			ia++
			ib++
			continue
		}

		if ia >= len(a) || ib >= len(b) {
			return cmpInt(len(a)-ia, len(b)-ib)
		}

		aNum, bNum := isASCIIDigit(a[ia]), isASCIIDigit(b[ib])
		if aNum != bNum {
			if aNum {
				return 1
			}
			return -1
		}
		ea, eb := ia, ib
		if aNum {
			for ea < len(a) && isASCIIDigit(a[ea]) {
				ea++
			}
			for eb < len(b) && isASCIIDigit(b[eb]) {
				eb++
			}
			if c := cmpNumStr(a[ia:ea], b[ib:eb]); c != 0 {
				return c
			}
		} else {
			for ea < len(a) && isASCIIAlpha(a[ea]) {
				ea++
			}
			for eb < len(b) && isASCIIAlpha(b[eb]) {
				eb++
			}
			if c := cmpString(a[ia:ea], b[ib:eb]); c != 0 {
				return c
			}
		}
		ia, ib = ea, eb
	}
	return 0
}

func compareIntDot(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var va, vb string
		if i < len(pa) {
			va = pa[i]
		}
		if i < len(pb) {
			vb = pb[i]
		}
		if c := cmpNumStr(va, vb); c != 0 {
			return c
		}
	}
	return 0
}

func compareOpenSSL(a, b string) int {
	va, okA := parseOpenSSLVersion(a)
	vb, okB := parseOpenSSLVersion(b)
	if !okA || !okB {
		return CompareVersions(a, b)
	}
	if cmpNumStr(va.core[0], "3") >= 0 && cmpNumStr(vb.core[0], "3") >= 0 {
		return compareSemver(a, b)
	}
	for i := range va.core {
		if c := cmpNumStr(va.core[i], vb.core[i]); c != 0 {
			return c
		}
	}
	preA := strings.HasPrefix(va.patch, "-alpha") || strings.HasPrefix(va.patch, "-beta")
	preB := strings.HasPrefix(vb.patch, "-alpha") || strings.HasPrefix(vb.patch, "-beta")
	if preA != preB {
		if preA {
			return -1
		}
		return 1
	}
	return cmpString(va.patch, vb.patch)
}

type opensslVersion struct {
	core  [3]string
	patch string
}

func parseOpenSSLVersion(s string) (opensslVersion, bool) {
	var v opensslVersion
	parts := strings.SplitN(s, ".", 3) //nolint:mnd
	if len(parts) != 3 || !isDigits(parts[0]) || !isDigits(parts[1]) {
		return v, false
	}
	i := 0
	for i < len(parts[2]) && isASCIIDigit(parts[2][i]) {
		i++
	}
	if i == 0 {
		return v, false
	}
	v.core = [3]string{parts[0], parts[1], parts[2][:i]}
	v.patch = parts[2][i:]
	return v, true
}

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }
func isASCIIAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isASCIIAlnum(c byte) bool { return isASCIIDigit(c) || isASCIIAlpha(c) }
