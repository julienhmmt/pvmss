#!/bin/sh
# pvmss-node-setup.sh - prepare one Proxmox VE node so PVMSS can publish
# admin cloud-init documents to it over SSH.
#
# Run as root on EVERY node of the cluster. PVMSS serves this exact file
# (embedded in its binary) at /api/v1/pvmss-node-setup.sh, so a node that
# can reach PVMSS needs nothing else:
#
#   curl -fsSL https://<pvmss>/api/v1/pvmss-node-setup.sh \
#     | sh -s -- --storage local --user pvmss --key 'ssh-ed25519 AAAA... pvmss'
#
# Otherwise copy it from the repository (tools/pvmss-node-setup.sh) and run:
#
#   sh pvmss-node-setup.sh --storage local --user pvmss --key 'ssh-ed25519 AAAA... pvmss'
#
# The exact command, with PVMSS's key, is shown in Infrastructure > Clusters > Edit.
# Guide: docs/cloud-init-ssh.md. The copy embedded in the binary is
# server/internal/nodesetup/pvmss-node-setup.sh: edit this file, then copy
# it there (a test fails when the two differ).
#
#   --storage ID   Proxmox storage whose snippets/ content receives the files
#                  (must have the "snippets" content type enabled). Default: local
#                  A directory-backed storage (dir/local, nfs, cifs, cephfs).
#                  S3/object storage is not supported: Proxmox VE has no native
#                  S3 storage, and FUSE mounts or third-party S3 plugins do not
#                  guarantee the file is written and listed (docs/cloud-init-ssh.md,
#                  "Which storage?"). Use local.
#   --key KEY      PVMSS's public key, as shown in Infrastructure > Clusters:
#                  one line, ssh-ed25519, ecdsa-sha2-nistp256/384/521 or
#                  ssh-rsa (2048 bits minimum, 3072+ recommended)
#   --user NAME    dedicated system user (never root). Default: pvmss
#   --port N       SSH port PVMSS uses (Infrastructure > Clusters). Only used
#                  to print the pinned host key line. Default: 22
#
# What it does (idempotent):
#   1. installs /usr/local/bin/pvmss-snippet, the only thing PVMSS can run:
#      "write <name>" (content on stdin) and "remove <name>", where <name>
#      must match ^pvmss-[A-Za-z0-9._-]+\.ya?ml$ - no path, no shell;
#   2. writes /etc/pvmss-snippet.conf with the storage's snippets/ directory;
#   3. creates the user, gives it write access to that directory only (an ACL;
#      on storages without ACL support, e.g. many NFS/CIFS mounts, the
#      directory's group instead) and checks the user can write there;
#   4. installs the key with a forced command and every SSH feature disabled;
#   5. prints this node's host key line for Infrastructure > Clusters > pinned
#      host keys, under the node's address in /cluster/status (the one PVMSS
#      connects to).
set -eu

MIN_RSA_BITS=2048

die() {
	echo "pvmss-node-setup: $*" >&2
	exit 1
}

usage_error() {
	echo "pvmss-node-setup: $*" >&2
	exit 2
}

# parse_args sets STORAGE, KEY, USER_NAME and SSH_PORT.
parse_args() {
	STORAGE=local
	KEY=""
	USER_NAME=pvmss
	SSH_PORT=22
	while [ $# -gt 0 ]; do
		case "$1" in
		--storage | --key | --user | --port)
			[ $# -ge 2 ] || usage_error "$1 needs a value"
			case "$1" in
			--storage) STORAGE="$2" ;;
			--key) KEY="$2" ;;
			--user) USER_NAME="$2" ;;
			--port) SSH_PORT="$2" ;;
			esac
			shift 2
			;;
		*) usage_error "unknown option: $1" ;;
		esac
	done
}

# check_args rejects values that would break the setup or weaken it: a
# multi-line key would add unrestricted lines to authorized_keys.
check_args() {
	[ -n "$KEY" ] || usage_error "--key is required (copy it from Infrastructure > Clusters)"
	printf '%s' "$USER_NAME" | grep -Eq '^[a-z_][a-z0-9_-]{0,31}$' || usage_error "invalid --user '$USER_NAME'"
	[ "$USER_NAME" != root ] || usage_error "--user must not be root"
	printf '%s' "$SSH_PORT" | grep -Eq '^[0-9]{1,5}$' && [ "$SSH_PORT" -ge 1 ] && [ "$SSH_PORT" -le 65535 ] ||
		usage_error "invalid --port '$SSH_PORT'"
	printf '%s' "$STORAGE" | grep -Eq '^[A-Za-z][A-Za-z0-9._-]*$' || usage_error "invalid --storage '$STORAGE'"
	check_key
}

# check_key accepts exactly one public key line of a supported type,
# verified by ssh-keygen; RSA keys must have at least MIN_RSA_BITS bits.
check_key() {
	NL='
'
	CR=$(printf '\r')
	case "$KEY" in
	*"$NL"* | *"$CR"*) usage_error "--key must be a single line" ;;
	esac
	KEY_TYPE=$(printf '%s' "$KEY" | awk '{print $1}')
	case "$KEY_TYPE" in
	ssh-ed25519 | ssh-rsa | ecdsa-sha2-nistp256 | ecdsa-sha2-nistp384 | ecdsa-sha2-nistp521) ;;
	*) usage_error "unsupported key type '$KEY_TYPE' (use the key shown in Infrastructure > Clusters)" ;;
	esac
	command -v ssh-keygen >/dev/null || die "ssh-keygen not found: install openssh-client"
	KEY_TMP=$(mktemp)
	printf '%s\n' "$KEY" > "$KEY_TMP"
	KEY_BITS=$(ssh-keygen -l -f "$KEY_TMP" 2>/dev/null | awk '{print $1}') || true
	rm -f "$KEY_TMP"
	[ -n "$KEY_BITS" ] || usage_error "--key is not a valid SSH public key"
	if [ "$KEY_TYPE" = ssh-rsa ] && [ "$KEY_BITS" -lt "$MIN_RSA_BITS" ]; then
		usage_error "RSA key too short ($KEY_BITS bits, minimum $MIN_RSA_BITS; 3072+ recommended)"
	fi
}

check_node() {
	[ "$(id -u)" -eq 0 ] || die "run as root"
	command -v pvesm >/dev/null || die "pvesm not found: run this on a Proxmox VE node"
	if ! pvesm status --content snippets 2>/dev/null | awk 'NR>1 {print $1}' | grep -qx "$STORAGE"; then
		die "storage '$STORAGE' does not have the snippets content type enabled on this node.
Enable it (Datacenter > Storage > $STORAGE > Content: Snippets) and re-run."
	fi
}

# grant_access gives USER_NAME write access to SNIPPET_DIR: an ACL when the
# filesystem supports it, else the directory's group. Then proves it.
grant_access() {
	if command -v setfacl >/dev/null && setfacl -m "u:$USER_NAME:rwx" "$SNIPPET_DIR" 2>/dev/null; then
		ACCESS="ACL u:$USER_NAME:rwx"
	elif chgrp "$USER_NAME" "$SNIPPET_DIR" 2>/dev/null && chmod g+rwx "$SNIPPET_DIR" 2>/dev/null; then
		ACCESS="group $USER_NAME (no ACL support on this filesystem)"
	else
		die "cannot give '$USER_NAME' write access to $SNIPPET_DIR: no ACL support
and chgrp was refused (NFS root_squash?). Grant it on the storage server, or
use a local storage for snippets."
	fi
	su -s /bin/sh -c "test -w '$SNIPPET_DIR'" "$USER_NAME" ||
		die "'$USER_NAME' still cannot write $SNIPPET_DIR ($ACCESS): check the mount options and permissions"
}

install_key() {
	HOME_DIR=$(getent passwd "$USER_NAME" | cut -d: -f6)
	install -d -m 0700 -o "$USER_NAME" -g "$USER_NAME" "$HOME_DIR/.ssh"
	printf 'restrict,command="/usr/local/bin/pvmss-snippet" %s\n' "$KEY" > "$HOME_DIR/.ssh/authorized_keys"
	chown "$USER_NAME:$USER_NAME" "$HOME_DIR/.ssh/authorized_keys"
	chmod 0600 "$HOME_DIR/.ssh/authorized_keys"
}

# node_address prints this node's IP from /cluster/status (the address PVMSS
# connects to), else its first address.
node_address() {
	ADDR=$(pvesh get /cluster/status --output-format json 2>/dev/null |
		perl -MJSON -0ne 'for (@{decode_json($_)}) { print $_->{ip} if ($_->{type} // "") eq "node" && $_->{local} && $_->{ip} }' 2>/dev/null) || true
	[ -n "$ADDR" ] || ADDR=$(hostname -I 2>/dev/null | awk '{print $1}')
	[ -n "$ADDR" ] || ADDR=$(hostname)
	printf '%s' "$ADDR"
}

# print_host_key prints the known_hosts line PVMSS pins: the ed25519 host key
# (PVMSS's first choice), else ECDSA, else RSA, as "[addr]:port" off port 22.
print_host_key() {
	HOST=$(node_address)
	[ "$SSH_PORT" = 22 ] || HOST="[$HOST]:$SSH_PORT"
	for f in /etc/ssh/ssh_host_ed25519_key.pub /etc/ssh/ssh_host_ecdsa_key.pub /etc/ssh/ssh_host_rsa_key.pub; do
		if [ -r "$f" ]; then
			printf '%s %s\n' "$HOST" "$(cut -d' ' -f1,2 "$f")"
			return
		fi
	done
	echo "(no host key found in /etc/ssh - use Scan host keys in Infrastructure > Clusters)"
}

# Everything runs inside main, called on the last line: with
# `curl ... | sh -s --`, nothing executes until the whole file has arrived, so
# a truncated download cannot run half a setup.
main() {
parse_args "$@"
check_args
check_node

SNIPPET_DIR=$(dirname "$(pvesm path "$STORAGE:snippets/pvmss-probe.yml")")
mkdir -p "$SNIPPET_DIR"

# 1. helper
cat > /usr/local/bin/pvmss-snippet <<'HELPER'
#!/bin/sh
# pvmss-snippet - the only command PVMSS may run on this node (forced
# command). Installed by pvmss-node-setup.sh.
set -eu
umask 022
. /etc/pvmss-snippet.conf
MAX_BYTES=65536

# Forced command: the requested command is in SSH_ORIGINAL_COMMAND.
# shellcheck disable=SC2086
set -- ${SSH_ORIGINAL_COMMAND:-$*}
VERB="${1:-}"
NAME="${2:-}"

case "$VERB" in
check) echo "pvmss-snippet ok $SNIPPET_DIR"; exit 0 ;;
write | remove) ;;
*) echo "usage: write <name> | remove <name> | check" >&2; exit 2 ;;
esac

if [ $# -ne 2 ] || ! printf '%s' "$NAME" | grep -Eq '^pvmss-[A-Za-z0-9._-]+\.ya?ml$'; then
	echo "invalid snippet name" >&2
	exit 2
fi

case "$VERB" in
write)
	TMP=$(mktemp "$SNIPPET_DIR/.pvmss-XXXXXX")
	trap 'rm -f "$TMP"' EXIT
	head -c $((MAX_BYTES + 1)) > "$TMP"
	if [ "$(wc -c < "$TMP")" -gt "$MAX_BYTES" ]; then
		echo "snippet larger than $MAX_BYTES bytes" >&2
		exit 1
	fi
	chmod 0644 "$TMP"
	mv -f "$TMP" "$SNIPPET_DIR/$NAME"
	trap - EXIT
	;;
remove)
	rm -f "$SNIPPET_DIR/$NAME"
	;;
esac
HELPER
chmod 0755 /usr/local/bin/pvmss-snippet

# 2. config
printf 'SNIPPET_DIR=%s\n' "$SNIPPET_DIR" > /etc/pvmss-snippet.conf
chmod 0644 /etc/pvmss-snippet.conf

# 3. user + directory access
if ! id "$USER_NAME" >/dev/null 2>&1; then
	useradd --system --create-home --home-dir "/var/lib/$USER_NAME" --shell /bin/sh "$USER_NAME"
fi
grant_access

# 4. key with forced command
install_key

# 5. host key for Infrastructure > Clusters
echo "pvmss-snippet installed: $SNIPPET_DIR (storage $STORAGE), user $USER_NAME, access: $ACCESS"
echo "Pinned host key line for this node (paste in Infrastructure > Clusters, or use Scan host keys and compare):"
print_host_key
}

main "$@"
