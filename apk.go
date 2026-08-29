package vers

import "strings"

// APK version comparison follows the tokenizer in apk-tools src/version.c
// at commit 900a3f5280bfad7ea22eda7de46d0d4c0c9ce8f2.

const (
	apkTokenInitialDigit = iota
	apkTokenDigit
	apkTokenLetter
	apkTokenSuffix
	apkTokenSuffixNumber
	apkTokenCommitHash
	apkTokenRevisionNumber
	apkTokenEnd
	apkTokenInvalid
)

const apkSuffixNone = 5

var apkSuffixRank = map[string]int{
	"alpha": 1,
	"beta":  2,
	"pre":   3,
	"rc":    4,
	"cvs":   6,
	"svn":   7,
	"git":   8,
	"hg":    9,
	"p":     10,
}

type apkToken struct {
	kind   int
	number uint64
	text   string
}

func compareAPK(a, b string) int {
	left := parseAPKVersion(a)
	right := parseAPKVersion(b)

	i := 0
	for apkTokenType(left, i) == apkTokenType(right, i) && apkTokenType(left, i) < apkTokenEnd {
		if c := compareAPKToken(left[i], right[i]); c != 0 {
			return c
		}
		i++
	}

	lt, rt := apkTokenType(left, i), apkTokenType(right, i)
	if lt == rt {
		return 0
	}
	if lt == apkTokenSuffix && left[i].number < apkSuffixNone {
		return -1
	}
	if rt == apkTokenSuffix && right[i].number < apkSuffixNone {
		return 1
	}
	if lt > rt {
		return -1
	}
	return 1
}

func compareAPKToken(a, b apkToken) int {
	switch a.kind {
	case apkTokenDigit:
		if strings.HasPrefix(a.text, "0") || strings.HasPrefix(b.text, "0") {
			return cmpString(a.text, b.text)
		}
		return cmpUint64(a.number, b.number)
	case apkTokenInitialDigit, apkTokenSuffixNumber, apkTokenRevisionNumber, apkTokenLetter, apkTokenSuffix:
		return cmpUint64(a.number, b.number)
	default:
		return cmpString(a.text, b.text)
	}
}

func validAPKVersion(s string) bool {
	tokens := parseAPKVersion(s)
	for _, t := range tokens {
		if t.kind == apkTokenInvalid {
			return false
		}
	}
	return len(tokens) > 0
}

func apkVersionIsPrerelease(s string) bool {
	for _, t := range parseAPKVersion(s) {
		if t.kind == apkTokenInvalid {
			return false
		}
		if t.kind == apkTokenSuffix && t.number < apkSuffixNone {
			return true
		}
	}
	return false
}

func parseAPKVersion(s string) []apkToken {
	s = strings.TrimSpace(s)
	tokens := make([]apkToken, 0, 6) //nolint:mnd

	first, i := scanAPKDigits(s, 0)
	if first == "" {
		return append(tokens, apkToken{kind: apkTokenInvalid})
	}
	tokens = append(tokens, apkToken{kind: apkTokenInitialDigit, number: apkUint64(first), text: first})
	previous := apkTokenInitialDigit

	for i < len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			if previous > apkTokenDigit {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			tokens = append(tokens, apkToken{kind: apkTokenLetter, number: uint64(c)})
			previous = apkTokenLetter
			i++
		case c == '.':
			if previous > apkTokenDigit {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			digits, next := scanAPKDigits(s, i+1)
			if digits == "" {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			tokens = append(tokens, apkToken{kind: apkTokenDigit, number: apkUint64(digits), text: digits})
			previous = apkTokenDigit
			i = next
		case c >= '0' && c <= '9':
			var kind int
			switch previous {
			case apkTokenInitialDigit, apkTokenDigit:
				kind = apkTokenDigit
			case apkTokenSuffix:
				kind = apkTokenSuffixNumber
			default:
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			digits, next := scanAPKDigits(s, i)
			tokens = append(tokens, apkToken{kind: kind, number: apkUint64(digits), text: digits})
			previous = kind
			i = next
		case c == '_':
			if previous > apkTokenSuffixNumber {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			suffix, next := scanAPKLower(s, i+1)
			rank, ok := apkSuffixRank[suffix]
			if !ok {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			tokens = append(tokens, apkToken{kind: apkTokenSuffix, number: uint64(rank)})
			previous = apkTokenSuffix
			i = next
		case c == '~':
			if previous >= apkTokenCommitHash {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			hash, next := scanAPKHex(s, i+1)
			if hash == "" {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			tokens = append(tokens, apkToken{kind: apkTokenCommitHash, text: hash})
			previous = apkTokenCommitHash
			i = next
		case c == '-':
			if previous >= apkTokenRevisionNumber || i+1 >= len(s) || s[i+1] != 'r' {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			digits, next := scanAPKDigits(s, i+2)
			if digits == "" {
				return append(tokens, apkToken{kind: apkTokenInvalid})
			}
			tokens = append(tokens, apkToken{kind: apkTokenRevisionNumber, number: apkUint64(digits), text: digits})
			previous = apkTokenRevisionNumber
			i = next
		default:
			return append(tokens, apkToken{kind: apkTokenInvalid})
		}
	}

	return tokens
}

func apkTokenType(tokens []apkToken, i int) int {
	if i < len(tokens) {
		return tokens[i].kind
	}
	return apkTokenEnd
}

func apkUint64(s string) uint64 {
	var n uint64
	for i := 0; i < len(s); i++ {
		n = n*10 + uint64(s[i]-'0')
	}
	return n
}

func scanAPKDigits(s string, i int) (string, int) {
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[start:i], i
}

func scanAPKLower(s string, i int) (string, int) {
	start := i
	for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
		i++
	}
	return s[start:i], i
}

func scanAPKHex(s string, i int) (string, int) {
	start := i
	for i < len(s) {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			i++
		} else {
			break
		}
	}
	return s[start:i], i
}

func cmpUint64(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
