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

// nextTypedSegment splits the leading run of same-kind bytes off s. ok is
// false once s is exhausted, which is how an empty part reports that it has no
// segments at all.
func nextTypedSegment(s string) (seg typedSegment, rest string, ok bool) {
	if s == "" {
		return typedSegment{}, "", false
	}
	kind := segmentKind(s[0])
	i := 1
	for i < len(s) && segmentKind(s[i]) == kind {
		i++
	}
	return typedSegment{value: s[:i], kind: kind}, s[i:], true
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
	for {
		sa, restA, okA := nextTypedSegment(a)
		sb, restB, okB := nextTypedSegment(b)
		if !okA && !okB {
			return 0
		}
		if !okA {
			if sb.kind == segmentAlpha {
				return 1
			}
			return -1
		}
		if !okB {
			if sa.kind == segmentAlpha {
				return -1
			}
			return 1
		}
		if sa.kind != sb.kind {
			if sa.kind == segmentDigit {
				return 1
			}
			if sb.kind == segmentDigit {
				return -1
			}
			if sa.kind == segmentOther {
				return 1
			}
			return -1
		}
		var c int
		switch sa.kind {
		case segmentDigit:
			c = cmpNumStr(sa.value, sb.value)
		case segmentAlpha:
			c = cmpString(sa.value, sb.value)
		default:
			c = cmpInt(len(sa.value), len(sb.value))
		}
		if c != 0 {
			return c
		}
		a, b = restA, restB
	}
}

func compareConan(a, b string) int {
	mainA, preA, hasPreA, buildA, hasBuildA := splitConanVersion(a)
	mainB, preB, hasPreB, buildB, hasBuildB := splitConanVersion(b)

	if c := compareConanMain(mainA, mainB); c != 0 {
		return c
	}
	if c := compareOptionalConan(preA, hasPreA, preB, hasPreB, true); c != 0 {
		return c
	}
	return compareOptionalConan(buildA, hasBuildA, buildB, hasBuildB, false)
}

// splitConanVersion separates s into its main component list plus the optional
// prerelease and build parts, keeping every result as a view into s. The build
// part is taken first so that a prerelease containing a plus sign, such as
// 1.0-alpha+build, splits the same way the previous recursive parser did.
func splitConanVersion(s string) (main, pre string, hasPre bool, build string, hasBuild bool) {
	if i := strings.LastIndexByte(s, '+'); i >= 0 {
		build, hasBuild, s = s[i+1:], true, s[:i]
	}
	if i := strings.LastIndexByte(s, '-'); i >= 0 {
		pre, hasPre, s = s[i+1:], true, s[:i]
	}
	return s, pre, hasPre, build, hasBuild
}

func compareConanMain(a, b string) int {
	na, nb := conanMainLen(a), conanMainLen(b)
	for i := 0; i < na && i < nb; i++ {
		pa, restA, _ := nextDotPart(a)
		pb, restB, _ := nextDotPart(b)
		var c int
		if isDigits(pa) && isDigits(pb) {
			c = cmpNumStr(pa, pb)
		} else {
			c = cmpString(pa, pb)
		}
		if c != 0 {
			return c
		}
		a, b = restA, restB
	}
	return cmpInt(na, nb)
}

// conanMainLen counts the dot separated components in s, less the trailing
// numeric zero components conan treats as absent so that 1.2.0 and 1.2 compare
// equal.
func conanMainLen(s string) int {
	total, trailingZeros := 0, 0
	for {
		part, rest, more := nextDotPart(s)
		total++
		if isDigits(part) && cmpNumStr(part, "0") == 0 {
			trailingZeros++
		} else {
			trailingZeros = 0
		}
		if !more {
			return total - trailingZeros
		}
		s = rest
	}
}

// compareOptionalConan orders a present part against an absent one. A missing
// prerelease outranks a present one, while a missing build is outranked by a
// present one. Two present parts recurse through compareConan, which stays
// allocation free because every part is a view into the original string.
func compareOptionalConan(a string, hasA bool, b string, hasB bool, prerelease bool) int {
	if !hasA && !hasB {
		return 0
	}
	if !hasA {
		if prerelease {
			return 1
		}
		return -1
	}
	if !hasB {
		if prerelease {
			return -1
		}
		return 1
	}
	return compareConan(a, b)
}

func compareGentoo(a, b string) int {
	va, ra := splitGentooRevision(a)
	vb, rb := splitGentooRevision(b)
	if va == vb {
		return cmpNumStr(ra, rb)
	}
	baseA, suffixesA, hasSuffixA := splitGentooBase(va)
	baseB, suffixesB, hasSuffixB := splitGentooBase(vb)
	if c := compareGentooBase(baseA, baseB); c != 0 {
		return c
	}
	if c := compareGentooSuffixes(suffixesA, hasSuffixA, suffixesB, hasSuffixB); c != 0 {
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

// splitGentooBase separates the base component list from the underscore
// separated suffixes. hasSuffixes distinguishes a version with no underscore
// from one such as 1.2.3_ whose single suffix is empty.
func splitGentooBase(s string) (base, suffixes string, hasSuffixes bool) {
	if i := strings.IndexByte(s, '_'); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return s, "", false
}

func compareGentooBase(a, b string) int {
	a, letterA := splitGentooLetter(a)
	b, letterB := splitGentooLetter(b)

	for i := 0; ; i++ {
		pa, restA, moreA := nextDotPart(a)
		pb, restB, moreB := nextDotPart(b)
		if c := compareGentooComponent(i, pa, pb); c != 0 {
			return c
		}
		if !moreA || !moreB {
			if moreA != moreB {
				return cmpInt(boolInt(moreA), boolInt(moreB))
			}
			return cmpInt(letterA, letterB)
		}
		a, b = restA, restB
	}
}

// splitGentooLetter strips the single trailing letter a version such as 1.2.3a
// may carry. letter is -1 when there is none, which sorts it below any letter.
func splitGentooLetter(s string) (rest string, letter int) {
	if s != "" && isASCIIAlpha(s[len(s)-1]) {
		return s[:len(s)-1], int(s[len(s)-1])
	}
	return s, -1
}

// compareGentooComponent orders one pair of base components. Only the leading
// component is always numeric; a later component starting with a zero is
// compared as a fraction, with trailing zeros ignored.
func compareGentooComponent(index int, a, b string) int {
	if a == b {
		return 0
	}
	if index == 0 || (!strings.HasPrefix(a, "0") && !strings.HasPrefix(b, "0")) {
		return cmpNumStr(a, b)
	}
	return cmpString(strings.TrimRight(a, "0"), strings.TrimRight(b, "0"))
}

func compareGentooSuffixes(a string, hasA bool, b string, hasB bool) int {
	for {
		if !hasA && !hasB {
			return 0
		}
		if !hasA {
			suffix, _, _ := nextGentooSuffix(b)
			kind, number := parseGentooSuffix(suffix)
			if rank := gentooSuffixRank(kind); rank != 0 {
				return cmpInt(0, rank)
			}
			return cmpNumStr("0", number)
		}
		if !hasB {
			suffix, _, _ := nextGentooSuffix(a)
			kind, number := parseGentooSuffix(suffix)
			if rank := gentooSuffixRank(kind); rank != 0 {
				return cmpInt(rank, 0)
			}
			return cmpNumStr(number, "0")
		}
		suffixA, restA, moreA := nextGentooSuffix(a)
		suffixB, restB, moreB := nextGentooSuffix(b)
		ka, na := parseGentooSuffix(suffixA)
		kb, nb := parseGentooSuffix(suffixB)
		if c := cmpInt(gentooSuffixRank(ka), gentooSuffixRank(kb)); c != 0 {
			return c
		}
		if c := cmpNumStr(na, nb); c != 0 {
			return c
		}
		a, b, hasA, hasB = restA, restB, moreA, moreB
	}
}

// nextGentooSuffix splits the leading underscore separated suffix off s.
func nextGentooSuffix(s string) (suffix, rest string, more bool) {
	if i := strings.IndexByte(s, '_'); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return s, "", false
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
