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
    -o main . && mkdir /data

FROM gcr.io/distroless/static-debian13

COPY --from=builder /app/main /swiezak
COPY --from=builder /data /data

ENV REPORT_PATH=/data/report.json

EXPOSE 1991

ENTRYPOINT ["/swiezak"]
