package middleware

import (
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// downloadPrefix is where download tickets travel. A ticket is a bearer
// credential for a file until it expires, so it must not land in the request
// log in readable form.
const downloadPrefix = "/api/public/downloads/"

// Logger is chi's request logger with download tickets masked.
func Logger(next http.Handler) http.Handler {
	return chimiddleware.RequestLogger(maskingFormatter{
		// Same settings as chi's own default logger, so the log looks as before.
		next: &chimiddleware.DefaultLogFormatter{Logger: log.New(os.Stdout, "", log.LstdFlags), NoColor: runtime.GOOS == "windows"},
	})(next)
}

type maskingFormatter struct {
	next chimiddleware.LogFormatter
}

func (f maskingFormatter) NewLogEntry(r *http.Request) chimiddleware.LogEntry {
	if strings.HasPrefix(r.URL.Path, downloadPrefix) {
		// A shallow copy is enough: the formatter only reads the request line,
		// and the handler keeps the original.
		masked := *r
		masked.RequestURI = downloadPrefix + "[ticket]"
		return f.next.NewLogEntry(&masked)
	}
	return f.next.NewLogEntry(r)
}
