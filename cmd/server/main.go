// Kommando mimik-server: ein Binary, eine SQLite-Datei, kein Unterbau.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"mimik/internal"
	"mimik/internal/api"
	"mimik/internal/mimik"
	"mimik/internal/store"
)

func env(k, standard string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return standard
}

func main() {
	log.SetFlags(log.Ltime)

	pfad := env("MIMIK_DB", "mimik.db")
	s, err := store.Open(pfad)
	if err != nil {
		log.Fatalf("datenbank %s: %v", pfad, err)
	}
	defer s.Close()

	m := mimik.NeuAusUmgebung()
	if m.APIKey == "" {
		log.Print("WARNUNG: MIMIK_API_KEY fehlt – Runden bleiben auf MIMIK_ARBEITET stehen")
	}
	log.Printf("MIMIK %s · modell %s über %s", internal.Version, m.Model, m.BaseURL)

	einladung := os.Getenv("MIMIK_EINLADUNG")
	if einladung == "" {
		log.Print("WARNUNG: MIMIK_EINLADUNG fehlt – jede Person, die den Server erreicht, " +
			"kann sich anmelden und Modellaufrufe auf deine Rechnung auslösen")
	}

	// Wie viele Proxys vor dem Server stehen. Ohne diese Angabe zaehlt die
	// Anmeldegrenze fuer alle gemeinsam - hinter einem Reverse Proxy sieht der
	// Server nur dessen Adresse. Mit ihr wird die weitergereichte gezaehlt, und
	// zwar von rechts: alles weiter links kann sich ein Klient ausdenken.
	hops, _ := strconv.Atoi(os.Getenv("MIMIK_PROXY_HOPS"))
	if hops > 0 {
		log.Printf("hinter %d proxy(s): anmeldungen werden je weitergereichter adresse gezählt", hops)
	}

	ctx, stopp := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopp()

	w := api.NeuerWorker(s, m)
	go w.Laufen(ctx)
	// Zweite Schleife: Sie wertet gespielte Runden aus und pflegt das Profil.
	// Getrennt, weil ein Review Minuten dauern kann und den Kartenbau nicht
	// aufhalten darf - auf den wartet ein Mensch.
	go w.LernenLaufen(ctx)

	srv := &http.Server{
		Addr:              env("MIMIK_ADDR", ":8080"),
		Handler:           (&api.Server{S: s, M: m, Worker: w, Tor: api.Torwache{Einladung: einladung, Hops: hops}}).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("hört auf %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("fahre herunter")
	ab, abbruch := context.WithTimeout(context.Background(), 10*time.Second)
	defer abbruch()
	srv.Shutdown(ab)
}
