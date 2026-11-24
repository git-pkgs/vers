package vers

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func joinN(n int, sep string, item func(int) string) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = item(i)
	}
	return strings.Join(parts, sep)
}

// Every list the parsers split on, at the default limit and one above it.
var constraintListCases = []struct {
	name   string
	scheme string // "" = vers URI through Parse
	sep    string
	item   func(int) string
}{
	{"vers URI", "", "|", func(i int) string { return fmt.Sprintf("!=%d.0", i) }},
	{"generic", "generic", "|", func(i int) string { return fmt.Sprintf("!=%d.0", i) }},
	{"npm or", "npm", " || ", func(i int) string { return fmt.Sprintf("%d.0.0", i) }},
	{"npm and", "npm", " ", func(i int) string { return fmt.Sprintf(">=%d.0.0", i) }},
	{"gem", "gem", ",", func(i int) string { return fmt.Sprintf("!= %d.0", i) }},
	{"pypi", "pypi", ",", func(i int) string { return fmt.Sprintf("!=%d.0", i) }},
	{"cargo", "cargo", ",", func(i int) string { return fmt.Sprintf(">=%d.0.0", i) }},
	{"composer or", "composer", " || ", func(i int) string { return fmt.Sprintf("%d.0", i) }},
	{"composer and", "composer", " ", func(i int) string { return fmt.Sprintf("!=%d.0", i) }},
	{"pub", "pub", " ", func(i int) string { return fmt.Sprintf(">=%d.0.0", i) }},
	{"hex or", "hex", " or ", func(i int) string { return fmt.Sprintf("== %d.0.0", i) }},
	{"hex and", "hex", " and ", func(i int) string { return fmt.Sprintf("!= %d.0.0", i) }},
}

func parseCase(p *Parser, scheme, input string) error {
	if scheme == "" {
		_, err := p.Parse("vers:pypi/" + input)
		return err
	}
	_, err := p.ParseNative(input, scheme)
	return err
}

func TestConstraintLimit(t *testing.T) {
	for _, tc := range constraintListCases {
		t.Run(tc.name, func(t *testing.T) {
			atLimit := joinN(DefaultMaxConstraints, tc.sep, tc.item)
			if err := parseCase(NewParser(), tc.scheme, atLimit); errors.Is(err, ErrTooManyConstraints) {
				t.Errorf("%d constraints rejected: %v", DefaultMaxConstraints, err)
			}
			over := joinN(DefaultMaxConstraints+1, tc.sep, tc.item)
			if len(over) > DefaultMaxLength {
				t.Fatalf("fixture is %d bytes, over the length limit", len(over))
			}
			for name, p := range map[string]*Parser{"NewParser": NewParser(), "zero Parser": {}} {
				if err := parseCase(p, tc.scheme, over); !errors.Is(err, ErrTooManyConstraints) {
					t.Errorf("%s: %d constraints: got %v, want ErrTooManyConstraints", name, DefaultMaxConstraints+1, err)
				}
			}
			raised := &Parser{MaxConstraints: DefaultMaxConstraints + 1}
			if err := parseCase(raised, tc.scheme, over); errors.Is(err, ErrTooManyConstraints) {
				t.Errorf("MaxConstraints %d: %v", DefaultMaxConstraints+1, err)
			}
		})
	}
}

func TestLengthLimit(t *testing.T) {
	versAt := "vers:npm/>=1.0.0|<" + strings.Repeat("9", DefaultMaxLength-len("vers:npm/>=1.0.0|<"))
	nativeAt := ">=1.0.0 <" + strings.Repeat("9", DefaultMaxLength-len(">=1.0.0 <"))
	for _, tc := range []struct {
		name  string
		parse func(*Parser, string) error
		input string
	}{
		{"Parse", func(p *Parser, s string) error { _, err := p.Parse(s); return err }, versAt},
		{"ParseNative", func(p *Parser, s string) error { _, err := p.ParseNative(s, "npm"); return err }, nativeAt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.parse(NewParser(), tc.input); errors.Is(err, ErrRangeTooLong) {
				t.Errorf("%d bytes rejected: %v", len(tc.input), err)
			}
			over := tc.input + "9"
			for name, p := range map[string]*Parser{"NewParser": NewParser(), "zero Parser": {}} {
				err := tc.parse(p, over)
				if !errors.Is(err, ErrRangeTooLong) {
					t.Fatalf("%s: %d bytes: got %v, want ErrRangeTooLong", name, len(over), err)
				}
				if want := fmt.Sprintf("(%d > %d)", len(over), DefaultMaxLength); !strings.Contains(err.Error(), want) {
					t.Errorf("%s: error %q does not say %s", name, err, want)
				}
			}
			if err := tc.parse(&Parser{MaxLength: len(over)}, over); errors.Is(err, ErrRangeTooLong) {
				t.Errorf("MaxLength %d: %v", len(over), err)
			}
			if err := tc.parse(&Parser{MaxLength: 10}, tc.input[:11]); !errors.Is(err, ErrRangeTooLong) {
				t.Errorf("MaxLength 10, 11 bytes: got %v", err)
			}
		})
	}
}

func TestPackageFunctionsUseDefaultLimits(t *testing.T) {
	if _, err := Parse("vers:pypi/" + joinN(DefaultMaxConstraints+1, "|", func(i int) string { return fmt.Sprintf("!=%d", i) })); !errors.Is(err, ErrTooManyConstraints) {
		t.Errorf("Parse: got %v", err)
	}
	if _, err := ParseNative(strings.Repeat("1", DefaultMaxLength+1), "npm"); !errors.Is(err, ErrRangeTooLong) {
		t.Errorf("ParseNative: got %v", err)
	}
}
