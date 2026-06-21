#!/bin/sh

set -eu

PORTNAME="pfSense-pkg-adaptive-limiter"
PORTVERSION="${PORTVERSION:-0.1.0}"
ABI="FreeBSD:15:amd64"
PREFIX="/usr/local"
ROOT=$(cd -- "$(dirname -- "$0")" && pwd)
FILES="${ROOT}/files"
BUILD="${ROOT}/build"
STAGE="${BUILD}/stage"
OUTPUT="${BUILD}/pkg"
BINARY="${ROOT}/dist/adaptive-limiterd"

clean() {
	rm -rf "${BUILD}"
}

json_escape() {
	awk '{ gsub(/\\/, "\\\\"); gsub(/\t/, "\\t"); gsub(/"/, "\\\""); printf "%s\\n", $0 }'
}

stage() {
	if [ ! -x "${BINARY}" ]; then
		echo "Missing FreeBSD/amd64 binary: ${BINARY}" >&2
		echo "Run 'make build-freebsd' first." >&2
		exit 1
	fi

	rm -rf "${STAGE}"
	mkdir -p \
		"${STAGE}${PREFIX}/sbin" \
		"${STAGE}${PREFIX}/etc/rc.d" \
		"${STAGE}${PREFIX}/pkg" \
		"${STAGE}${PREFIX}/www" \
		"${STAGE}${PREFIX}/www/widgets/widgets" \
		"${STAGE}${PREFIX}/share/${PORTNAME}" \
		"${STAGE}/etc/inc/priv"

	install -m 0755 "${BINARY}" "${STAGE}${PREFIX}/sbin/adaptive-limiterd"
	install -m 0555 "${FILES}${PREFIX}/etc/rc.d/adaptive_limiter" "${STAGE}${PREFIX}/etc/rc.d/adaptive_limiter"
	install -m 0644 "${FILES}${PREFIX}/pkg/adaptive-limiter.xml" "${STAGE}${PREFIX}/pkg/adaptive-limiter.xml"
	install -m 0644 "${FILES}${PREFIX}/pkg/adaptive-limiter.inc" "${STAGE}${PREFIX}/pkg/adaptive-limiter.inc"
	install -m 0644 "${FILES}${PREFIX}/www/status_adaptive_limiter.php" "${STAGE}${PREFIX}/www/status_adaptive_limiter.php"
	install -m 0644 "${FILES}${PREFIX}/www/widgets/widgets/adaptive_limiter.widget.php" "${STAGE}${PREFIX}/www/widgets/widgets/adaptive_limiter.widget.php"
	install -m 0644 "${FILES}${PREFIX}/share/${PORTNAME}/info.xml" "${STAGE}${PREFIX}/share/${PORTNAME}/info.xml"
	install -m 0644 "${FILES}/etc/inc/priv/adaptive-limiter.priv.inc" "${STAGE}/etc/inc/priv/adaptive-limiter.priv.inc"

	for file in \
		"${STAGE}${PREFIX}/pkg/adaptive-limiter.xml" \
		"${STAGE}${PREFIX}/share/${PORTNAME}/info.xml"; do
		sed "s/%%PKGVERSION%%/${PORTVERSION}/g" "${file}" > "${file}.tmp"
		mv "${file}.tmp" "${file}"
	done
}

manifest() {
	post_install_script=$(sed "s/%%PORTNAME%%/${PORTNAME}/g" "${FILES}/pkg-install.in" | json_escape)
	pre_deinstall_script=$(sed "s/%%PORTNAME%%/${PORTNAME}/g" "${FILES}/pkg-deinstall.in" | json_escape)

	cat > "${BUILD}/+MANIFEST" <<EOF
name: "${PORTNAME}"
version: "${PORTVERSION}"
origin: "net/${PORTNAME}"
comment: "Adaptive bandwidth controller for pfSense dummynet limiters"
maintainer: "noreply@github.com"
prefix: "${PREFIX}"
abi: "${ABI}"
desc: "Latency-aware dynamic bandwidth control for an existing pfSense dummynet/FQ_CODEL limiter pair."
www: "https://github.com/r0bb10/pfsense-adaptive-limiter"
licenselogic: "single"
licenses: ["ISC"]
categories: ["net"]
scripts: {
  post-install: "${post_install_script}",
  pre-deinstall: "${pre_deinstall_script}"
}
EOF

	sed "s|%%DATADIR%%|share/${PORTNAME}|g" "${ROOT}/pkg-plist" > "${BUILD}/plist"
}

package() {
	stage
	manifest
	mkdir -p "${OUTPUT}"
	pkg create -M "${BUILD}/+MANIFEST" -p "${BUILD}/plist" -r "${STAGE}" -o "${OUTPUT}"
	find "${OUTPUT}" -maxdepth 1 -type f -print
}

case "${1:-package}" in
	clean) clean ;;
	stage) stage ;;
	package) package ;;
	*) echo "Usage: $0 [package|stage|clean]" >&2; exit 2 ;;
esac
