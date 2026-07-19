FROM golang:1.26.5-alpine AS builder

WORKDIR /app

COPY go.mod ./
#COPY go.sum

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o planetarium-app ./cmd/api/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/planetarium-app .

EXPOSE 8080

CMD ["./planetarium-app"]
