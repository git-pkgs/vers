package vers

import "testing"

func TestRangeContains(t *testing.T) {
	tests := []struct {
		name    string
		r       *Range
		version string
		want    bool
	}{
		{
			"single interval contains",
			NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, false)}),
			"1.5.0",
			true,
		},
		{
			"single interval excludes below",
			NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, false)}),
			"0.9.0",
			false,
		},
		{
			"single interval excludes above",
			NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, false)}),
			"2.0.0",
			false,
		},
		{
			"multiple intervals (union)",
			NewRange([]Interval{
				NewInterval("1.0.0", "2.0.0", true, true),
				NewInterval("3.0.0", "4.0.0", true, true),
			}),
			"3.5.0",
			true,
		},
		{
			"gap between intervals",
			NewRange([]Interval{
				NewInterval("1.0.0", "2.0.0", true, true),
				NewInterval("3.0.0", "4.0.0", true, true),
			}),
			"2.5.0",
			false,
		},
		{
			"exclusion",
			&Range{
				Intervals:  []Interval{NewInterval("1.0.0", "3.0.0", true, true)},
				Exclusions: []string{"2.0.0"},
			},
			"2.0.0",
			false,
		},
		{
			"exclusion allows other versions",
			&Range{
				Intervals:  []Interval{NewInterval("1.0.0", "3.0.0", true, true)},
				Exclusions: []string{"2.0.0"},
			},
			"2.1.0",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestRangeIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		r    *Range
		want bool
	}{
		{"no intervals", &Range{}, true},
		{"empty intervals only", NewRange([]Interval{EmptyInterval()}), true},
		{"valid interval", NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, true)}), false},
		{"unbounded", Unbounded(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.IsEmpty()
			if got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRangeIsUnbounded(t *testing.T) {
	tests := []struct {
		name string
		r    *Range
		want bool
	}{
		{"unbounded", Unbounded(), true},
		{"bounded", NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, true)}), false},
		{
			"unbounded with exclusion",
			&Range{
				Intervals:  []Interval{UnboundedInterval()},
				Exclusions: []string{"1.0.0"},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.IsUnbounded()
			if got != tt.want {
				t.Errorf("IsUnbounded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRangeUnion(t *testing.T) {
	tests := []struct {
		name    string
		r1      *Range
		r2      *Range
		version string
		want    bool
	}{
		{
			"union of non-overlapping",
			NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, true)}),
			NewRange([]Interval{NewInterval("3.0.0", "4.0.0", true, true)}),
			"3.5.0",
			true,
		},
		{
			"union of overlapping",
			NewRange([]Interval{NewInterval("1.0.0", "3.0.0", true, true)}),
			NewRange([]Interval{NewInterval("2.0.0", "4.0.0", true, true)}),
			"3.5.0",
			true,
		},
		{
			"union preserves first range",
			NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, true)}),
			NewRange([]Interval{NewInterval("3.0.0", "4.0.0", true, true)}),
			"1.5.0",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.r1.Union(tt.r2)
			got := result.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Union().Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestRangeIntersect(t *testing.T) {
	tests := []struct {
		name    string
		r1      *Range
		r2      *Range
		version string
		want    bool
	}{
		{
			"intersection of overlapping",
			NewRange([]Interval{NewInterval("1.0.0", "3.0.0", true, true)}),
			NewRange([]Interval{NewInterval("2.0.0", "4.0.0", true, true)}),
			"2.5.0",
			true,
		},
		{
			"intersection excludes non-overlap from first",
			NewRange([]Interval{NewInterval("1.0.0", "3.0.0", true, true)}),
			NewRange([]Interval{NewInterval("2.0.0", "4.0.0", true, true)}),
			"1.5.0",
			false,
		},
		{
			"intersection excludes non-overlap from second",
			NewRange([]Interval{NewInterval("1.0.0", "3.0.0", true, true)}),
			NewRange([]Interval{NewInterval("2.0.0", "4.0.0", true, true)}),
			"3.5.0",
			false,
		},
		{
			"non-overlapping produces empty",
			NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, true)}),
			NewRange([]Interval{NewInterval("3.0.0", "4.0.0", true, true)}),
			"1.5.0",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.r1.Intersect(tt.r2)
			got := result.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Intersect().Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestRangeExclude(t *testing.T) {
	r := Unbounded().Exclude("1.5.0")

	if !r.Contains("1.0.0") {
		t.Error("should contain 1.0.0")
	}
	if r.Contains("1.5.0") {
		t.Error("should not contain excluded 1.5.0")
	}
	if !r.Contains("2.0.0") {
		t.Error("should contain 2.0.0")
	}
}

func TestRangeString(t *testing.T) {
	tests := []struct {
		name string
		r    *Range
		want string
	}{
		{"empty", &Range{}, "empty"},
		{"unbounded", Unbounded(), "*"},
		{
			"single interval",
			NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, false)}),
			"[1.0.0,2.0.0)",
		},
		{
			"multiple intervals",
			NewRange([]Interval{
				NewInterval("1.0.0", "2.0.0", true, true),
				NewInterval("3.0.0", "4.0.0", true, true),
			}),
			"[1.0.0,2.0.0] | [3.0.0,4.0.0]",
		},
		{
			"with exclusion",
			&Range{
				Intervals:  []Interval{NewInterval("1.0.0", "3.0.0", true, true)},
				Exclusions: []string{"2.0.0"},
			},
			"[1.0.0,3.0.0] excluding 2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExact(t *testing.T) {
	r := Exact("1.2.3")

	if !r.Contains("1.2.3") {
		t.Error("Exact should contain its version")
	}
	if r.Contains("1.2.4") {
		t.Error("Exact should not contain other versions")
	}
}

func TestUnbounded(t *testing.T) {
	r := Unbounded()

	if !r.Contains("0.0.0") {
		t.Error("Unbounded should contain any version")
	}
	if !r.Contains("999.999.999") {
		t.Error("Unbounded should contain any version")
	}
	if !r.IsUnbounded() {
		t.Error("IsUnbounded() should return true")
	}
}
