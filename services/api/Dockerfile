FROM golang:1.27.0-alpine3.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.24.1

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S lifehub \
    && adduser -S -G lifehub -h /nonexistent -s /sbin/nologin lifehub
COPY --from=build /out/api /out/worker /out/migrate /usr/local/bin/
USER lifehub
EXPOSE 10000
CMD ["/usr/local/bin/api"]
