#!/usr/bin/env bash
#
# Smoke-test the hardened runner container contract on a real Podman host:
#
#   * the image filesystem is read-only;
#   * /tmp and /run are writable, size-capped tmpfs;
#   * the C runner compiles and executes a case through the real harness with
#     those flags, using only the workspace bind mount for persistence.
#
# This is intentionally a host-side test: the judge's Go tests can assert the
# spec, but only a container runtime proves that gcc and the harness still work
# with no writable image layer.
#
# Usage:
#   judge/test/rootfs-smoke.sh [IMAGE]
#
# Defaults to ghcr.io/runcodes-icmc/runcodes-runner-c:latest. Override with
# RUNCODES_SMOKE_IMAGE, SMOKE_TMPFS_SIZE (default 64m for the test), or PODMAN.
# Requires rootless Podman able to run the image; no network is given to the
# container.

set -euo pipefail

PODMAN="${PODMAN:-podman}"
IMAGE="${1:-${RUNCODES_SMOKE_IMAGE:-ghcr.io/runcodes-icmc/runcodes-runner-c:latest}}"
TMPFS_SIZE="${SMOKE_TMPFS_SIZE:-64m}"
MEMORY="${SMOKE_MEMORY:-1g}"
PIDS_LIMIT="${SMOKE_PIDS_LIMIT:-256}"
KEEP_WORKDIR="${KEEP_WORKDIR:-0}"

usage() {
  sed -n '3,21p' "$0" | sed 's/^# \{0,1\}//'
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if ! command -v "${PODMAN}" >/dev/null 2>&1; then
  echo "error: ${PODMAN} is not installed" >&2
  exit 1
fi

if ! "${PODMAN}" image exists "${IMAGE}" >/dev/null 2>&1; then
  cat >&2 <<EOF
error: image ${IMAGE} is not available locally.

Build it from this repository:
  podman build -t ghcr.io/runcodes-icmc/runcodes-monitor:latest monitor
  podman build -t ghcr.io/runcodes-icmc/runcodes-runner-base:latest runners/base
  podman build -t ghcr.io/runcodes-icmc/runcodes-runner-c:latest runners/c

Or pull the published image:
  podman pull ${IMAGE}
EOF
  exit 1
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/runcodes-rootfs-smoke.XXXXXX")"
cleanup() {
  if [[ "${KEEP_WORKDIR}" == "1" ]]; then
    echo "kept workspace: ${work}" >&2
  else
    rm -rf "${work}"
  fi
}
trap cleanup EXIT

# The container's non-root user is mapped to a subuid; a world-writable
# workspace is what the judge itself uses, and is safe in this disposable temp
# directory.
mkdir -p "${work}/src" "${work}/test_1"
printf '%s\n' \
  '#include <stdio.h>' \
  'int main(void) {' \
  '  int a, b;' \
  '  if (scanf("%d %d", &a, &b) != 2) return 1;' \
  '  printf("%d\n", a + b);' \
  '  return 0;' \
  '}' >"${work}/src/main.c"
printf '2 3\n' >"${work}/1.in"
printf '5\n' >"${work}/expected.out"
cat >"${work}/container.config" <<'EOF'
src_file='main.c'
run_nonce='rootfs-smoke'
t_1=5
ms_1=268435456
fs_1=5242880
stack_1=0
EOF
chmod -R 0777 "${work}"

common=(
  --rm
  --read-only
  --tmpfs "/tmp:rw,nosuid,nodev,size=${TMPFS_SIZE}"
  --tmpfs "/run:rw,nosuid,nodev,size=16m"
  --cap-drop=ALL
  --security-opt=no-new-privileges
  --network=none
  --memory="${MEMORY}"
  --pids-limit="${PIDS_LIMIT}"
  --volume "${work}:/root:rw,z"
  --workdir /root
)

echo "==> checking the read-only rootfs and tmpfs caps"
"${PODMAN}" run "${common[@]}" "${IMAGE}" /bin/sh -c '
set -eu
root_options="$(awk "\$2 == \"/\" { print \$4 }" /proc/mounts)"
case ",${root_options}," in
  *,ro,*) ;;
  *) echo "root filesystem is not read-only: ${root_options}" >&2; exit 1 ;;
esac
touch /tmp/smoke-writable /root/smoke-writable
if dd if=/dev/zero of=/tmp/smoke-fill bs=1M count=80 status=none 2>/dev/null; then
  echo "tmpfs accepted 80 MiB, so it is not capped at the requested size" >&2
  exit 1
fi
rm -f /tmp/smoke-writable /tmp/smoke-fill /root/smoke-writable
echo "root filesystem read-only; /tmp and /root writable"
'

echo "==> compiling and running a case through the harness"
"${PODMAN}" run "${common[@]}" "${IMAGE}" /usr/bin/runcodes

for report in compilation.monitor_out 1.monitor_out; do
  if ! grep -qx 'exit_status=0' "${work}/${report}"; then
    echo "error: ${report} does not report exit_status=0" >&2
    cat "${work}/${report}" >&2
    exit 1
  fi
done

if ! diff -u "${work}/expected.out" "${work}/1.output"; then
  echo "error: program output does not match the expected output" >&2
  exit 1
fi

if [[ -e "${work}/container.config" ]]; then
  echo "error: the harness did not remove container.config" >&2
  exit 1
fi

if find "${work}" -maxdepth 1 -name 'core*' -print -quit | grep -q .; then
  echo "error: a core file was written" >&2
  exit 1
fi

echo
echo "PASS: read-only rootfs, capped tmpfs, compile and run all work."
echo "Image: ${IMAGE}"
