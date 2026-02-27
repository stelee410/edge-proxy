package commands

import (
	"testing"
)

func TestParseSimple(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPos    []string
		wantFlags  map[string]interface{}
	}{
		{
			name:    "simple args",
			args:    []string{"arg1", "arg2", "arg3"},
			wantPos: []string{"arg1", "arg2", "arg3"},
			wantFlags: map[string]interface{}{},
		},
		{
			name:    "with bool flag",
			args:    []string{"arg1", "--verbose"},
			wantPos: []string{"arg1"},
			wantFlags: map[string]interface{}{"verbose": true},
		},
		{
			name:    "with string flag",
			args:    []string{"arg1", "--output", "file.txt"},
			wantPos: []string{"arg1"},
			wantFlags: map[string]interface{}{"output": "file.txt"},
		},
		{
			name:    "with int flag",
			args:    []string{"--count", "42", "arg1"},
			wantPos: []string{"arg1"},
			wantFlags: map[string]interface{}{"count": 42},
		},
		{
			name:    "with float flag",
			args:    []string{"--rate", "3.14"},
			wantPos: []string{},
			wantFlags: map[string]interface{}{"rate": 3.14},
		},
		{
			name:    "flag with equals",
			args:    []string{"--file=test.txt"},
			wantPos: []string{},
			wantFlags: map[string]interface{}{"file": "test.txt"},
		},
		{
			name:    "short flag",
			args:    []string{"-v", "arg1"},
			wantPos: []string{"arg1"},
			wantFlags: map[string]interface{}{"v": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSimple(tt.args)
			if err != nil {
				t.Fatalf("ParseSimple() error = %v", err)
			}

			if len(result.Positional) != len(tt.wantPos) {
				t.Errorf("Positional length mismatch: got %d, want %d",
					len(result.Positional), len(tt.wantPos))
			}

			for i, want := range tt.wantPos {
				if result.Positional[i] != want {
					t.Errorf("Positional[%d] = %s, want %s", i, result.Positional[i], want)
				}
			}

			for key, wantVal := range tt.wantFlags {
				gotVal, ok := result.Flags[key]
				if !ok {
					t.Errorf("Flag %s not found", key)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("Flag %s = %v, want %v", key, gotVal, wantVal)
				}
			}
		})
	}
}

func TestParser(t *testing.T) {
	parser := NewParser()

	parser.AddFlag(&FlagSpec{
		Name:        "verbose",
		Short:       "v",
		Description:  "Verbose output",
		Type:        "bool",
		DefaultValue: false,
	})

	parser.AddFlag(&FlagSpec{
		Name:        "output",
		Short:       "o",
		Description:  "Output file",
		Type:        "string",
		DefaultValue: "",
	})

	parser.AddFlag(&FlagSpec{
		Name:        "count",
		Description:  "Number of items",
		Type:        "int",
		DefaultValue: 1,
	})

	parser.AddArg(&ArgSpec{
		Name:        "input",
		Description: "Input file",
		Required:    false,
	})

	parser.AddArg(&ArgSpec{
		Name:        "output",
		Description: "Output file",
		Required:    false,
	})

	t.Run("parse with flags", func(t *testing.T) {
		result, err := parser.Parse([]string{"input.txt", "--verbose", "-o", "out.txt", "--count", "5"})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		if len(result.Positional) != 2 {
			t.Errorf("Expected 2 positional args, got %d", len(result.Positional))
		}

		if result.Positional[0] != "input.txt" {
			t.Errorf("Expected input.txt, got %s", result.Positional[0])
		}

		if result.GetString("output", "") != "out.txt" {
			t.Errorf("Expected output out.txt, got %s", result.GetString("output", ""))
		}

		if result.GetBool("verbose", false) != true {
			t.Error("Expected verbose to be true")
		}

		if result.GetInt("count", 0) != 5 {
			t.Errorf("Expected count 5, got %d", result.GetInt("count", 0))
		}
	})

	t.Run("parse with required flag", func(t *testing.T) {
		parser2 := NewParser()
		parser2.AddFlag(&FlagSpec{
			Name:     "required",
			Type:     "string",
			Required: true,
		})

		_, err := parser2.Parse([]string{})
		if err == nil {
			t.Error("Expected error for missing required flag")
		}
	})

	t.Run("help generation", func(t *testing.T) {
		help := parser.Help("testcmd")
		if help == "" {
			t.Error("Expected non-empty help text")
		}

		// Check for expected content
		// Note: This is a simple check, actual help format may vary
	})
}

func TestParseResultAccessors(t *testing.T) {
	result := &ParseResult{
		Positional: []string{"arg1", "arg2", "arg3"},
		Flags: map[string]interface{}{
			"string": "value",
			"int":    42,
			"float":  3.14,
			"bool":   true,
		},
	}

	t.Run("GetString", func(t *testing.T) {
		if result.GetString("string", "default") != "value" {
			t.Error("GetString failed")
		}
		if result.GetString("missing", "default") != "default" {
			t.Error("GetString with default failed")
		}
	})

	t.Run("GetInt", func(t *testing.T) {
		if result.GetInt("int", 0) != 42 {
			t.Error("GetInt failed")
		}
		if result.GetInt("missing", 10) != 10 {
			t.Error("GetInt with default failed")
		}
	})

	t.Run("GetFloat", func(t *testing.T) {
		if result.GetFloat("float", 0) != 3.14 {
			t.Error("GetFloat failed")
		}
	})

	t.Run("GetBool", func(t *testing.T) {
		if result.GetBool("bool", false) != true {
			t.Error("GetBool failed")
		}
	})

	t.Run("GetPositional", func(t *testing.T) {
		if result.GetPositional(0, "") != "arg1" {
			t.Error("GetPositional failed")
		}
		if result.GetPositional(10, "default") != "default" {
			t.Error("GetPositional with default failed")
		}
	})
}
