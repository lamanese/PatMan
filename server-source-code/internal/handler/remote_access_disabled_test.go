package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteAccessDisabledReturns403WithCode(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	RemoteAccessDisabled().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/ssh-ticket", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if body["code"] != "remote_access_disabled" {
		t.Fatalf("code=%q", body["code"])
	}
}
