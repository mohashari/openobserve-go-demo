FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o load-generator .

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/load-generator .
CMD ["./load-generator"]
