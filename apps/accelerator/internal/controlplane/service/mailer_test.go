package service

import (
	"strings"
	"testing"
)

func TestBuildSMTPMessage_Plain(t *testing.T) {
	msg := string(buildSMTPMessage("Oxella", "noreply@oxella.com", "user@example.com", "Hello", "plain body"))
	if !strings.Contains(msg, "From: Oxella <noreply@oxella.com>") {
		t.Fatalf("from header: %s", msg)
	}
	if !strings.Contains(msg, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("content-type: %s", msg)
	}
	if !strings.Contains(msg, "plain body") {
		t.Fatalf("body missing")
	}
	if !strings.Contains(msg, "\r\n") {
		t.Fatal("expected CRLF")
	}
}

func TestBuildSMTPMessage_HTML(t *testing.T) {
	body := "<!DOCTYPE html><html><body>hi</body></html>"
	msg := string(buildSMTPMessage("", "noreply@oxella.com", "user@example.com", "Sub", body))
	if !strings.Contains(msg, "Content-Type: text/html; charset=UTF-8") {
		t.Fatalf("content-type: %s", msg)
	}
	if strings.Contains(msg, "From:  <") {
		t.Fatal("empty from name should not add angle brackets only")
	}
	if !strings.HasPrefix(msg, "From: noreply@oxella.com") {
		t.Fatalf("from: %s", msg)
	}
}

func TestSanitizeHeader(t *testing.T) {
	if got := sanitizeHeader("a\r\nBcc: evil@x.com"); strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Fatalf("injection not stripped: %q", got)
	}
}

func TestOTPEmailBodies(t *testing.T) {
	plain, htmlBody := otpEmailBodies("482917", 10)
	if !strings.Contains(plain, "482917") || !strings.Contains(plain, "10 minutes") {
		t.Fatalf("plain: %s", plain)
	}
	if !strings.Contains(htmlBody, "482917") || !strings.Contains(htmlBody, "Oxella") {
		t.Fatalf("html missing brand/code")
	}
	if !strings.Contains(htmlBody, "<!DOCTYPE html>") {
		t.Fatal("expected html document")
	}
}
