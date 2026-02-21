FROM golang:1.25-trixie AS builder

WORKDIR /go/src/app

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -o /go/bin/hcloud-ddns-fw .

FROM alpine:3 AS app

WORKDIR /app

RUN apk add --no-cache tzdata ca-certificates

COPY  --from=builder /go/bin/hcloud-ddns-fw .

COPY ./LICENSE ./THIRD_PARTY_LICENSES ./

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD pgrep -x /app/hcloud-ddns-fw >/dev/null || exit 1

ENTRYPOINT [ "/app/hcloud-ddns-fw" ]
