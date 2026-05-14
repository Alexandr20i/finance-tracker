FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o transaction ./services/transaction/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/transaction .
EXPOSE 50051
CMD ["./transaction"]
