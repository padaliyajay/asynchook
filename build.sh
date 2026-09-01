#!/usr/bin/env bash
#
# Local build helper for asynchook.
#
#   ./build.sh build            build for this machine  -> bin/asynchook
#   ./build.sh run [args...]    build and run against config.local.yaml
#   ./build.sh check            gofmt + go vet + go test
#   ./build.sh release          cross-compile tarballs   -> dist/
#   ./build.sh deb [arch]       build a Debian package (arch: amd64 default, or arm64)
#   ./build.sh clean            remove bin/ and dist/
#
# With no argument it runs `check` then `build`.

set -euo pipefail

cd "$(dirname "$0")"

APP_NAME="asynchook"
BIN_DIR="./bin"
DIST_DIR="./dist"
LOCAL_CONFIG="config.local.yaml"

# Version comes from git when available so release artifacts are traceable.
VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

# Trim the binary; there is no runtime use for the symbol table or DWARF data.
LDFLAGS="-s -w"

# Debian package metadata.
DEB_SECTION="utils"
DEB_PRIORITY="optional"
DEB_MAINTAINER="Jay padaliya <developer.padaliyajay@gmail.com>"
DEB_DESCRIPTION="Offload heavy tasks from a synchronous process to run later as a web hook"

# Platforms built by `release`.
RELEASE_TARGETS=(
	"linux/amd64"
	"linux/arm64"
	"darwin/amd64"
	"darwin/arm64"
)

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

cmd_check() {
	log "gofmt"
	local unformatted
	unformatted="$(gofmt -l .)"
	if [ -n "$unformatted" ]; then
		printf '%s\n' "$unformatted" >&2
		die "the files above need 'gofmt -w .'"
	fi

	log "go vet"
	go vet ./...

	log "go test"
	go test ./...
}

cmd_build() {
	log "building $APP_NAME $VERSION for $(go env GOOS)/$(go env GOARCH)"
	mkdir -p "$BIN_DIR"
	go build -ldflags "$LDFLAGS" -o "$BIN_DIR/$APP_NAME" .
	log "wrote $BIN_DIR/$APP_NAME"
}

cmd_run() {
	[ -f "$LOCAL_CONFIG" ] || die "$LOCAL_CONFIG not found. Copy config.yaml to it and point it at your local redis."
	cmd_build
	log "running with $LOCAL_CONFIG (ctrl-c to stop)"
	exec "$BIN_DIR/$APP_NAME" -config "$LOCAL_CONFIG" "$@"
}

cmd_release() {
	cmd_check
	rm -rf "$DIST_DIR"
	mkdir -p "$DIST_DIR"

	local target os arch out
	for target in "${RELEASE_TARGETS[@]}"; do
		os="${target%/*}"
		arch="${target#*/}"
		out="$DIST_DIR/${APP_NAME}_${VERSION}_${os}_${arch}"

		log "building $os/$arch"
		mkdir -p "$out"
		CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
			go build -ldflags "$LDFLAGS" -o "$out/$APP_NAME" .
		cp config.yaml README.md "$out/"
		tar -czf "$out.tar.gz" -C "$DIST_DIR" "$(basename "$out")"
		rm -rf "$out"
	done

	log "artifacts:"
	ls -lh "$DIST_DIR"
}

# deb_version turns the git description into something dpkg will accept:
# it must start with a digit and use only alphanumerics and . + - : ~
deb_version() {
	local v="${VERSION#v}"
	v="${v%-dirty}"
	v="${v//-/+}"
	case "$v" in
		[0-9]*) ;;
		*) v="0.0.0+$v" ;;   # no tag to describe from, only a bare hash
	esac
	printf '%s' "$v"
}

cmd_deb() {
	command -v dpkg-deb >/dev/null 2>&1 \
		|| die "dpkg-deb not found. Debian packaging needs it (on macOS: brew install dpkg)."

	local arch="${1:-amd64}"
	case "$arch" in
		amd64|arm64) ;;
		*) die "unsupported deb architecture '$arch' (use amd64 or arm64)" ;;
	esac

	local version pkg_dir deb_file
	version="$(deb_version)"
	pkg_dir="$(mktemp -d)"
	deb_file="$DIST_DIR/${APP_NAME}_${version}_${arch}.deb"
	trap 'rm -rf "$pkg_dir"' RETURN
	# mktemp gives 0700; the package root must be world-readable like any other dir.
	chmod 755 "$pkg_dir"

	log "building $APP_NAME $version for linux/$arch"
	mkdir -p "$pkg_dir/DEBIAN" "$pkg_dir/usr/bin" "$pkg_dir/lib/systemd/system" "$DIST_DIR"
	CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
		go build -ldflags "$LDFLAGS" -o "$pkg_dir/usr/bin/$APP_NAME" .

	log "writing package metadata"
	cat > "$pkg_dir/DEBIAN/control" <<-EOF
		Package: $APP_NAME
		Version: $version
		Section: $DEB_SECTION
		Priority: $DEB_PRIORITY
		Architecture: $arch
		Maintainer: $DEB_MAINTAINER
		Description: $DEB_DESCRIPTION
	EOF

	# asynchook writes a default config on first start if none exists, so the
	# package deliberately does not ship /etc/$APP_NAME/config.yaml.
	cat > "$pkg_dir/lib/systemd/system/$APP_NAME.service" <<-EOF
		[Unit]
		Description=$APP_NAME Service
		After=network.target

		[Service]
		ExecStart=/usr/bin/$APP_NAME --config=/etc/$APP_NAME/config.yaml
		Restart=on-failure
		User=root
		Group=root

		[Install]
		WantedBy=multi-user.target
	EOF

	dpkg-deb --build --root-owner-group "$pkg_dir" "$deb_file" >/dev/null
	log "wrote $deb_file"
}

cmd_clean() {
	log "removing $BIN_DIR and $DIST_DIR"
	rm -rf "$BIN_DIR" "$DIST_DIR"
}

case "${1:-all}" in
	build)   cmd_build ;;
	run)     shift; cmd_run "$@" ;;
	check)   cmd_check ;;
	release) cmd_release ;;
	deb)     shift || true; cmd_deb "$@" ;;
	clean)   cmd_clean ;;
	all)     cmd_check; cmd_build ;;
	-h|--help|help)
		# print the header comment block, stopping at the first line of code
		awk 'NR>1 { if (/^#/) { sub(/^# ?/, ""); print } else { exit } }' "$0" ;;
	*)       die "unknown command '$1' (try: ./build.sh help)" ;;
esac
