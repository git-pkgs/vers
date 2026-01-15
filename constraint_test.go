package vers

import "testing"

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		input    string
		operator string
		version  string
		wantErr  bool
	}{
		{">=1.0.0", ">=", "1.0.0", false},
		{"<=2.0.0", "<=", "2.0.0", false},
		{">1.0.0", ">", "1.0.0", false},
		{"<2.0.0", "<", "2.0.0", false},
		{"=1.0.0", "=", "1.0.0", false},
		{"!=1.5.0", "!=", "1.5.0", false},
		{"1.0.0", "=", "1.0.0", false}, // No operator defaults to =
		{"", "", "", true},
		{">=", "", "", true}, // Missing version
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c, err := ParseConstraint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConstraint(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if c.Operator != tt.operator {
				t.Errorf("Operator = %q, want %q", c.Operator, tt.operator)
			}
			if c.Version != tt.version {
				t.Errorf("Version = %q, want %q", c.Version, tt.version)
			}
		})
	}
}

func TestConstraintSatisfies(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		want       bool
	}{
		{">=1.0.0", "1.0.0", true},
		{">=1.0.0", "1.5.0", true},
		{">=1.0.0", "0.9.0", false},
		{">1.0.0", "1.0.0", false},
		{">1.0.0", "1.0.1", true},
		{"<=2.0.0", "2.0.0", true},
		{"<=2.0.0", "1.5.0", true},
		{"<=2.0.0", "2.0.1", false},
		{"<2.0.0", "2.0.0", false},
		{"<2.0.0", "1.9.9", true},
		{"=1.0.0", "1.0.0", true},
		{"=1.0.0", "1.0.1", false},
		{"!=1.5.0", "1.5.0", false},
		{"!=1.5.0", "1.4.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.constraint+"_"+tt.version, func(t *testing.T) {
			c, err := ParseConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error = %v", tt.constraint, err)
			}
			got := c.Satisfies(tt.version)
			if got != tt.want {
				t.Errorf("Satisfies(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestConstraintString(t *testing.T) {
	c, _ := ParseConstraint(">=1.0.0")
	if c.String() != ">=1.0.0" {
		t.Errorf("String() = %q, want %q", c.String(), ">=1.0.0")
	}
}
