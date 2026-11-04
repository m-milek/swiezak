FROM golang:1.27.3 AS builder

WORKDIR /app

COPY go.mod go.sum* ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o main .

FROM golang:1.27.3

COPY --from=builder --chown=65532:65532 /app/main /app/main

WORKDIR /app

EXPOSE 1991

CMD ["./main"]
