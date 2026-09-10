# --- build stage ---
# go.mod requires >= 1.24.0 (a transitive requirement from
# paho.mqtt.golang v1.5.1's own go.mod), so the build image must be at
# least that new.
FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mqtt-nad .

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10000 runner

COPY --from=build /out/mqtt-nad /usr/local/bin/mqtt-nad

USER 10000
ENTRYPOINT ["/usr/local/bin/mqtt-nad"]
