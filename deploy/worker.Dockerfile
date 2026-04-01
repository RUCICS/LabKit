FROM golang:1.26.1 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -o /out/labkit-worker ./apps/worker/cmd/labkit-worker

FROM alpine:3.21
RUN apk add --no-cache ca-certificates docker-cli
WORKDIR /app
COPY --from=build /out/labkit-worker /usr/local/bin/labkit-worker
ENTRYPOINT ["/usr/local/bin/labkit-worker"]
