FROM golang:1.26.1 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -o /out/labkit-migrate ./apps/migrate/cmd/labkit-migrate

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata postgresql-client
WORKDIR /app
COPY --from=build /out/labkit-migrate /usr/local/bin/labkit-migrate
ENTRYPOINT ["/usr/local/bin/labkit-migrate"]
