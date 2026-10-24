#!/bin/sh
# Generate the SeaweedFS S3 identity file from the same credentials the backend
# and judge use. SeaweedFS disables S3 authentication when no config is supplied,
# so this one-shot runs before the gateway starts.
set -eu

key="${RUNCODES_S3_CREDENTIALS_KEY:?RUNCODES_S3_CREDENTIALS_KEY must be set}"
secret="${RUNCODES_S3_CREDENTIALS_SECRET:?RUNCODES_S3_CREDENTIALS_SECRET must be set}"

# Keep the JSON valid without pulling a JSON tool into a one-shot image. Both
# values are interpolated into JSON strings verbatim, so reject only the bytes
# that cannot appear there: control characters, the double quote and the
# backslash. Punctuation such as `&` or `*` is valid inside a JSON string and in
# an S3 credential, so hand-written development values are accepted too.
case "${key}${secret}" in
	*'"'* | *'\'*)
		echo "S3 credentials must not contain double quotes or backslashes" >&2
		exit 1
		;;
	*[![:print:]]*)
		echo "S3 credentials must be printable characters" >&2
		exit 1
		;;
esac

mkdir -p /config
printf '{"identities":[{"name":"runcodes","credentials":[{"accessKey":"%s","secretKey":"%s"}],"actions":["Admin","Read","Write","List","Tagging"]}]}' \
	"${key}" "${secret}" >/config/s3.json
chmod 0644 /config/s3.json
