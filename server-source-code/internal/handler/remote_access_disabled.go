package handler

import (
	"encoding/json"
	"net/http"
)

// RemoteAccessDisabled answers the SSH/RDP routes when PM_ENABLE_REMOTE_ACCESS
// is not "true". The routes stay registered so callers get a clear code
// instead of a 404 that looks like a misconfigured proxy.
func RemoteAccessDisabled() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Remote access is disabled on this server",
			"code":  "remote_access_disabled",
		})
	}
}
