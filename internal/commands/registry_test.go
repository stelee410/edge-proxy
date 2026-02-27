package commands

import (
	"testing"
)

// mockCommand 模拟命令用于测试
type mockCommand struct {
	name        string
	description string
	category    string
	aliases     []string
	executeFn   func(ctx *Context, args []string) (string, error)
}

func (m *mockCommand) Name() string              { return m.name }
func (m *mockCommand) Description() string       { return m.description }
func (m *mockCommand) Usage() string           { return "/" + m.name }
func (m *mockCommand) Aliases() []string        { return m.aliases }
func (m *mockCommand) Category() string         { return m.category }
func (m *mockCommand) Execute(ctx *Context, args []string) (string, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, args)
	}
	return "executed", nil
}
func (m *mockCommand) Validate(args []string) error { return nil }

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	// Check default prefix
	if registry.prefix != "/" {
		t.Errorf("Expected prefix '/', got '%s'", registry.prefix)
	}
}

func TestRegistryRegister(t *testing.T) {
	registry := NewRegistry()

	cmd := &mockCommand{
		name:        "test",
		description: "Test command",
		category:    "Test",
	}

	err := registry.Register(cmd)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Check command is registered
	retrieved, ok := registry.Get("test")
	if !ok {
		t.Error("Expected command to be registered")
	}
	if retrieved.Name() != "test" {
		t.Errorf("Retrieved command name = %s, want test", retrieved.Name())
	}
}

func TestRegistryWithAlias(t *testing.T) {
	registry := NewRegistry()

	cmd := &mockCommand{
		name:        "test",
		description: "Test command",
		aliases:     []string{"t", "testing"},
		category:    "Test",
	}

	err := registry.Register(cmd)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Check command is accessible via alias
	_, ok := registry.Get("t")
	if !ok {
		t.Error("Expected command to be accessible via alias 't'")
	}

	_, ok = registry.Get("testing")
	if !ok {
		t.Error("Expected command to be accessible via alias 'testing'")
	}
}

func TestRegistryUnregister(t *testing.T) {
	registry := NewRegistry()

	cmd := &mockCommand{
		name:        "test",
		description: "Test command",
		category:    "Test",
	}

	registry.Register(cmd)

	err := registry.Unregister("test")
	if err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	_, ok := registry.Get("test")
	if ok {
		t.Error("Expected command to be unregistered")
	}
}

func TestRegistryList(t *testing.T) {
	registry := NewRegistry()

	cmd1 := &mockCommand{name: "test1", description: "Test 1", category: "Test"}
	cmd2 := &mockCommand{name: "test2", description: "Test 2", category: "Test"}
	cmd3 := &mockCommand{name: "other", description: "Other", category: "Other"}

	registry.Register(cmd1)
	registry.Register(cmd2)
	registry.Register(cmd3)

	commands := registry.List()

	if len(commands) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(commands))
	}
}

func TestRegistryListByCategory(t *testing.T) {
	registry := NewRegistry()

	cmd1 := &mockCommand{name: "test1", description: "Test 1", category: "Test"}
	cmd2 := &mockCommand{name: "test2", description: "Test 2", category: "Test"}
	cmd3 := &mockCommand{name: "other", description: "Other", category: "Other"}

	registry.Register(cmd1)
	registry.Register(cmd2)
	registry.Register(cmd3)

	testCommands := registry.ListByCategory("Test")

	if len(testCommands) != 2 {
		t.Errorf("Expected 2 test commands, got %d", len(testCommands))
	}

	otherCommands := registry.ListByCategory("Other")

	if len(otherCommands) != 1 {
		t.Errorf("Expected 1 other command, got %d", len(otherCommands))
	}
}

func TestRegistryExecute(t *testing.T) {
	registry := NewRegistry()

	executed := false
	cmd := &mockCommand{
		name:        "test",
		description: "Test command",
		category:    "Test",
		executeFn: func(ctx *Context, args []string) (string, error) {
			executed = true
			return "test executed", nil
		},
	}

	registry.Register(cmd)

	output, err := registry.Execute(&Context{}, "/test arg1")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !executed {
		t.Error("Expected command to be executed")
	}

	if output != "test executed" {
		t.Errorf("Expected 'test executed', got '%s'", output)
	}
}

func TestRegistrySearch(t *testing.T) {
	registry := NewRegistry()

	cmd1 := &mockCommand{name: "test-command", description: "Test", category: "Test"}
	cmd2 := &mockCommand{name: "test-another", description: "Test", category: "Test"}

	registry.Register(cmd1)
	registry.Register(cmd2)

	// Fuzzy match
	results := registry.Search("test")

	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'test', got %d", len(results))
	}
}

func TestRegistryIsCommand(t *testing.T) {
	registry := NewRegistry()

	// Test with default prefix "/"
	if !registry.IsCommand("/test") {
		t.Error("Expected '/test' to be recognized as command")
	}

	if registry.IsCommand("test") {
		t.Error("Expected 'test' (without prefix) to not be recognized as command")
	}
}

func TestRegistryGetHelp(t *testing.T) {
	registry := NewRegistry()

	cmd1 := &mockCommand{name: "test1", description: "Test 1", category: "Test"}
	cmd2 := &mockCommand{name: "test2", description: "Test 2", category: "Test"}

	registry.Register(cmd1)
	registry.Register(cmd2)

	help := registry.GetHelp("Test")

	if help == "" {
		t.Error("Expected non-empty help text")
	}

	// Check for category name
	// Note: This is a simple check
}
