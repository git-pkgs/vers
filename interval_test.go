package vers

import "testing"

func TestNewInterval(t *testing.T) {
	i := NewInterval("1.0.0", "2.0.0", true, false)
	if i.Min != "1.0.0" {
		t.Errorf("Min = %q, want %q", i.Min, "1.0.0")
	}
	if i.Max != "2.0.0" {
		t.Errorf("Max = %q, want %q", i.Max, "2.0.0")
	}
	if !i.MinInclusive {
		t.Error("MinInclusive = false, want true")
	}
	if i.MaxInclusive {
		t.Error("MaxInclusive = true, want false")
	}
}

func TestExactInterval(t *testing.T) {
	i := ExactInterval("1.2.3")
	if i.Min != "1.2.3" || i.Max != "1.2.3" {
		t.Errorf("ExactInterval bounds incorrect: [%s, %s]", i.Min, i.Max)
	}
	if !i.MinInclusive || !i.MaxInclusive {
		t.Error("ExactInterval should be inclusive on both ends")
	}
	if !i.Contains("1.2.3") {
		t.Error("ExactInterval should contain its version")
	}
	if i.Contains("1.2.4") {
		t.Error("ExactInterval should not contain other versions")
	}
}

func TestGreaterThanInterval(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		inclusive bool
		check     string
		want      bool
	}{
		{">1.0.0 contains 1.0.1", "1.0.0", false, "1.0.1", true},
		{">1.0.0 excludes 1.0.0", "1.0.0", false, "1.0.0", false},
		{">1.0.0 excludes 0.9.9", "1.0.0", false, "0.9.9", false},
		{">=1.0.0 contains 1.0.0", "1.0.0", true, "1.0.0", true},
		{">=1.0.0 contains 1.0.1", "1.0.0", true, "1.0.1", true},
		{">=1.0.0 excludes 0.9.9", "1.0.0", true, "0.9.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := GreaterThanInterval(tt.version, tt.inclusive)
			got := i.Contains(tt.check)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}

func TestLessThanInterval(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		inclusive bool
		check     string
		want      bool
	}{
		{"<2.0.0 contains 1.9.9", "2.0.0", false, "1.9.9", true},
		{"<2.0.0 excludes 2.0.0", "2.0.0", false, "2.0.0", false},
		{"<2.0.0 excludes 2.0.1", "2.0.0", false, "2.0.1", false},
		{"<=2.0.0 contains 2.0.0", "2.0.0", true, "2.0.0", true},
		{"<=2.0.0 contains 1.9.9", "2.0.0", true, "1.9.9", true},
		{"<=2.0.0 excludes 2.0.1", "2.0.0", true, "2.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := LessThanInterval(tt.version, tt.inclusive)
			got := i.Contains(tt.check)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}

func TestIntervalIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		i    Interval
		want bool
	}{
		{"empty interval", EmptyInterval(), true},
		{"min > max", NewInterval("2.0.0", "1.0.0", true, true), true},
		{"equal exclusive min", NewInterval("1.0.0", "1.0.0", false, true), true},
		{"equal exclusive max", NewInterval("1.0.0", "1.0.0", true, false), true},
		{"equal exclusive both", NewInterval("1.0.0", "1.0.0", false, false), true},
		{"equal inclusive", NewInterval("1.0.0", "1.0.0", true, true), false},
		{"valid range", NewInterval("1.0.0", "2.0.0", true, false), false},
		{"unbounded", UnboundedInterval(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.i.IsEmpty()
			if got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntervalIsUnbounded(t *testing.T) {
	tests := []struct {
		name string
		i    Interval
		want bool
	}{
		{"unbounded", UnboundedInterval(), true},
		{"has min", GreaterThanInterval("1.0.0", true), false},
		{"has max", LessThanInterval("2.0.0", true), false},
		{"has both", NewInterval("1.0.0", "2.0.0", true, true), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.i.IsUnbounded()
			if got != tt.want {
				t.Errorf("IsUnbounded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntervalContains(t *testing.T) {
	tests := []struct {
		name    string
		i       Interval
		version string
		want    bool
	}{
		{"[1.0.0,2.0.0] contains 1.0.0", NewInterval("1.0.0", "2.0.0", true, true), "1.0.0", true},
		{"[1.0.0,2.0.0] contains 1.5.0", NewInterval("1.0.0", "2.0.0", true, true), "1.5.0", true},
		{"[1.0.0,2.0.0] contains 2.0.0", NewInterval("1.0.0", "2.0.0", true, true), "2.0.0", true},
		{"[1.0.0,2.0.0] excludes 0.9.0", NewInterval("1.0.0", "2.0.0", true, true), "0.9.0", false},
		{"[1.0.0,2.0.0] excludes 2.0.1", NewInterval("1.0.0", "2.0.0", true, true), "2.0.1", false},
		{"[1.0.0,2.0.0) excludes 2.0.0", NewInterval("1.0.0", "2.0.0", true, false), "2.0.0", false},
		{"(1.0.0,2.0.0] excludes 1.0.0", NewInterval("1.0.0", "2.0.0", false, true), "1.0.0", false},
		{"unbounded contains anything", UnboundedInterval(), "999.999.999", true},
		{"empty contains nothing", EmptyInterval(), "1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.i.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestIntervalIntersect(t *testing.T) {
	tests := []struct {
		name   string
		i1     Interval
		i2     Interval
		expect func(Interval) bool
	}{
		{
			"overlapping ranges",
			NewInterval("1.0.0", "3.0.0", true, true),
			NewInterval("2.0.0", "4.0.0", true, true),
			func(r Interval) bool {
				return r.Min == "2.0.0" && r.Max == "3.0.0" && r.MinInclusive && r.MaxInclusive
			},
		},
		{
			"one inside other",
			NewInterval("1.0.0", "5.0.0", true, true),
			NewInterval("2.0.0", "3.0.0", true, true),
			func(r Interval) bool {
				return r.Min == "2.0.0" && r.Max == "3.0.0"
			},
		},
		{
			"same boundary different inclusivity",
			NewInterval("1.0.0", "2.0.0", true, true),
			NewInterval("1.0.0", "2.0.0", false, false),
			func(r Interval) bool {
				return r.Min == "1.0.0" && r.Max == "2.0.0" && !r.MinInclusive && !r.MaxInclusive
			},
		},
		{
			"non-overlapping",
			NewInterval("1.0.0", "2.0.0", true, true),
			NewInterval("3.0.0", "4.0.0", true, true),
			func(r Interval) bool {
				return r.IsEmpty()
			},
		},
		{
			"unbounded with bounded",
			UnboundedInterval(),
			NewInterval("1.0.0", "2.0.0", true, false),
			func(r Interval) bool {
				return r.Min == "1.0.0" && r.Max == "2.0.0" && r.MinInclusive && !r.MaxInclusive
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.i1.Intersect(tt.i2)
			if !tt.expect(result) {
				t.Errorf("Intersect() = %v, didn't meet expectations", result)
			}
		})
	}
}

func TestIntervalOverlaps(t *testing.T) {
	tests := []struct {
		name string
		i1   Interval
		i2   Interval
		want bool
	}{
		{"overlapping", NewInterval("1.0.0", "3.0.0", true, true), NewInterval("2.0.0", "4.0.0", true, true), true},
		{"touching inclusive", NewInterval("1.0.0", "2.0.0", true, true), NewInterval("2.0.0", "3.0.0", true, true), true},
		{"touching exclusive", NewInterval("1.0.0", "2.0.0", true, false), NewInterval("2.0.0", "3.0.0", false, true), false},
		{"non-overlapping", NewInterval("1.0.0", "2.0.0", true, true), NewInterval("3.0.0", "4.0.0", true, true), false},
		{"empty", EmptyInterval(), NewInterval("1.0.0", "2.0.0", true, true), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.i1.Overlaps(tt.i2)
			if got != tt.want {
				t.Errorf("Overlaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntervalAdjacent(t *testing.T) {
	tests := []struct {
		name string
		i1   Interval
		i2   Interval
		want bool
	}{
		{"adjacent [,a] (a,]", NewInterval("1.0.0", "2.0.0", true, true), NewInterval("2.0.0", "3.0.0", false, true), true},
		{"adjacent [,a) [a,]", NewInterval("1.0.0", "2.0.0", true, false), NewInterval("2.0.0", "3.0.0", true, true), true},
		{"not adjacent [,a] [a,]", NewInterval("1.0.0", "2.0.0", true, true), NewInterval("2.0.0", "3.0.0", true, true), false},
		{"gap between", NewInterval("1.0.0", "2.0.0", true, true), NewInterval("3.0.0", "4.0.0", true, true), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.i1.Adjacent(tt.i2)
			if got != tt.want {
				t.Errorf("Adjacent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntervalUnion(t *testing.T) {
	tests := []struct {
		name   string
		i1     Interval
		i2     Interval
		isNil  bool
		expect func(*Interval) bool
	}{
		{
			"overlapping",
			NewInterval("1.0.0", "3.0.0", true, true),
			NewInterval("2.0.0", "4.0.0", true, true),
			false,
			func(r *Interval) bool {
				return r.Min == "1.0.0" && r.Max == "4.0.0" && r.MinInclusive && r.MaxInclusive
			},
		},
		{
			"adjacent",
			NewInterval("1.0.0", "2.0.0", true, false),
			NewInterval("2.0.0", "3.0.0", true, true),
			false,
			func(r *Interval) bool {
				return r.Min == "1.0.0" && r.Max == "3.0.0"
			},
		},
		{
			"non-overlapping non-adjacent",
			NewInterval("1.0.0", "2.0.0", true, true),
			NewInterval("3.0.0", "4.0.0", true, true),
			true,
			nil,
		},
		{
			"same boundary different inclusivity",
			NewInterval("1.0.0", "2.0.0", true, false),
			NewInterval("1.0.0", "2.0.0", false, true),
			false,
			func(r *Interval) bool {
				return r.MinInclusive && r.MaxInclusive
			},
		},
		{
			"unbounded min with bounded",
			LessThanInterval("2.0.0", true),
			NewInterval("1.0.0", "3.0.0", true, true),
			false,
			func(r *Interval) bool {
				return r.Min == "" && r.Max == "3.0.0" && r.MaxInclusive
			},
		},
		{
			"bounded with unbounded max",
			NewInterval("1.0.0", "3.0.0", true, true),
			GreaterThanInterval("2.0.0", true),
			false,
			func(r *Interval) bool {
				return r.Min == "1.0.0" && r.MinInclusive && r.Max == ""
			},
		},
		{
			"unbounded min with unbounded max",
			LessThanInterval("2.0.0", true),
			GreaterThanInterval("1.0.0", true),
			false,
			func(r *Interval) bool {
				return r.Min == "" && r.Max == ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.i1.Union(tt.i2)
			if tt.isNil {
				if result != nil {
					t.Errorf("Union() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Error("Union() = nil, want non-nil")
				} else if !tt.expect(result) {
					t.Errorf("Union() = %v, didn't meet expectations", result)
				}
			}
		})
	}
}

func TestIntervalString(t *testing.T) {
	tests := []struct {
		name string
		i    Interval
		want string
	}{
		{"empty", EmptyInterval(), "empty"},
		{"unbounded", UnboundedInterval(), "(-inf,+inf)"},
		{"[1.0.0,2.0.0]", NewInterval("1.0.0", "2.0.0", true, true), "[1.0.0,2.0.0]"},
		{"(1.0.0,2.0.0)", NewInterval("1.0.0", "2.0.0", false, false), "(1.0.0,2.0.0)"},
		{"[1.0.0,2.0.0)", NewInterval("1.0.0", "2.0.0", true, false), "[1.0.0,2.0.0)"},
		{">=1.0.0", GreaterThanInterval("1.0.0", true), "[1.0.0,+inf)"},
		{">1.0.0", GreaterThanInterval("1.0.0", false), "(1.0.0,+inf)"},
		{"<=2.0.0", LessThanInterval("2.0.0", true), "(-inf,2.0.0]"},
		{"<2.0.0", LessThanInterval("2.0.0", false), "(-inf,2.0.0)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.i.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
