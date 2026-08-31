# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o merka-api ./cmd/api

# Run stage — imagem final mínima
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/merka-api .
EXPOSE 8080
CMD ["./merka-api"]
