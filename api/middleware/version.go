package middleware

import (
	"net/http"
)

func AppVersion(appVersion string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("X-App-Version", appVersion)
			next.ServeHTTP(writer, request)
		})
	}
}
