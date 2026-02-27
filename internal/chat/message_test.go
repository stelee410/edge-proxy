package chat

import (
	"testing"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage(RoleUser, "Hello, world!")

	if msg.Role != RoleUser {
		t.Errorf("Expected role %s, got %s", RoleUser, msg.Role)
	}

	if msg.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", msg.Content)
	}

	if msg.ID == "" {
		t.Error("Expected non-empty ID")
	}

	if msg.IsSystem() {
		t.Error("Expected user message, not system message")
	}

	if !msg.IsUser() {
		t.Error("Expected user message")
	}

	if msg.IsAssistant() {
		t.Error("Expected user message, not assistant message")
	}
}

func TestMessageWithID(t *testing.T) {
	msg := NewMessage(RoleUser, "Test")
	msg.WithID("custom-id")

	if msg.ID != "custom-id" {
		t.Errorf("Expected ID 'custom-id', got '%s'", msg.ID)
	}
}

func TestMessageWithTokenCount(t *testing.T) {
	msg := NewMessage(RoleAssistant, "Response")
	msg.WithTokenCount(100)

	if msg.TokenCount != 100 {
		t.Errorf("Expected token count 100, got %d", msg.TokenCount)
	}
}

func TestMessageWithMetadata(t *testing.T) {
	msg := NewMessage(RoleUser, "Test")
	msg.WithMetadata("key1", "value1")
	msg.WithMetadata("key2", 42)

	val, ok := msg.GetMetadata("key1")
	if !ok {
		t.Error("Expected metadata key1 to exist")
	}
	if val != "value1" {
		t.Errorf("Expected value 'value1', got '%v'", val)
	}

	val, ok = msg.GetMetadata("key2")
	if !ok {
		t.Error("Expected metadata key2 to exist")
	}
	if val != 42 {
		t.Errorf("Expected value 42, got %v", val)
	}
}

func TestMessageClone(t *testing.T) {
	original := NewMessage(RoleUser, "Original")
	original.WithTokenCount(50)
	original.WithMetadata("key", "value")

	cloned := original.Clone()

	if cloned.ID != original.ID {
		t.Error("Cloned message should have same ID")
	}

	if cloned.Content != original.Content {
		t.Error("Cloned message should have same content")
	}

	if cloned.TokenCount != original.TokenCount {
		t.Error("Cloned message should have same token count")
	}

	// Modify original
	original.Content = "Modified"

	if cloned.Content == "Modified" {
		t.Error("Cloned message should not be affected by original modification")
	}
}

func TestRoleTypes(t *testing.T) {
	tests := []struct {
		role      Role
		isUser    bool
		isAssistant bool
		isSystem  bool
	}{
		{RoleUser, true, false, false},
		{RoleAssistant, false, true, false},
		{RoleSystem, false, false, true},
	}

	for _, tt := range tests {
		msg := NewMessage(tt.role, "test")
		if msg.IsUser() != tt.isUser {
			t.Errorf("Role %s: IsUser() should be %v", tt.role, tt.isUser)
		}
		if msg.IsAssistant() != tt.isAssistant {
			t.Errorf("Role %s: IsAssistant() should be %v", tt.role, tt.isAssistant)
		}
		if msg.IsSystem() != tt.isSystem {
			t.Errorf("Role %s: IsSystem() should be %v", tt.role, tt.isSystem)
		}
	}
}
