// Package api provides HTTP handlers for the Peekaboo backend.
package api

import "net/http"

// ContentSecurityPolicy defines the CSP header value.
// - default-src 'self': only allow resources from same origin
// - script-src 'self': only allow scripts from same origin (Astro bundles)
// - style-src 'self' 'unsafe-inline': allow inline styles (Astro components use <style> tags)
// - img-src 'self' data: blob:: images from same origin, data URIs, and blob URLs
// - media-src 'self' blob:: audio/video from same origin and blob URLs (MediaRecorder)
// - connect-src 'self': API calls to same origin only
// - frame-ancestors 'none': no iframe embedding (CSP equivalent of X-Frame-Options: DENY)
const ContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; frame-ancestors 'none'"

// PermissionsPolicy defines the Permissions-Policy header value.
// - microphone=self: allow microphone access (required for voice input)
// - camera=(): deny camera access
// - geolocation=(): deny location access
// - payment=(): deny payment API
// - usb=(): deny USB access
// - interest-cohort=(): opt out of FLoC/Topics tracking
const PermissionsPolicy = "microphone=(self), camera=(), geolocation=(), payment=(), usb=(), interest-cohort=()"

// SecurityHeadersMiddleware wraps an http.Handler to add security headers.
// These headers help protect against common web vulnerabilities:
// - Content-Security-Policy: restricts resource loading to prevent XSS and injection attacks
// - X-Frame-Options: DENY prevents clickjacking by disallowing iframe embedding
// - X-Content-Type-Options: nosniff prevents MIME type sniffing attacks
// - X-XSS-Protection: 1; mode=block enables browser XSS filtering (legacy, but harmless)
// - Referrer-Policy: limits referrer info sent to other origins
// - Permissions-Policy: restricts browser features (allows mic, denies camera/geo/payment/usb)
// - Strict-Transport-Security: HSTS header when HTTPS_ONLY=true
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", ContentSecurityPolicy)
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", PermissionsPolicy)

		if isHTTPSOnly() {
			// max-age=31536000 = 1 year, includeSubDomains for full coverage
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
