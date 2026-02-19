package vers

import "testing"

func TestParseVersURI(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
		wantErr bool
	}{
		// Basic VERS URI
		{"exact version", "vers:npm/=1.0.0", "1.0.0", true, false},
		{"exact version excludes other", "vers:npm/=1.0.0", "1.0.1", false, false},
		{"greater than", "vers:npm/>=1.0.0", "1.5.0", true, false},
		{"less than", "vers:npm/<2.0.0", "1.9.9", true, false},
		{"combined constraints", "vers:npm/>=1.0.0|<2.0.0", "1.5.0", true, false},
		{"combined excludes below", "vers:npm/>=1.0.0|<2.0.0", "0.9.0", false, false},
		{"combined excludes above", "vers:npm/>=1.0.0|<2.0.0", "2.0.0", false, false},
		{"exclusion", "vers:npm/>=1.0.0|!=1.5.0", "1.5.0", false, false},
		{"exclusion allows other", "vers:npm/>=1.0.0|!=1.5.0", "1.6.0", true, false},

		// Wildcard
		{"wildcard matches all", "vers:npm/*", "999.0.0", true, false},
		{"empty constraints", "vers:npm/", "999.0.0", true, false},

		// Different schemes
		{"gem scheme", "vers:gem/>=1.0.0", "1.5.0", true, false},
		{"pypi scheme", "vers:pypi/>=1.0.0", "1.5.0", true, false},
		{"maven scheme", "vers:maven/>=1.0.0", "1.5.0", true, false},

		// Error cases
		{"invalid format", "invalid", "", false, true},
		{"missing scheme", "vers:/>=1.0.0", "", false, true},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseNpmRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		// Caret ranges
		{"^1.2.3 includes exact", "^1.2.3", "1.2.3", true},
		{"^1.2.3 includes patch", "^1.2.3", "1.2.4", true},
		{"^1.2.3 includes minor", "^1.2.3", "1.9.0", true},
		{"^1.2.3 excludes major", "^1.2.3", "2.0.0", false},
		{"^0.2.3 includes patch", "^0.2.3", "0.2.5", true},
		{"^0.2.3 excludes minor", "^0.2.3", "0.3.0", false},
		{"^0.0.3 excludes patch", "^0.0.3", "0.0.4", false},

		// Tilde ranges
		{"~1.2.3 includes patch", "~1.2.3", "1.2.5", true},
		{"~1.2.3 excludes minor", "~1.2.3", "1.3.0", false},
		{"~1.2.0 includes patch", "~1.2.0", "1.2.9", true},
		{"~1.2.0 excludes minor", "~1.2.0", "1.3.0", false},
		{"~1.0.0 includes patch", "~1.0.0", "1.0.9", true},
		{"~1.0.0 excludes minor", "~1.0.0", "1.1.0", false},
		{"~1.0 includes patch", "~1.0", "1.0.9", true},
		{"~1.0 excludes minor", "~1.0", "1.1.0", false},
		{"~1 includes minor", "~1", "1.9.0", true},
		{"~1 excludes major", "~1", "2.0.0", false},

		// X-ranges
		{"1.x includes 1.0.0", "1.x", "1.0.0", true},
		{"1.x includes 1.9.9", "1.x", "1.9.9", true},
		{"1.x excludes 2.0.0", "1.x", "2.0.0", false},
		{"1.2.x includes 1.2.0", "1.2.x", "1.2.0", true},
		{"1.2.x excludes 1.3.0", "1.2.x", "1.3.0", false},

		// Hyphen ranges
		{"1.0.0 - 2.0.0 includes min", "1.0.0 - 2.0.0", "1.0.0", true},
		{"1.0.0 - 2.0.0 includes max", "1.0.0 - 2.0.0", "2.0.0", true},
		{"1.0.0 - 2.0.0 includes middle", "1.0.0 - 2.0.0", "1.5.0", true},
		{"1.0.0 - 2.0.0 excludes below", "1.0.0 - 2.0.0", "0.9.0", false},

		// OR ranges
		{"|| includes first", "1.0.0 || 2.0.0", "1.0.0", true},
		{"|| includes second", "1.0.0 || 2.0.0", "2.0.0", true},
		{"|| excludes other", "1.0.0 || 2.0.0", "1.5.0", false},

		// AND ranges (space-separated)
		{"AND satisfies both", ">=1.0.0 <2.0.0", "1.5.0", true},
		{"AND fails first", ">=1.0.0 <2.0.0", "0.9.0", false},
		{"AND fails second", ">=1.0.0 <2.0.0", "2.0.0", false},

		// Wildcards
		{"* matches all", "*", "999.0.0", true},
		{"x matches all", "x", "999.0.0", true},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "npm")
			if err != nil {
				t.Fatalf("ParseNative(%q, npm) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseGemRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		// Pessimistic operator
		{"~> 1.2.3 includes patch", "~> 1.2.3", "1.2.5", true},
		{"~> 1.2.3 excludes minor", "~> 1.2.3", "1.3.0", false},
		{"~> 1.2 includes minor", "~> 1.2", "1.5.0", true},
		{"~> 1.2 excludes major", "~> 1.2", "2.0.0", false},

		// Standard constraints
		{">= 1.0.0 includes", ">= 1.0.0", "1.5.0", true},
		{">= 1.0.0 excludes below", ">= 1.0.0", "0.9.0", false},
		{"< 2.0.0 includes", "< 2.0.0", "1.9.9", true},
		{"< 2.0.0 excludes", "< 2.0.0", "2.0.0", false},

		// Comma-separated (AND)
		{">= 1.0.0, < 2.0.0 includes", ">= 1.0.0, < 2.0.0", "1.5.0", true},
		{">= 1.0.0, < 2.0.0 excludes below", ">= 1.0.0, < 2.0.0", "0.9.0", false},
		{">= 1.0.0, < 2.0.0 excludes above", ">= 1.0.0, < 2.0.0", "2.0.0", false},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "gem")
			if err != nil {
				t.Fatalf("ParseNative(%q, gem) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParsePypiRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		// Compatible release
		{"~=1.4.2 includes patch", "~=1.4.2", "1.4.5", true},
		{"~=1.4.2 excludes minor", "~=1.4.2", "1.5.0", false},

		// Standard constraints
		{">=1.0.0 includes", ">=1.0.0", "1.5.0", true},
		{"<2.0.0 includes", "<2.0.0", "1.9.9", true},
		{"!=1.5.0 excludes", "!=1.5.0", "1.5.0", false},
		{"!=1.5.0 includes other", "!=1.5.0", "1.4.0", true},

		// Comma-separated
		{">=1.0.0,<2.0.0 includes", ">=1.0.0,<2.0.0", "1.5.0", true},
		{">=1.0.0,<2.0.0 excludes below", ">=1.0.0,<2.0.0", "0.9.0", false},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "pypi")
			if err != nil {
				t.Fatalf("ParseNative(%q, pypi) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseMavenRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		// Bracket notation
		{"[1.0,2.0] includes min", "[1.0,2.0]", "1.0", true},
		{"[1.0,2.0] includes max", "[1.0,2.0]", "2.0", true},
		{"[1.0,2.0] includes middle", "[1.0,2.0]", "1.5", true},
		{"[1.0,2.0) excludes max", "[1.0,2.0)", "2.0", false},
		{"(1.0,2.0] excludes min", "(1.0,2.0]", "1.0", false},

		// Open-ended
		{"[1.0,) includes above", "[1.0,)", "5.0.0", true},
		{"[1.0,) excludes below", "[1.0,)", "0.9.0", false},
		{"(,2.0] includes below", "(,2.0]", "1.0.0", true},
		{"(,2.0] excludes above", "(,2.0]", "2.0.1", false},

		// Exact version
		{"[1.0] exact match", "[1.0]", "1.0", true},
		{"[1.0] excludes other", "[1.0]", "1.1", false},

		// Simple version (minimum)
		{"1.0 includes minimum", "1.0", "1.0", true},
		{"1.0 includes above", "1.0", "2.0.0", true},
		{"1.0 excludes below", "1.0", "0.9.0", false},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "maven")
			if err != nil {
				t.Fatalf("ParseNative(%q, maven) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseNugetRange(t *testing.T) {
	// NuGet uses the same syntax as Maven
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		{"[1.0,2.0] includes middle", "[1.0,2.0]", "1.5", true},
		{"[1.0,2.0) excludes max", "[1.0,2.0)", "2.0", false},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "nuget")
			if err != nil {
				t.Fatalf("ParseNative(%q, nuget) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseCargoRange(t *testing.T) {
	// Cargo uses npm-like syntax
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		{"^1.2.3 includes minor", "^1.2.3", "1.9.0", true},
		{"^1.2.3 excludes major", "^1.2.3", "2.0.0", false},
		{"~1.2.3 includes patch", "~1.2.3", "1.2.9", true},
		{"~1.2.3 excludes minor", "~1.2.3", "1.3.0", false},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "cargo")
			if err != nil {
				t.Fatalf("ParseNative(%q, cargo) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseGoRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		{">=1.0.0 includes", ">=1.0.0", "1.5.0", true},
		{"<2.0.0 includes", "<2.0.0", "1.9.9", true},
		{">=1.0.0,<2.0.0 includes", ">=1.0.0,<2.0.0", "1.5.0", true},
		{">=1.0.0,<2.0.0 excludes below", ">=1.0.0,<2.0.0", "0.9.0", false},
		{">=1.0.0,<2.0.0 excludes above", ">=1.0.0,<2.0.0", "2.0.0", false},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "go")
			if err != nil {
				t.Fatalf("ParseNative(%q, go) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseDebianRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		{">= 1.0 includes", ">= 1.0", "1.5.0", true},
		{">> 1.0 includes", ">> 1.0", "1.5.0", true},
		{">> 1.0 excludes exact", ">> 1.0", "1.0", false},
		{"<< 2.0 includes", "<< 2.0", "1.9.9", true},
		{"<< 2.0 excludes exact", "<< 2.0", "2.0", false},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "deb")
			if err != nil {
				t.Fatalf("ParseNative(%q, deb) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseRpmRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    bool
	}{
		{">= 1.0 includes", ">= 1.0", "1.5.0", true},
		{"<= 2.0 includes", "<= 2.0", "1.9.9", true},
		{"<= 2.0 includes exact", "<= 2.0", "2.0", true},
		{"< 2.0 excludes exact", "< 2.0", "2.0", false},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parser.ParseNative(tt.input, "rpm")
			if err != nil {
				t.Fatalf("ParseNative(%q, rpm) error = %v", tt.input, err)
			}
			got := r.Contains(tt.version)
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestToVersString(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name   string
		r      *Range
		scheme string
		want   string
	}{
		{"unbounded", Unbounded(), "npm", "vers:npm/*"},
		{"empty", Empty(), "npm", "vers:npm/"},
		{"exact", Exact("1.0.0"), "npm", "vers:npm/1.0.0"},
		{"greater than", GreaterThan("1.0.0", true), "npm", "vers:npm/>=1.0.0"},
		{"less than", LessThan("2.0.0", false), "npm", "vers:npm/<2.0.0"},
		{
			"range",
			NewRange([]Interval{NewInterval("1.0.0", "2.0.0", true, false)}),
			"npm",
			"vers:npm/>=1.0.0|<2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ToVersString(tt.r, tt.scheme)
			if got != tt.want {
				t.Errorf("ToVersString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPublicAPISatisfies(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		constraint string
		scheme     string
		want       bool
	}{
		{"vers URI", "1.5.0", "vers:npm/>=1.0.0", "", true},
		{"npm native", "1.5.0", "^1.0.0", "npm", true},
		{"gem native", "1.5.0", "~> 1.0", "gem", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Satisfies(tt.version, tt.constraint, tt.scheme)
			if err != nil {
				t.Fatalf("Satisfies() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Satisfies() = %v, want %v", got, tt.want)
			}
		})
	}
}
