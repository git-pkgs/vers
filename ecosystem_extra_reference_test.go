package vers

// This file is a mechanically renamed copy of the pre-optimization
// implementations of compareALPM, compareConan and compareGentoo. It exists so
// the differential tests in ecosystem_extra_parity_test.go can assert that the
// allocation-free rewrites order every input exactly the way the previous code
// did. Do not edit it to fix a behavior difference; fix the real
// implementation instead.

import "strings"

const (
	refSegmentDigit = iota
	refSegmentAlpha
	refSegmentOther
)

type refTypedSegment struct {
	value string
	kind  int
}

func refCompareALPM(a, b string) int {
	ea, va, ra, hasRA := refSplitALPMVersion(a)
	eb, vb, rb, hasRB := refSplitALPMVersion(b)
	if c := refCompareALPMPart(ea, eb); c != 0 {
		return c
	}
	if c := refCompareALPMPart(va, vb); c != 0 {
		return c
	}
	if hasRA && hasRB {
		return refCompareALPMPart(ra, rb)
	}
	return 0
}

func refSplitALPMVersion(s string) (epoch, version, release string, hasRelease bool) {
	epoch, version = "0", s
	if i := strings.IndexByte(version, ':'); i >= 0 {
		epoch, version = version[:i], version[i+1:]
	}
	if i := strings.LastIndexByte(version, '-'); i >= 0 {
		version, release, hasRelease = version[:i], version[i+1:], true
	}
	return epoch, version, release, hasRelease
}

func refSplitTypedSegments(s string) []refTypedSegment {
	if s == "" {
		return nil
	}
	segments := make([]refTypedSegment, 0)
	start, kind := 0, refSegmentKind(s[0])
	for i := 1; i < len(s); i++ {
		if next := refSegmentKind(s[i]); next != kind {
			segments = append(segments, refTypedSegment{value: s[start:i], kind: kind})
			start, kind = i, next
		}
	}
	return append(segments, refTypedSegment{value: s[start:], kind: kind})
}

func refSegmentKind(c byte) int {
	if isASCIIDigit(c) {
		return refSegmentDigit
	}
	if isASCIIAlpha(c) {
		return refSegmentAlpha
	}
	return refSegmentOther
}

func refCompareALPMPart(a, b string) int {
	pa, pb := refSplitTypedSegments(a), refSplitTypedSegments(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		if i >= len(pa) {
			if pb[i].kind == refSegmentAlpha {
				return 1
			}
			return -1
		}
		if i >= len(pb) {
			if pa[i].kind == refSegmentAlpha {
				return -1
			}
			return 1
		}
		a, b := pa[i], pb[i]
		if a.kind != b.kind {
			if a.kind == refSegmentDigit {
				return 1
			}
			if b.kind == refSegmentDigit {
				return -1
			}
			if a.kind == refSegmentOther {
				return 1
			}
			return -1
		}
		var c int
		switch a.kind {
		case refSegmentDigit:
			c = cmpNumStr(a.value, b.value)
		case refSegmentAlpha:
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

type refConanVersion struct {
	main  []refConanItem
	pre   *refConanVersion
	build *refConanVersion
}

type refConanItem struct {
	value string
	num   bool
}

func refCompareConan(a, b string) int {
	return refCompareConanVersion(refParseConanVersion(a), refParseConanVersion(b))
}

func refParseConanVersion(s string) refConanVersion {
	v := refConanVersion{}
	if i := strings.LastIndexByte(s, '+'); i >= 0 {
		build := refParseConanVersion(s[i+1:])
		v.build, s = &build, s[:i]
	}
	if i := strings.LastIndexByte(s, '-'); i >= 0 {
		pre := refParseConanVersion(s[i+1:])
		v.pre, s = &pre, s[:i]
	}
	for _, item := range strings.Split(s, ".") {
		v.main = append(v.main, refConanItem{value: item, num: isDigits(item)})
	}
	for len(v.main) > 0 && v.main[len(v.main)-1].num && cmpNumStr(v.main[len(v.main)-1].value, "0") == 0 {
		v.main = v.main[:len(v.main)-1]
	}
	return v
}

func refCompareConanVersion(a, b refConanVersion) int {
	if c := refCompareConanItems(a.main, b.main); c != 0 {
		return c
	}
	if c := refCompareOptionalConan(a.pre, b.pre, true); c != 0 {
		return c
	}
	return refCompareOptionalConan(a.build, b.build, false)
}

func refCompareConanItems(a, b []refConanItem) int {
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

func refCompareOptionalConan(a, b *refConanVersion, prerelease bool) int {
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
	return refCompareConanVersion(*a, *b)
}

func refCompareGentoo(a, b string) int {
	va, ra := refSplitGentooRevision(a)
	vb, rb := refSplitGentooRevision(b)
	if va == vb {
		return cmpNumStr(ra, rb)
	}
	pa, pb := strings.Split(va, "_"), strings.Split(vb, "_")
	if c := refCompareGentooBase(pa[0], pb[0]); c != 0 {
		return c
	}
	if c := refCompareGentooSuffixes(pa[1:], pb[1:]); c != 0 {
		return c
	}
	return cmpNumStr(ra, rb)
}

func refSplitGentooRevision(s string) (version, revision string) {
	version = s
	if i := strings.LastIndex(s, "-r"); i >= 0 && isDigits(s[i+2:]) {
		version, revision = s[:i], s[i+2:]
	}
	return version, revision
}

func refCompareGentooBase(a, b string) int {
	pa, la := refSplitGentooBase(a)
	pb, lb := refSplitGentooBase(b)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] == pb[i] {
			continue
		}
		var c int
		if i == 0 || (!strings.HasPrefix(pa[i], "0") && !strings.HasPrefix(pb[i], "0")) {
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

func refSplitGentooBase(s string) ([]string, int) {
	parts := strings.Split(s, ".")
	letter := -1
	last := parts[len(parts)-1]
	if len(last) > 0 && isASCIIAlpha(last[len(last)-1]) {
		letter = int(last[len(last)-1])
		parts[len(parts)-1] = last[:len(last)-1]
	}
	return parts, letter
}

func refCompareGentooSuffixes(a, b []string) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		if i >= len(a) {
			kind, number := refParseGentooSuffix(b[i])
			if rank := refGentooSuffixRank(kind); rank != 0 {
				return cmpInt(0, rank)
			}
			return cmpNumStr("0", number)
		}
		if i >= len(b) {
			kind, number := refParseGentooSuffix(a[i])
			if rank := refGentooSuffixRank(kind); rank != 0 {
				return cmpInt(rank, 0)
			}
			return cmpNumStr(number, "0")
		}
		ka, na := refParseGentooSuffix(a[i])
		kb, nb := refParseGentooSuffix(b[i])
		if c := cmpInt(refGentooSuffixRank(ka), refGentooSuffixRank(kb)); c != 0 {
			return c
		}
		if c := cmpNumStr(na, nb); c != 0 {
			return c
		}
	}
	return 0
}

func refParseGentooSuffix(s string) (kind, number string) {
	i := len(s)
	for i > 0 && isASCIIDigit(s[i-1]) {
		i--
	}
	return s[:i], s[i:]
}

func refGentooSuffixRank(s string) int {
	switch s {
	case qualifierAlpha:
		return -4
	case qualifierBeta:
		return -3
	case qualifierPre:
		return -2
	case "rc":
		return -1
	case "p":
		return 1
	default:
		return 0
	}
}
