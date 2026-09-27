package ingest

import (
	"net"
	"strings"
	"testing"
)

func TestValidateFileFailClosed(t *testing.T) {
	if kind, err := ValidateFile("example.txt", []byte("Синтетический текст")); err != nil || kind != "text/plain" {
		t.Fatalf("valid TXT rejected: %v %q", err, kind)
	}
	if kind, err := ValidateFile("example.pdf", []byte("%PDF-1.7\n1 0 obj\n%%EOF")); err != nil || kind != "application/pdf" {
		t.Fatalf("PDF signature rejected: %v %q", err, kind)
	}
	bad := []struct {
		name string
		data []byte
	}{
		{"../secret.txt", []byte("hello")},
		{"script.txt", []byte("<script>alert(1)</script>")},
		{"fake.pdf", []byte("not a pdf")},
		{"large.txt", []byte(strings.Repeat("a", (256<<10)+1))},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateFile(tc.name, tc.data); err == nil {
				t.Fatal("unsafe file admitted")
			}
		})
	}
}

func TestValidateURLRejectsPrivateAndUnsafeSchemes(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://127.0.0.1/path", "https://10.0.0.1/path", "https://169.254.169.254/latest", "https://user:pass@example.com", "https://example.local/path", "https://example.com:8443/path"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ValidateURLSyntax(raw); err == nil {
				t.Fatal("unsafe URL admitted")
			}
		})
	}
}

func TestEveryRedirectAndResolvedAddressMustBePublic(t *testing.T) {
	for _, hop := range []string{"https://127.0.0.1/redirect", "http://example.org/redirect", "https://169.254.169.254/metadata"} {
		if _, err := ValidateURLSyntax(hop); err == nil {
			t.Fatalf("unsafe redirect admitted: %s", hop)
		}
	}
	if err := ValidateAddresses([]net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("10.0.0.1")}); err == nil {
		t.Fatal("mixed public/private DNS answer admitted")
	}
	if err := ValidateAddresses([]net.IP{net.ParseIP("8.8.8.8")}); err != nil {
		t.Fatalf("public address rejected: %v", err)
	}
}

func TestSyntheticFixturesAreBounded(t *testing.T) {
	a, err := SyntheticText("sample-v1")
	if err != nil || a == "" {
		t.Fatal("missing fixture")
	}
	b, err := SyntheticText("sample-v2")
	if err != nil || b == a {
		t.Fatal("changed fixture missing")
	}
	if _, err := SyntheticText("arbitrary"); err == nil {
		t.Fatal("arbitrary fixture accepted")
	}
}
