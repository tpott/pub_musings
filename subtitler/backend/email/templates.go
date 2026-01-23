package email

import "fmt"

// PasswordResetHTML returns the HTML body for password reset emails
func PasswordResetHTML(resetURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset Your Password</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="text-align: center; margin-bottom: 30px;">
        <h1 style="color: #1a1a1a; margin: 0;">Subtitler</h1>
    </div>

    <div style="background: #f9f9f9; border-radius: 8px; padding: 30px;">
        <h2 style="margin-top: 0;">Reset Your Password</h2>
        <p>We received a request to reset your password. Click the button below to create a new password:</p>

        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; background: #4F46E5; color: white; text-decoration: none; padding: 12px 30px; border-radius: 6px; font-weight: 500;">Reset Password</a>
        </div>

        <p style="color: #666; font-size: 14px;">This link will expire in 1 hour. If you didn't request a password reset, you can safely ignore this email.</p>

        <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">

        <p style="color: #999; font-size: 12px;">If the button doesn't work, copy and paste this link into your browser:</p>
        <p style="color: #4F46E5; font-size: 12px; word-break: break-all;">%s</p>
    </div>

    <div style="text-align: center; margin-top: 30px; color: #999; font-size: 12px;">
        <p>Subtitler - AI-powered video subtitles</p>
    </div>
</body>
</html>`, resetURL, resetURL)
}

// PasswordResetText returns the plain text body for password reset emails
func PasswordResetText(resetURL string) string {
	return fmt.Sprintf(`Reset Your Password

We received a request to reset your Subtitler password.

Click the link below to create a new password:
%s

This link will expire in 1 hour.

If you didn't request a password reset, you can safely ignore this email.

---
Subtitler - AI-powered video subtitles
`, resetURL)
}
