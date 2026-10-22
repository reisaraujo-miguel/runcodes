#!/bin/sh
# Generate the SeaweedFS S3 identity file from the same credentials the backend
# and judge use. SeaweedFS disables S3 authentication when no config is supplied,
# so this one-shot runs before the gateway starts.
set -eu

key="${RUNCODES_S3_CREDENTIALS_KEY:?RUNCODES_S3_CREDENTIALS_KEY must be set}"
secret="${RUNCODES_S3_CREDENTIALS_SECRET:?RUNCODES_S3_CREDENTIALS_SECRET must be set}"

# Keep the JSON valid without pulling a JSON tool into a one-shot image. The
# generated credentials are hex/base64; reject anything outside that alphabet
# rather than trying to escape it.
case "${key}${secret}" in
	*[!A-Za-z0-9+/=_-]*)
		echo "S3 credentials must be URL-safe base64/hex characters" >&2
		exit 1
		;;
esac

mkdir -p /config
printf '{"identities":[{"name":"runcodes","credentials":[{"accessKey":"%s","secretKey":"%s"}],"actions":["Admin","Read","Write","List","Tagging"]}]}' \
	"${key}" "${secret}" >/config/s3.json
chmod 0644 /config/s3.json
