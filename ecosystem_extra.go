package vers

import "strings"

const (
	segmentDigit = iota
	segmentAlpha
	segmentOther
)

type typedSegment struct {
	value string
	kind  int
}

func compareALPM(a, b string) int {
	ea, va, ra, hasRA := splitALPMVersion(a)
	eb, vb, rb, hasRB := splitALPMVersion(b)
	if c := compareALPMPart(ea, eb); c != 0 {
		return c
	}
	if c := compareALPMPart(va, vb); c != 0 {
		return c
	}
	if hasRA && hasRB {
		return compareALPMPart(ra, rb)
	}
	return 0
}

func splitALPMVersion(s string) (epoch, version, release string, hasRelease bool) {
	epoch, version = "0", s
	if i := strings.IndexByte(version, ':'); i >= 0 {
		epoch, version = version[:i], version[i+1:]
	}
	if i := strings.LastIndexByte(version, '-'); i >= 0 {
		version, release, hasRelease = version[:i], version[i+1:], true
	}
	return epoch, version, release, hasRelease
}

func splitTypedSegments(s string) []typedSegment {
	if s == "" {
		return nil
	}
	segments := make([]typedSegment, 0)
	start, kind := 0, segmentKind(s[0])
	for i := 1; i < len(s); i++ {
		if next := segmentKind(s[i]); next != kind {
			segments = append(segments, typedSegment{value: s[start:i], kind: kind})
			start, kind = i, next
		}
	}
	return append(segments, typedSegment{value: s[start:], kind: kind})
}

func segmentKind(c byte) int {
	if isASCIIDigit(c) {
		return segmentDigit
	}
	if isASCIIAlpha(c) {
		return segmentAlpha
	}
	return segmentOther
}

func compareALPMPart(a, b string) int {
	pa, pb := splitTypedSegments(a), splitTypedSegments(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		if i >= len(pa) {
			if pb[i].kind == segmentAlpha {
				return 1
			}
			return -1
		}
		if i >= len(pb) {
			if pa[i].kind == segmentAlpha {
				return -1
			}
			return 1
		}
		a, b := pa[i], pb[i]
		if a.kind != b.kind {
			if a.kind == segmentDigit {
				return 1
			}
			if b.kind == segmentDigit {
				return -1
			}
			if a.kind == segmentOther {
				return 1
			}
			return -1
		}
		var c int
		switch a.kind {
		case segmentDigit:
			c = cmpNumStr(a.value, b.value)
		case segmentAlpha:
			c = cmpString(a.value, b.value)
		default:
			c = cmpInt(len(a.value), len(b.value))
		}
		if c != 0 {
			return c
		}
	}
	return 0
}

type conanVersion struct {
	main  []conanItem
	pre   *conanVersion
	build *conanVersion
}

type conanItem struct {
	value string
	num   bool
}

func compareConan(a, b string) int {
	return compareConanVersion(parseConanVersion(a), parseConanVersion(b))
}

func parseConanVersion(s string) conanVersion {
	v := conanVersion{}
	if i := strings.LastIndexByte(s, '+'); i >= 0 {
		build := parseConanVersion(s[i+1:])
		v.build, s = &build, s[:i]
	}
	if i := strings.LastIndexByte(s, '-'); i >= 0 {
		pre := parseConanVersion(s[i+1:])
		v.pre, s = &pre, s[:i]
	}
	for _, item := range strings.Split(s, ".") {
		v.main = append(v.main, conanItem{value: item, num: isDigits(item)})
	}
	for len(v.main) > 0 && v.main[len(v.main)-1].num && cmpNumStr(v.main[len(v.main)-1].value, "0") == 0 {
		v.main = v.main[:len(v.main)-1]
	}
	return v
}

func compareConanVersion(a, b conanVersion) int {
	if c := compareConanItems(a.main, b.main); c != 0 {
		return c
	}
	if c := compareOptionalConan(a.pre, b.pre, true); c != 0 {
		return c
	}
	return compareOptionalConan(a.build, b.build, false)
}

func compareConanItems(a, b []conanItem) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		var c int
		if a[i].num && b[i].num {
			c = cmpNumStr(a[i].value, b[i].value)
		} else {
			c = cmpString(a[i].value, b[i].value)
		}
		if c != 0 {
			return c
		}
	}
	return cmpInt(len(a), len(b))
}

func compareOptionalConan(a, b *conanVersion, prerelease bool) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		if prerelease {
			return 1
		}
		return -1
	}
	if b == nil {
		if prerelease {
			return -1
		}
		return 1
	}
	return compareConanVersion(*a, *b)
}

func compareGentoo(a, b string) int {
	va, ra := splitGentooRevision(a)
	vb, rb := splitGentooRevision(b)
	if va == vb {
		return cmpNumStr(ra, rb)
	}
	pa, pb := strings.Split(va, "_"), strings.Split(vb, "_")
	if c := compareGentooBase(pa[0], pb[0]); c != 0 {
		return c
	}
	if c := compareGentooSuffixes(pa[1:], pb[1:]); c != 0 {
		return c
	}
	return cmpNumStr(ra, rb)
}

func splitGentooRevision(s string) (version, revision string) {
	version = s
	if i := strings.LastIndex(s, "-r"); i >= 0 && isDigits(s[i+2:]) {
		version, revision = s[:i], s[i+2:]
	}
	return version, revision
}

func compareGentooBase(a, b string) int {
	pa, la := splitGentooBase(a)
	pb, lb := splitGentooBase(b)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] == pb[i] {
			continue
		}
		var c int
		if !strings.HasPrefix(pa[i], "0") && !strings.HasPrefix(pb[i], "0") {
			c = cmpNumStr(pa[i], pb[i])
		} else {
			c = cmpString(strings.TrimRight(pa[i], "0"), strings.TrimRight(pb[i], "0"))
		}
		if c != 0 {
			return c
		}
	}
	if len(pa) != len(pb) {
		return cmpInt(len(pa), len(pb))
	}
	return cmpInt(la, lb)
}

func splitGentooBase(s string) ([]string, int) {
	parts := strings.Split(s, ".")
	letter := -1
	last := parts[len(parts)-1]
	if len(last) > 0 && isASCIIAlpha(last[len(last)-1]) {
		letter = int(last[len(last)-1])
		parts[len(parts)-1] = last[:len(last)-1]
	}
	return parts, letter
}

func compareGentooSuffixes(a, b []string) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		if i >= len(a) {
			kind, number := parseGentooSuffix(b[i])
			if rank := gentooSuffixRank(kind); rank != 0 {
				return cmpInt(0, rank)
			}
			return cmpNumStr("0", number)
		}
		if i >= len(b) {
			kind, number := parseGentooSuffix(a[i])
			if rank := gentooSuffixRank(kind); rank != 0 {
				return cmpInt(rank, 0)
			}
			return cmpNumStr(number, "0")
		}
		ka, na := parseGentooSuffix(a[i])
		kb, nb := parseGentooSuffix(b[i])
		if c := cmpInt(gentooSuffixRank(ka), gentooSuffixRank(kb)); c != 0 {
			return c
		}
		if c := cmpNumStr(na, nb); c != 0 {
			return c
		}
	}
	return 0
}

func parseGentooSuffix(s string) (kind, number string) {
	i := len(s)
	for i > 0 && isASCIIDigit(s[i-1]) {
		i--
	}
	return s[:i], s[i:]
}

func gentooSuffixRank(s string) int {
	switch s {
	case "alpha":
		return -4
	case "beta":
		return -3
	case "pre":
		return -2
	case "rc":
		return -1
	case "p":
		return 1
	default:
		return 0
	}
}
