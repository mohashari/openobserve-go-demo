FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o order-service .

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/order-service .
EXPOSE 8080
CMD ["./order-service"]
