package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{"GET", "/healthz", 200}, {"POST", "/healthz", 405}, {"GET", "/api/me", 404},
	} {
		res := httptest.NewRecorder()
		handler().ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != tc.code {
			t.Fatalf("%s %s: %d", tc.method, tc.path, res.Code)
		}
		if tc.code == 200 {
			var body struct {
				Status  string
				Service string
			}
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.Status != "ok" || body.Service != "qingfan-server" {
				t.Fatalf("invalid health: %s", res.Body)
			}
		}
	}
}
