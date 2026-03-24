FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o inventory-service .

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/inventory-service .
EXPOSE 9090
CMD ["./inventory-service"]
