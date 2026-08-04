FROM golang:1.26.5 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o planetarium-app ./cmd/api/main.go

# Pinned rather than :latest so a rebuild months from now produces the same
# image. The binary is built with CGO_ENABLED=0, so it is static and does not
# care that it was compiled on a different distribution.
FROM alpine:3.23

WORKDIR /root/

COPY --from=builder /app/planetarium-app .

EXPOSE 8080

CMD ["./planetarium-app"]
