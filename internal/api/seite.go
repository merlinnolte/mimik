package api

import (
	"embed"
	"net/http"
)

// Die Startseite liegt im Binary, nicht auf der Platte. Der Behälter läuft
// read_only und hat kein Wurzelverzeichnis für Dateien; ein Bindemount dafür
// wäre ein Unterbau, den es hier bewusst nicht gibt. Eingebettet bleibt es
// bei: ein Binary, eine SQLite-Datei.
//
// Die Schrift liegt bei, sie kommt nicht von Google. Deren Ausschnitt von
// JetBrains Mono enthält U+2500–259F nicht, und ohne diese Zeichen fällt
// MIMIKs Gesicht auf eine andere Schrift mit anderer Laufweite zurück – der
// Mangel, der in Theme.kt steht.
//
//go:embed seite/index.html seite/schrift/jetbrains-mono.woff2
var seiteDateien embed.FS

// Nur die zwei Pfade, die es gibt, und jeder ausgeschrieben. Ein FileServer
// unter "/" wäre ein Auffangmuster: Jeder vertippte API-Pfad bekäme dann 200
// und die Startseite statt 404.
func (s *Server) startseite(w http.ResponseWriter, r *http.Request) {
	// Die Seite ändert sich mit jedem Release, die Schrift nie. Deshalb hier
	// eine Rückfrage pro Aufruf und dort ein Jahr.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFileFS(w, r, seiteDateien, "seite/index.html")
}

func (s *Server) schrift(w http.ResponseWriter, r *http.Request) {
	// Alpine bringt keine /etc/mime.types mit, und Gos eingebaute Tabelle
	// kennt .woff2 nicht: ohne diese Zeile ginge die Datei als
	// application/octet-stream raus.
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFileFS(w, r, seiteDateien, "seite/schrift/jetbrains-mono.woff2")
}
