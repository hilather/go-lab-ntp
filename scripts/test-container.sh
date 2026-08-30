#!/usr/bin/env bash
# Container contract. Requires Docker. Fail closed if the daemon is missing.
# Default path: --ntp-listen=:1123 and cap_drop ALL.
# Gated path: LABNTP_TEST_NET_BIND=1 runs :123 + NET_BIND_SERVICE (skip if the
# runtime cannot grant the cap).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${LABNTP_TEST_IMAGE:-ghcr.io/hilather/labntp:test}"
NAME="labntp-container-test-$$"
COMPOSE="${ROOT}/examples/compose.smoke.yaml"
TOKEN="${ROOT}/testdata/container/token"
CONFIG="${ROOT}/testdata/container/config.yaml"

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required for make test-container" >&2
	exit 1
fi
if ! docker info >/dev/null 2>&1; then
	echo "docker daemon is not available for make test-container" >&2
	exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
	echo "curl is required for make test-container" >&2
	exit 1
fi
if [ ! -f "${TOKEN}" ] || [ ! -f "${CONFIG}" ]; then
	echo "missing testdata/container/{token,config.yaml}" >&2
	exit 1
fi

cleanup() {
	docker rm -f "${NAME}" >/dev/null 2>&1 || true
	docker rm -f "${NAME}-bind" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "building ${IMAGE}"
docker build -t "${IMAGE}" "${ROOT}"

inspect_user="$(docker image inspect --format '{{.Config.User}}' "${IMAGE}")"
if [ "${inspect_user}" != "65532:65532" ]; then
	echo "image User=${inspect_user}, want 65532:65532" >&2
	exit 1
fi

licenses="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.licenses"}}' "${IMAGE}")"
if [ "${licenses}" != "Apache-2.0" ]; then
	echo "image license label=${licenses}, want Apache-2.0" >&2
	exit 1
fi

hc="$(docker image inspect --format '{{json .Config.Healthcheck.Test}}' "${IMAGE}")"
case "${hc}" in
*CMD-SHELL*)
	echo "image HEALTHCHECK=${hc} must be exec form, not shell" >&2
	exit 1
	;;
esac
case "${hc}" in
'["CMD",'*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want JSON array starting with CMD" >&2
	exit 1
	;;
esac
case "${hc}" in
*'/v1/health/ready'*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want /v1/health/ready" >&2
	exit 1
	;;
esac
case "${hc}" in
*healthcheck*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want exec-form labntp healthcheck" >&2
	exit 1
	;;
esac

if docker compose version >/dev/null 2>&1; then
	docker compose -f "${COMPOSE}" config >/dev/null
else
	echo "docker compose plugin not available; compose file parse skipped" >&2
fi

docker run -d --name "${NAME}" \
	--read-only \
	--cap-drop=ALL \
	--security-opt=no-new-privileges:true \
	--tmpfs /tmp:rw,noexec,nosuid,size=16m \
	-v "${CONFIG}:/etc/labntp/config.yaml:ro" \
	-v "${TOKEN}:/etc/labntp/token:ro" \
	-p 127.0.0.1:0:1123/udp \
	-p 127.0.0.1:0:8088/tcp \
	"${IMAGE}" \
	serve --config=/etc/labntp/config.yaml --ntp-listen=:1123 --management-listen=:8088

if [ "$(docker inspect --format '{{.State.Running}}' "${NAME}")" != "true" ]; then
	echo "container is not running" >&2
	docker inspect --format 'status={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}}' "${NAME}" >&2 || true
	docker logs "${NAME}" >&2 || true
	exit 1
fi

readonly_root="$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "${NAME}")"
if [ "${readonly_root}" != "true" ]; then
	echo "HostConfig.ReadonlyRootfs=${readonly_root}, want true" >&2
	exit 1
fi

mgmt_port="$(docker port "${NAME}" 8088/tcp | head -n1 | awk -F: '{print $NF}')"
ntp_port="$(docker port "${NAME}" 1123/udp | head -n1 | awk -F: '{print $NF}')"
if [ -z "${mgmt_port}" ] || [ -z "${ntp_port}" ]; then
	echo "published ports missing mgmt=${mgmt_port} ntp=${ntp_port}" >&2
	docker inspect --format '{{json .HostConfig.PortBindings}}' "${NAME}" >&2 || true
	docker logs "${NAME}" >&2 || true
	exit 1
fi

ok=0
for _ in $(seq 1 40); do
	if curl -fsS "http://127.0.0.1:${mgmt_port}/v1/health/ready" >/dev/null 2>&1; then
		ok=1
		break
	fi
	sleep 0.25
done
if [ "${ok}" -ne 1 ]; then
	echo "management ready check failed on 127.0.0.1:${mgmt_port}" >&2
	docker inspect --format 'status={{.State.Status}} exit={{.State.ExitCode}}' "${NAME}" >&2 || true
	docker logs "${NAME}" >&2 || true
	exit 1
fi

if ! docker exec "${NAME}" /labntp version >/dev/null; then
	echo "non-root exec of /labntp version failed" >&2
	exit 1
fi
if ! docker exec "${NAME}" /labntp healthcheck --url=http://127.0.0.1:8088/v1/health/ready >/dev/null; then
	echo "in-container HTTP ready healthcheck failed" >&2
	exit 1
fi
if docker exec "${NAME}" /bin/sh -c true >/dev/null 2>&1; then
	echo "image has a shell at /bin/sh" >&2
	exit 1
fi

SMOKE_TOKEN="$(tr -d '\r\n' < "${TOKEN}")"

unauth="$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${mgmt_port}/v1/state")"
if [ "${unauth}" != "401" ]; then
	echo "unauthenticated GET /v1/state status=${unauth}, want 401" >&2
	exit 1
fi

preview="$(curl -fsS -H "Authorization: Bearer ${SMOKE_TOKEN}" \
	"http://127.0.0.1:${mgmt_port}/v1/views/preview?ip=127.0.0.1")"
if ! printf '%s\n' "${preview}" | grep -q '"filter":"default"'; then
	echo "preview missing default filter: ${preview}" >&2
	exit 1
fi
if ! printf '%s\n' "${preview}" | grep -q '"mode":"follow-real"'; then
	echo "preview missing follow-real: ${preview}" >&2
	exit 1
fi

python3 - "${ntp_port}" <<'PY'
import socket
import sys

port = int(sys.argv[1])
pkt = bytearray(48)
pkt[0] = 0x23  # LI=0 VN=4 Mode=3
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(2)
s.sendto(pkt, ("127.0.0.1", port))
data, _ = s.recvfrom(576)
if len(data) < 48:
    raise SystemExit(f"short NTP reply {len(data)}")
li_vn_mode = data[0]
if (li_vn_mode & 0x07) != 4:
    raise SystemExit(f"mode={li_vn_mode & 0x07} want 4")
if data[1] != 2:
    raise SystemExit(f"stratum={data[1]} want 2")
PY

echo "container contract ok image=${IMAGE}"

if [ "${LABNTP_TEST_NET_BIND:-}" != "1" ]; then
	exit 0
fi

echo "gated NET_BIND_SERVICE smoke"
if ! docker run -d --name "${NAME}-bind" \
	--read-only \
	--user 65532:65532 \
	--cap-drop=ALL \
	--cap-add=NET_BIND_SERVICE \
	--security-opt=no-new-privileges:true \
	--tmpfs /tmp:rw,noexec,nosuid,size=16m \
	-v "${CONFIG}:/etc/labntp/config.yaml:ro" \
	-v "${TOKEN}:/etc/labntp/token:ro" \
	-p 127.0.0.1::8088/tcp \
	"${IMAGE}" \
	serve --config=/etc/labntp/config.yaml --ntp-listen=:123 --management-listen=:8088; then
	echo "LABNTP_TEST_NET_BIND=1: runtime could not start with NET_BIND_SERVICE; skip" >&2
	exit 0
fi

bind_mgmt="$(docker port "${NAME}-bind" 8088/tcp | head -n1 | awk -F: '{print $NF}')"
ok=0
for _ in $(seq 1 40); do
	if curl -fsS "http://127.0.0.1:${bind_mgmt}/v1/health/ready" >/dev/null 2>&1; then
		ok=1
		break
	fi
	sleep 0.25
done
if [ "${ok}" -ne 1 ]; then
	echo "NET_BIND_SERVICE :123 ready check failed" >&2
	docker logs "${NAME}-bind" >&2 || true
	exit 1
fi
echo "gated NET_BIND_SERVICE :123 ok"
