# --- build stage ---
FROM golang:1.23-alpine AS build
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
