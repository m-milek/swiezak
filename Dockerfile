FROM --platform=$BUILDPLATFORM golang:1.27.3 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /app

COPY go.mod go.sum* ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build \
    -ldflags="-s -w" \
    -o main .

FROM golang:1.27.3

COPY --from=builder --chown=65532:65532 /app/main /app/main

WORKDIR /app

EXPOSE 1991

CMD ["./main"]
