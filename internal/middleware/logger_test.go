package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestLoggerMasksDownloadTickets(t *testing.T) {
	var buf bytes.Buffer
	logged := chimiddleware.RequestLogger(maskingFormatter{
		next: &chimiddleware.DefaultLogFormatter{Logger: log.New(&buf, "", 0), NoColor: true},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The handler still sees the real ticket.
		if !strings.HasSuffix(r.RequestURI, "SECRET-TICKET") {
			t.Errorf("handler got %q", r.RequestURI)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/public/downloads/SECRET-TICKET", nil)
	logged.ServeHTTP(httptest.NewRecorder(), req)

	if strings.Contains(buf.String(), "SECRET-TICKET") {
		t.Fatalf("ticket leaked into the log: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "/api/public/downloads/[ticket]") {
		t.Fatalf("download request not logged: %s", buf.String())
	}
}
