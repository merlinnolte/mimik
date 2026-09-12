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
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 spiel
COPY --from=bau /mimik /usr/local/bin/mimik
USER spiel
ENV MIMIK_DB=/daten/mimik.db MIMIK_ADDR=:8080
VOLUME /daten
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mimik"]
