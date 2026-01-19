package auth

import (
	"net/http"

	"github.com/trevor/subtitler/internal/db"
)

// AdminMiddleware checks if the authenticated user is an admin.
// This middleware must be used AFTER AuthMiddleware in the middleware chain.
func AdminMiddleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context (set by AuthMiddleware)
			claims, ok := GetClaims(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Check if user is admin in database
			user, err := database.GetUserByID(claims.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to verify admin status")
				return
			}

			if !user.IsAdmin {
				writeError(w, http.StatusForbidden, "Admin access required")
				return
			}

			// User is admin, proceed
			next.ServeHTTP(w, r)
		})
	}
}
