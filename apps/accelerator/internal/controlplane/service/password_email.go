package service

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

const passwordResetEmailSubject = "Reset your Oxella password"

// passwordResetEmailBodies builds plain + HTML bodies.
// resetLink must be an absolute https URL (see buildPasswordResetURL).
func passwordResetEmailBodies(resetLink string, expiryMinutes int) (plain, htmlBody string) {
	if expiryMinutes <= 0 {
		expiryMinutes = 60
	}
	resetLink = strings.TrimSpace(resetLink)
	// Attribute-safe href (escape &, quotes, etc.). Do not escape for plain text.
	href := html.EscapeString(resetLink)
	display := html.EscapeString(resetLink)
	mins := strconv.Itoa(expiryMinutes)

	plain = fmt.Sprintf(
		"Reset your Oxella password\n\n"+
			"Open this link to choose a new password (expires in %s minutes):\n%s\n\n"+
			"If you did not request a reset, you can ignore this email.\n\n"+
			"— Oxella Technologies (Hosted Nance)\n",
		mins, resetLink,
	)

	// Use a table-based button so email clients keep the full href.
	// Placeholders: mins, href, display — exactly three.
	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <meta name="color-scheme" content="light only" />
  <meta name="supported-color-schemes" content="light" />
  <title>Reset your password</title>
</head>
<body style="margin:0;padding:0;background:#f4f6fa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0f172a;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="background:#f4f6fa;padding:40px 16px;">
    <tr><td align="center">
      <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="max-width:480px;background:#ffffff;border:1px solid #e2e8f0;border-radius:12px;">
        <tr><td style="padding:32px;">
          <p style="margin:0 0 8px;font-size:12px;letter-spacing:0.12em;text-transform:uppercase;color:#64748b;">Oxella</p>
          <h1 style="margin:0 0 16px;font-size:22px;color:#0f172a;">Reset your password</h1>
          <p style="margin:0 0 24px;font-size:15px;line-height:1.5;color:#475569;">
            We received a request to reset the password for your account. This link expires in %s minutes.
          </p>
          <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin:0 0 28px 0;">
            <tr>
              <td align="center" bgcolor="#f97316" style="border-radius:8px;background-color:#f97316;">
                <a href="%s" target="_blank" rel="noopener noreferrer"
                   style="display:inline-block;padding:12px 20px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;border-radius:8px;">
                  Choose a new password
                </a>
              </td>
            </tr>
          </table>
          <p style="margin:0;font-size:12px;line-height:1.5;color:#64748b;word-break:break-all;">
            Or paste this URL into your browser:<br/>
            <a href="%s" style="color:#2563eb;text-decoration:underline;">%s</a>
          </p>
          <p style="margin:24px 0 0;font-size:12px;color:#94a3b8;">
            If you did not request this, you can ignore this email.
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, mins, href, href, display)

	return plain, htmlBody
}
