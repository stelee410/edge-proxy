package docparse

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractDocumentText extracts text from document bytes based on file extension.
// Supports: .pdf, .docx, .doc, .txt
func ExtractDocumentText(data []byte, filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return extractPDF(data)
	case ".docx", ".doc":
		return extractDocx(data)
	case ".txt":
		return string(data), nil
	default:
		return "", fmt.Errorf("unsupported document type: %s", ext)
	}
}

func extractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	var sb strings.Builder
	numPages := r.NumPage()
	for i := 1; i <= numPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		if i < numPages {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

func extractDocx(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	var docXML *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docXML = f
			break
		}
	}
	if docXML == nil {
		return "", fmt.Errorf("word/document.xml not found in docx")
	}
	rc, err := docXML.Open()
	if err != nil {
		return "", fmt.Errorf("open document.xml: %w", err)
	}
	defer rc.Close()
	xmlData, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("read document.xml: %w", err)
	}
	re := regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
	matches := re.FindAllStringSubmatch(string(xmlData), -1)
	var sb strings.Builder
	for _, m := range matches {
		if len(m) >= 2 {
			sb.WriteString(html.UnescapeString(m[1]))
		}
	}
	return sb.String(), nil
}
