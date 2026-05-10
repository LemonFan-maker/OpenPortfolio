FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY apps/server/go.mod apps/server/go.sum* ./
RUN go mod download
COPY apps/server ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /server /app/server
EXPOSE 8080
CMD ["/app/server"]
