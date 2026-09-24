package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/timbasel/genetec-technical-assessment/internal/api"
)

func TestDocs(t *testing.T) {
	handler := api.NewServer(nil, nil).Handler()

	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Location") != "/docs/" {
		t.Fatalf("GET /docs = %d, Location %q", redirect.Code, redirect.Header().Get("Location"))
	}

	for _, tt := range []struct {
		path        string
		contentType string
	}{
		{"/docs/", "text/html"},
		{"/docs/swagger-ui-bundle.js", "javascript"},
		{"/docs/swagger-ui.css", "text/css"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), tt.contentType) || response.Body.Len() == 0 {
			t.Fatalf("GET %s = %d, Content-Type %q, body length %d", tt.path, response.Code, response.Header().Get("Content-Type"), response.Body.Len())
		}
		if tt.path == "/docs/" && !strings.Contains(response.Body.String(), `url: "/openapi.yaml"`) {
			t.Fatal("docs page does not load the embedded OpenAPI spec")
		}
	}
}
