FROM golang:1.26.1 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -o /out/labkit-api ./apps/api/cmd/labkit-api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/labkit-api /usr/local/bin/labkit-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/labkit-api"]
