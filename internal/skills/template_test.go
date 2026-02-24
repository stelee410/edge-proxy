package skills

import (
	"strings"
	"testing"
)

func TestRenderTemplateBasic(t *testing.T) {
	tmpl := "Hello, {{.name}}!"
	data := map[string]interface{}{"name": "World"}

	result, err := RenderTemplate(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", result)
	}
}

func TestRenderTemplateWithConditional(t *testing.T) {
	tmpl := `你是一个{{.role}}专家。{{if .language}}请使用{{.language}}回答。{{end}}`

	// With language
	data := map[string]interface{}{"role": "Go编程", "language": "中文"}
	result, err := RenderTemplate(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Go编程专家") {
		t.Errorf("expected to contain 'Go编程专家', got %q", result)
	}
	if !strings.Contains(result, "请使用中文回答") {
		t.Errorf("expected to contain '请使用中文回答', got %q", result)
	}

	// Without language
	data2 := map[string]interface{}{"role": "Python编程"}
	result2, err := RenderTemplate(tmpl, data2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result2, "请使用") {
		t.Errorf("should not contain language part, got %q", result2)
	}
}

func TestRenderTemplateEmpty(t *testing.T) {
	result, err := RenderTemplate("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestRenderTemplateBuiltinVars(t *testing.T) {
	tmpl := "Date: {{._date}}"
	result, err := RenderTemplate(tmpl, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" || result == "Date: " {
		t.Errorf("expected date to be injected, got %q", result)
	}
}

func TestRenderTemplateInvalidTemplate(t *testing.T) {
	_, err := RenderTemplate("{{.name", nil)
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestRenderTemplateFunctions(t *testing.T) {
	tmpl := `{{upper .text}}`
	result, err := RenderTemplate(tmpl, map[string]interface{}{"text": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "HELLO" {
		t.Errorf("expected 'HELLO', got %q", result)
	}
}
