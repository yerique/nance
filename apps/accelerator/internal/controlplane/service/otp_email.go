package service

import (
	"fmt"
	"html"
	"strconv"
	"time"
)

const otpEmailSubject = "Your Oxella sign-in code"

// otpEmailBodies returns plain-text and HTML bodies for the Oxella OTP email.
func otpEmailBodies(code string, expiryMinutes int) (plain, htmlBody string) {
	if expiryMinutes <= 0 {
		expiryMinutes = 10
	}
	code = html.EscapeString(code)
	mins := strconv.Itoa(expiryMinutes)

	plain = fmt.Sprintf(
		"Your Oxella sign-in code is: %s\n\n"+
			"This code expires in %s minutes. Enter it only on https://app.oxella.com.\n\n"+
			"If you did not request this code, you can safely ignore this email.\n\n"+
			"— Oxella Technologies (Hosted Nance)\n",
		code, mins,
	)

	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <meta http-equiv="X-UA-Compatible" content="IE=edge" />
  <title>Your Oxella sign-in code</title>
</head>
<body style="margin:0; padding:0; background-color:#070b14; font-family: 'DM Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;">
  <div style="display:none; max-height:0; overflow:hidden; mso-hide:all;">
    Your Oxella verification code is %s. It expires in %s minutes.
  </div>
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="background-color:#070b14; padding: 40px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="max-width:480px; background-color:#0d1424; border:1px solid rgba(120,180,255,0.12); border-radius:14px; overflow:hidden;">
          <tr>
            <td style="padding: 28px 32px 20px 32px; border-bottom:1px solid rgba(120,180,255,0.12);">
              <table role="presentation" cellspacing="0" cellpadding="0" border="0">
                <tr>
                  <td style="vertical-align:middle; padding-right:12px;">
                    <div style="width:36px; height:36px; border-radius:9px; background: linear-gradient(135deg, #3de0c5 0%%, #4d8fff 100%%); text-align:center; line-height:36px; font-family: Consolas, 'Courier New', monospace; font-size:12px; font-weight:700; color:#061018;">ox</div>
                  </td>
                  <td style="vertical-align:middle;">
                    <div style="font-size:17px; font-weight:700; color:#e8eef9; letter-spacing:0.2px; line-height:1.2;">Oxella</div>
                    <div style="font-size:12px; color:#8b9bb8; margin-top:2px;">Hosted Nance</div>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <tr>
            <td style="padding: 32px 32px 28px 32px;">
              <h1 style="margin:0 0 10px 0; font-size:22px; font-weight:700; color:#e8eef9; line-height:1.3;">Your sign-in code</h1>
              <p style="margin:0 0 28px 0; font-size:15px; line-height:1.65; color:#8b9bb8;">
                Use this one-time password to sign in to the
                <a href="https://app.oxella.com" style="color:#3de0c5; text-decoration:none;">Oxella console</a>
                and manage your hosted Nance organization, connections, and cache policy.
              </p>
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="margin:0 0 24px 0;">
                <tr>
                  <td align="center" style="background-color:#111a2e; border:1px solid rgba(120,180,255,0.12); border-radius:12px; padding:28px 16px;">
                    <div style="font-size:11px; font-weight:600; letter-spacing:1.4px; text-transform:uppercase; color:#8b9bb8; margin-bottom:12px;">Verification code</div>
                    <div style="font-size:36px; font-weight:700; letter-spacing:10px; color:#e8eef9; font-family: Consolas, 'Courier New', monospace; line-height:1.2;">%s</div>
                  </td>
                </tr>
              </table>
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="margin:0 0 28px 0;">
                <tr>
                  <td style="background-color:rgba(61,224,197,0.10); border-left:3px solid #3de0c5; border-radius:0 10px 10px 0; padding:14px 16px;">
                    <p style="margin:0; font-size:14px; line-height:1.5; color:#e8eef9;">
                      This code expires in <strong style="color:#3de0c5;">%s minutes</strong>.
                      Enter it only on
                      <a href="https://app.oxella.com" style="color:#3de0c5; text-decoration:none;">app.oxella.com</a>.
                    </p>
                  </td>
                </tr>
              </table>
              <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin:0 0 28px 0;">
                <tr>
                  <td align="center" style="border-radius:10px; background: linear-gradient(135deg, #3de0c5 0%%, #4d8fff 100%%);">
                    <a href="https://app.oxella.com/login" target="_blank"
                       style="display:inline-block; padding:14px 28px; font-size:14px; font-weight:600; color:#061018; text-decoration:none; border-radius:10px;">
                      Open Oxella console →
                    </a>
                  </td>
                </tr>
              </table>
              <p style="margin:0 0 10px 0; font-size:13px; line-height:1.6; color:#8b9bb8;">
                For your security, never share this code. Oxella will never ask for your OTP by phone, chat, or a link outside
                <span style="color:#e8eef9;">app.oxella.com</span>.
              </p>
              <p style="margin:0; font-size:13px; line-height:1.6; color:#8b9bb8;">
                If you didn’t request this code, you can safely ignore this email — no changes will be made to your account.
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 32px;">
              <div style="height:1px; background-color:rgba(120,180,255,0.12); width:100%%; line-height:1px; font-size:1px;">&nbsp;</div>
            </td>
          </tr>
          <tr>
            <td style="padding: 22px 32px 28px 32px; text-align:center;">
              <p style="margin:0 0 6px 0; font-size:13px; color:#8b9bb8;">
                <strong style="color:#e8eef9; font-weight:600;">Oxella Technologies</strong>
                · Hosted Nance
              </p>
              <p style="margin:0 0 12px 0; font-size:12px; line-height:1.5; color:#8b9bb8;">
                <a href="https://oxella.com" style="color:#4d8fff; text-decoration:none;">oxella.com</a>
                &nbsp;·&nbsp;
                <a href="https://app.oxella.com" style="color:#4d8fff; text-decoration:none;">app.oxella.com</a>
              </p>
              <p style="margin:0; font-size:11px; line-height:1.5; color:#5c6b85;">
                This is an automated message. Please do not reply.
              </p>
            </td>
          </tr>
        </table>
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="max-width:480px; margin-top:20px;">
          <tr>
            <td align="center" style="padding:0 12px;">
              <p style="margin:0; font-size:11px; line-height:1.55; color:#5c6b85;">
                © %d Oxella Technologies. All rights reserved.<br />
                You’re receiving this because someone used this email to sign in to hosted Nance on Oxella.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, code, mins, code, mins, time.Now().UTC().Year())

	return plain, htmlBody
}
