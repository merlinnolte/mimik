package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Die Startseite ist der einzige Pfad, den ein Browser von sich aus ansteuert.
// Vorher gab es dort 404: Der Server kannte nur /healthz und /v1/...
func TestStartseiteKommtOhneAnmeldung(t *testing.T) {
	h := (&Server{}).Routes()

	for _, f := range []struct {
		pfad, typ, suche string
	}{
		{"/", "text/html; charset=utf-8", "MIMIK"},
		{"/schrift/jetbrains-mono.woff2", "font/woff2", "wOF2"},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", f.pfad, nil))

		if w.Code != http.StatusOK {
			t.Errorf("%s: %d statt 200", f.pfad, w.Code)
			continue
		}
		if got := w.Header().Get("Content-Type"); got != f.typ {
			t.Errorf("%s: Content-Type %q statt %q", f.pfad, got, f.typ)
		}
		if !strings.Contains(w.Body.String(), f.suche) {
			t.Errorf("%s: %q fehlt im Körper", f.pfad, f.suche)
		}
	}
}

// Die Seite darf kein Auffangmuster sein. Ein vertippter API-Pfad muss 404
// bleiben - bekäme er die Startseite mit 200, liefe jeder Klient in einen
// JSON-Parserfehler statt in einen klaren Fehlercode.
func TestStartseiteFaengtNichtsAuf(t *testing.T) {
	h := (&Server{}).Routes()

	for _, pfad := range []string{"/v1/quatsch", "/schrift/", "/index.html", "/irgendwas"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", pfad, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: %d statt 404", pfad, w.Code)
		}
	}
}
