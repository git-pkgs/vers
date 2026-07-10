package vers

import (
	"regexp"
	"strconv"
	"strings"
)

// PEP 440 version parsing and comparison.
// https://peps.python.org/pep-0440/
//
// Version form: [N!]N(.N)*[{a|b|rc}N][.postN][.devN][+local]
// Ordering: .devN < aN < bN < rcN < (release) < .postN

// pep440Regex is derived from the canonical regex in PEP 440 Appendix B,
// simplified to the parts we need for ordering.
var pep440Regex = regexp.MustCompile(`(?i)^\s*v?` +
	`(?:(\d+)!)?` + // 1: epoch
	`(\d+(?:\.\d+)*)` + // 2: release
	`(?:[-_.]?(alpha|beta|preview|pre|rc|a|b|c)[-_.]?(\d*))?` + // 3,4: pre
	`(?:(?:[-_.]?(post|rev|r)[-_.]?(\d*))|(?:-(\d+)))?` + // 5,6,7: post (or -N implicit post)
	`(?:[-_.]?(dev)[-_.]?(\d*))?` + // 8,9: dev
	`(?:\+([a-z0-9]+(?:[-_.][a-z0-9]+)*))?` + // 10: local
	`\s*$`)

const (
	pep440NegInf = -1
	pep440PosInf = 1 << 30 //nolint:mnd
)

//nolint:goconst,mnd
var pep440PreTags = map[string]int{
	"a": 0, "alpha": 0,
	"b": 1, "beta": 1,
	"c": 2, "rc": 2, "pre": 2, "preview": 2,
}

type pep440Version struct {
	epoch   int
	release []int
	preTag  int // pep440NegInf, pep440PosInf, or 0/1/2
	preNum  int
	post    int // pep440NegInf or post number
	dev     int // pep440PosInf or dev number
	local   []pep440LocalPart
}

type pep440LocalPart struct {
	num   int
	str   string
	isNum bool
}

func parsePEP440(s string) (pep440Version, bool) {
	m := pep440Regex.FindStringSubmatch(s)
	if m == nil {
		return pep440Version{}, false
	}

	v := pep440Version{
		preTag: pep440PosInf,
		post:   pep440NegInf,
		dev:    pep440PosInf,
	}

	if m[1] != "" {
		v.epoch, _ = strconv.Atoi(m[1])
	}

	for _, p := range strings.Split(m[2], ".") {
		n, _ := strconv.Atoi(p)
		v.release = append(v.release, n)
	}
	// Trim trailing zeros so 1.0 == 1.0.0
	for len(v.release) > 1 && v.release[len(v.release)-1] == 0 {
		v.release = v.release[:len(v.release)-1]
	}

	hasPre := m[3] != ""
	if hasPre {
		v.preTag = pep440PreTags[strings.ToLower(m[3])]
		v.preNum, _ = strconv.Atoi(m[4]) // "" -> 0, matches PEP 440 implicit 0
	}

	hasPost := m[5] != "" || m[7] != ""
	if hasPost {
		if m[7] != "" {
			v.post, _ = strconv.Atoi(m[7])
		} else {
			v.post, _ = strconv.Atoi(m[6])
		}
	}

	hasDev := m[8] != ""
	if hasDev {
		v.dev, _ = strconv.Atoi(m[9])
	}

	// PEP 440: a version with only .dev (no pre, no post) sorts before
	// all pre-releases of the same release.
	if !hasPre && !hasPost && hasDev {
		v.preTag = pep440NegInf
	}

	if m[10] != "" {
		v.local = parsePEP440Local(m[10])
	}

	return v, true
}

func parsePEP440Local(s string) []pep440LocalPart {
	raw := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	parts := make([]pep440LocalPart, 0, len(raw))
	for _, p := range raw {
		if n, err := strconv.Atoi(p); err == nil {
			parts = append(parts, pep440LocalPart{num: n, isNum: true})
		} else {
			parts = append(parts, pep440LocalPart{str: p})
		}
	}
	return parts
}

// comparePyPI compares two PEP 440 version strings.
// Falls back to generic comparison if either side is not valid PEP 440.
func comparePyPI(a, b string) int {
	va, okA := parsePEP440(a)
	vb, okB := parsePEP440(b)
	if !okA || !okB {
		return CompareVersions(a, b)
	}

	if c := cmpInt(va.epoch, vb.epoch); c != 0 {
		return c
	}
	if c := cmpIntSlice(va.release, vb.release); c != 0 {
		return c
	}
	if c := cmpInt(va.preTag, vb.preTag); c != 0 {
		return c
	}
	if c := cmpInt(va.preNum, vb.preNum); c != 0 {
		return c
	}
	if c := cmpInt(va.post, vb.post); c != 0 {
		return c
	}
	if c := cmpInt(va.dev, vb.dev); c != 0 {
		return c
	}
	return cmpPEP440Local(va.local, vb.local)
}

func cmpIntSlice(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if c := cmpInt(x, y); c != 0 {
			return c
		}
	}
	return 0
}

func cmpPEP440Local(a, b []pep440LocalPart) int {
	// No local sorts before any local.
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return -1
	}
	if len(b) == 0 {
		return 1
	}
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if i >= len(a) {
			return -1
		}
		if i >= len(b) {
			return 1
		}
		pa, pb := a[i], b[i]
		// Numeric segments sort after strings; within kind, natural order.
		if pa.isNum != pb.isNum {
			if pa.isNum {
				return 1
			}
			return -1
		}
		if pa.isNum {
			if c := cmpInt(pa.num, pb.num); c != 0 {
				return c
			}
		} else {
			if c := cmpString(pa.str, pb.str); c != 0 {
				return c
			}
		}
	}
	return 0
}
