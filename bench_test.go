package vers

import "testing"

// Parsing benchmarks

func BenchmarkParse_VersURI_Simple(b *testing.B) {
	for b.Loop() {
		_, _ = Parse("vers:npm/>=1.2.3")
	}
}

func BenchmarkParse_VersURI_Complex(b *testing.B) {
	for b.Loop() {
		_, _ = Parse("vers:npm/>=1.2.3|<2.0.0|!=1.5.0")
	}
}

func BenchmarkParseNative_Npm_Caret(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("^1.2.3", "npm")
	}
}

func BenchmarkParseNative_Npm_Tilde(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("~1.2.3", "npm")
	}
}

func BenchmarkParseNative_Npm_Range(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative(">=1.0.0 <2.0.0", "npm")
	}
}

func BenchmarkParseNative_Npm_Or(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("^1.0.0 || ^2.0.0 || ^3.0.0", "npm")
	}
}

func BenchmarkParseNative_Gem_Pessimistic(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("~> 1.2.3", "gem")
	}
}

func BenchmarkParseNative_Pypi_Compatible(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("~=1.4.2", "pypi")
	}
}

func BenchmarkParseNative_Maven_Bracket(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("[1.0,2.0)", "maven")
	}
}

// Contains benchmarks

func BenchmarkContains_Simple(b *testing.B) {
	r, _ := ParseNative("^1.2.3", "npm")
	b.ResetTimer()
	for b.Loop() {
		r.Contains("1.5.0")
	}
}

func BenchmarkContains_MultiInterval(b *testing.B) {
	r, _ := ParseNative("^1.0.0 || ^2.0.0 || ^3.0.0", "npm")
	b.ResetTimer()
	for b.Loop() {
		r.Contains("2.5.0")
	}
}

func BenchmarkContains_WithExclusions(b *testing.B) {
	r, _ := Parse("vers:npm/>=1.0.0|<2.0.0|!=1.5.0|!=1.6.0|!=1.7.0")
	b.ResetTimer()
	for b.Loop() {
		r.Contains("1.8.0")
	}
}

func BenchmarkContains_Prerelease(b *testing.B) {
	r, _ := ParseNative(">=1.0.0-alpha.1", "npm")
	b.ResetTimer()
	for b.Loop() {
		r.Contains("1.0.0-beta.2")
	}
}

func BenchmarkCompare_Simple(b *testing.B) {
	for b.Loop() {
		Compare("1.2.3", "1.2.4")
	}
}

func BenchmarkCompare_Prerelease(b *testing.B) {
	for b.Loop() {
		Compare("1.0.0-alpha.1", "1.0.0-beta.2")
	}
}

// Range operation benchmarks

func BenchmarkUnion_TwoRanges(b *testing.B) {
	r1, _ := ParseNative("^1.0.0", "npm")
	r2, _ := ParseNative("^2.0.0", "npm")
	b.ResetTimer()
	for b.Loop() {
		r1.Union(r2)
	}
}

func BenchmarkUnion_ManyRanges(b *testing.B) {
	ranges := make([]*Range, 10)
	for i := range ranges {
		ranges[i], _ = ParseNative("^1.0.0", "npm")
	}
	b.ResetTimer()
	for b.Loop() {
		result := ranges[0]
		for _, r := range ranges[1:] {
			result = result.Union(r)
		}
	}
}

func BenchmarkIntersect_TwoRanges(b *testing.B) {
	r1, _ := ParseNative(">=1.0.0", "npm")
	r2, _ := ParseNative("<2.0.0", "npm")
	b.ResetTimer()
	for b.Loop() {
		r1.Intersect(r2)
	}
}

func BenchmarkIntersect_ManyRanges(b *testing.B) {
	r1, _ := ParseNative(">=1.0.0", "npm")
	r2, _ := ParseNative("<3.0.0", "npm")
	r3, _ := ParseNative(">=1.5.0", "npm")
	r4, _ := ParseNative("<2.5.0", "npm")
	b.ResetTimer()
	for b.Loop() {
		r1.Intersect(r2).Intersect(r3).Intersect(r4)
	}
}

// Satisfies benchmarks (combines parsing and contains)

func BenchmarkSatisfies_VersURI(b *testing.B) {
	for b.Loop() {
		_, _ = Satisfies("1.5.0", "vers:npm/>=1.0.0|<2.0.0", "")
	}
}

func BenchmarkSatisfies_Native(b *testing.B) {
	for b.Loop() {
		_, _ = Satisfies("1.5.0", "^1.2.3", "npm")
	}
}
