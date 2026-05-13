package middleware

import "net/http"

func RequireRoles(allowedRoles ...string) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRoles, ok := r.Context().Value(UserRolesKey).([]string)
			if !ok || len(userRoles) == 0 {
				http.Error(w, "Forbidden: no roles assigned", http.StatusForbidden)
				return
			}

			hasAccess := false
			for _, allowedRole := range allowedRoles {
				for _, userRole := range userRoles {
					if allowedRole == userRole {
						hasAccess = true
						break
					}
				}
				if hasAccess {
					break
				}
			}

			if !hasAccess {
				http.Error(w, "forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
