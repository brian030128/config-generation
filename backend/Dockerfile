FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /server .
RUN go build -tags 'postgres' -o /migrate github.com/golang-migrate/migrate/v4/cmd/migrate

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app
COPY --from=builder /server /server
COPY --from=builder /migrate /usr/local/bin/migrate
COPY db/migrations /migrations
USER app
EXPOSE 8080
CMD ["/server"]
