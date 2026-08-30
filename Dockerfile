# LabNTP production image: ghcr.io/hilather/labntp
#
# Multi-stage, static binary, numeric non-root UID, no shell.
# No Node stage — UI-001 embeds dist/ on the host.
# Run with a read-only root filesystem. Compose restores only
# NET_BIND_SERVICE so bind-to-123 works. Default make test-container
# (later) uses --ntp-listen=:1123 and cap_drop ALL.

FROM golang:1.26.6-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -trimpath \
	-ldflags="-s -w \
	-X github.com/hilather/go-lab-ntp/internal/buildinfo.version=${VERSION} \
	-X github.com/hilather/go-lab-ntp/internal/buildinfo.commit=${COMMIT} \
	-X github.com/hilather/go-lab-ntp/internal/buildinfo.buildTime=${BUILD_TIME}" \
	-o /out/labntp ./cmd/labntp \
	&& printf 'labntp:x:65532:65532:labntp:/:/sbin/nologin\n' > /out/passwd \
	&& printf 'labntp:x:65532:\n' > /out/group \
	&& cp /etc/ssl/certs/ca-certificates.crt /out/ca-certificates.crt \
	&& cp LICENSE /out/LICENSE

FROM scratch

LABEL org.opencontainers.image.title="labntp" \
	org.opencontainers.image.description="Laboratory NTPv3/v4 server with per-IP virtual clocks" \
	org.opencontainers.image.source="https://github.com/hilather/go-lab-ntp" \
	org.opencontainers.image.url="https://github.com/hilather/go-lab-ntp" \
	org.opencontainers.image.licenses="Apache-2.0" \
	org.opencontainers.image.vendor="hilather" \
	org.opencontainers.image.documentation="https://github.com/hilather/go-lab-ntp/blob/main/docs/11-deployment.md"

COPY --from=build /out/passwd /etc/passwd
COPY --from=build /out/group /etc/group
COPY --from=build /out/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/labntp /labntp
COPY --from=build /out/LICENSE /LICENSE

USER 65532:65532
EXPOSE 123/udp 8088/tcp
WORKDIR /

HEALTHCHECK --interval=10s --timeout=3s --start-period=3s --retries=3 \
	CMD ["/labntp", "healthcheck", "--url=http://127.0.0.1:8088/v1/health/ready"]

ENTRYPOINT ["/labntp"]
# serve --management-listen defaults to off (NTP-only). The image must bind
# management so HEALTHCHECK and authenticated /v1 work from the published 8088.
CMD ["serve", "--config=/etc/labntp/config.yaml", "--management-listen=:8088"]
