package service

import (
	"strings"
	"testing"
)

func TestBuildPasswordResetURL(t *testing.T) {
	got := buildPasswordResetURL("https://app.oxella.com", "abc123token")
	want := "https://app.oxella.com/reset-password?token=abc123token"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	// Trailing slash + accidental path ignored
	got = buildPasswordResetURL("https://app.oxella.com/login/", "tok")
	if !strings.HasPrefix(got, "https://app.oxella.com/reset-password?token=") {
		t.Fatalf("path not forced: %q", got)
	}
	if strings.Contains(got, "/login") {
		t.Fatalf("should not keep /login: %q", got)
	}

	// Missing scheme
	got = buildPasswordResetURL("app.oxella.com", "xyz")
	if !strings.HasPrefix(got, "https://app.oxella.com/reset-password?token=") {
		t.Fatalf("scheme: %q", got)
	}

	// Query escape
	got = buildPasswordResetURL("https://app.oxella.com", "a+b=c&d")
	if !strings.Contains(got, "token=a%2Bb%3Dc%26d") {
		t.Fatalf("escape: %q", got)
	}
}

func TestPasswordResetEmailBodies_ButtonHref(t *testing.T) {
	link := "https://app.oxella.com/reset-password?token=deadbeef"
	_, htmlBody := passwordResetEmailBodies(link, 60)
	if !strings.Contains(htmlBody, `href="https://app.oxella.com/reset-password?token=deadbeef"`) {
		t.Fatalf("button/link href missing full URL:\n%s", htmlBody)
	}
	// should not use minutes (60) as href
	if strings.Contains(htmlBody, `href="60"`) {
		t.Fatal("href incorrectly set to expiry minutes")
	}
}
