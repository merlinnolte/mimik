# Bauen
FROM golang:1.26-alpine AS bau
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Cgo-frei dank modernc.org/sqlite. Ohne Cgo braucht das Ergebnis keine
# Systembibliotheken und liefe sogar in scratch.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mimik ./cmd/server

# Laufen
FROM alpine:3.20
# ca-certificates für HTTPS zum Modellanbieter, tzdata für lesbare Zeitstempel.
# Mehr kommt nicht dazu: Was nicht im Abbild liegt, kann auch nicht ausgeführt
# werden, wenn jemand doch einmal an eine Schale käme.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 spiel \
    && mkdir -p /daten && chown spiel:spiel /daten
COPY --from=bau /mimik /usr/local/bin/mimik
USER spiel
ENV MIMIK_DB=/daten/mimik.db MIMIK_ADDR=:8080

# Das Verzeichnis MUSS vor VOLUME existieren und dem Dienst gehören.
#
# Legt man es nicht an, erzeugt Docker es beim ersten Start selbst - als root
# mit 0755. Der Dienst läuft aber als spiel (uid 10001) und kann dann keine
# Datei darin anlegen; SQLite meldet "unable to open database file (14)" und
# der Server versucht es endlos neu. Genau so ist es beim ersten echten Start
# passiert. Beim Anlegen eines LEEREN Volumes übernimmt Docker Besitzer und
# Modus dieses Verzeichnisses - deshalb steht chown hier und nicht im Start.
VOLUME /daten
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mimik"]
