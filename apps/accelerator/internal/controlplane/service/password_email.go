package service

import (
	"fmt"
	"html"
	"strconv"
)

const passwordResetEmailSubject = "Reset your Oxella password"

func passwordResetEmailBodies(resetLink string, expiryMinutes int) (plain, htmlBody string) {
	if expiryMinutes <= 0 {
		expiryMinutes = 60
	}
	link := html.EscapeString(resetLink)
	mins := strconv.Itoa(expiryMinutes)

	plain = fmt.Sprintf(
		"Reset your Oxella password\n\n"+
			"Open this link to choose a new password (expires in %s minutes):\n%s\n\n"+
			"If you did not request a reset, you can ignore this email.\n\n"+
			"— Oxella Technologies (Hosted Nance)\n",
		mins, resetLink,
	)

	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8" /><title>Reset your password</title></head>
<body style="margin:0;padding:0;background:#070b14;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#e8edf7;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#070b14;padding:40px 16px;">
    <tr><td align="center">
      <table role="presentation" width="100%%" style="max-width:480px;background:#0f1629;border:1px solid #1e2a44;border-radius:12px;padding:32px;">
        <tr><td>
          <p style="margin:0 0 8px;font-size:12px;letter-spacing:0.12em;text-transform:uppercase;color:#8b9bb8;">Oxella</p>
          <h1 style="margin:0 0 16px;font-size:22px;color:#f5f7fb;">Reset your password</h1>
          <p style="margin:0 0 24px;font-size:15px;line-height:1.5;color:#b8c4d9;">
            We received a request to reset the password for your account. This link expires in %s minutes.
          </p>
          <p style="margin:0 0 28px;">
            <a href="%s" style="display:inline-block;background:#f97316;color:#0b1220;text-decoration:none;font-weight:600;padding:12px 20px;border-radius:8px;">
              Choose a new password
            </a>
          </p>
          <p style="margin:0;font-size:12px;line-height:1.5;color:#6b7a94;word-break:break-all;">
            Or paste this URL into your browser:<br/>%s
          </p>
          <p style="margin:24px 0 0;font-size:12px;color:#6b7a94;">
            If you did not request this, you can ignore this email.
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, mins, link, link)

	return plain, htmlBody
}
