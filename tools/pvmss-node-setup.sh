#!/bin/sh
# pvmss-node-setup.sh - prepare one Proxmox VE node so PVMSS can publish
# admin cloud-init documents to it over SSH.
#
# Run as root on EVERY node of the cluster:
#
#   sh pvmss-node-setup.sh --storage local --key 'ssh-ed25519 AAAA... pvmss'
#
#   --storage ID   Proxmox storage whose snippets/ content receives the files
#                  (must have the "snippets" content type enabled). Default: local
#   --key KEY      PVMSS's public key, as shown in Admin > Clusters
#   --user NAME    dedicated system user. Default: pvmss
#
# What it does (idempotent):
#   1. installs /usr/local/bin/pvmss-snippet, the only thing PVMSS can run:
#      "write <name>" (content on stdin) and "remove <name>", where <name>
#      must match ^pvmss-[A-Za-z0-9._-]+\.ya?ml$ - no path, no shell;
#   2. writes /etc/pvmss-snippet.conf with the storage's snippets/ directory;
#   3. creates the user, gives it write access to that directory only (ACL);
#   4. installs the key with a forced command and every SSH feature disabled;
#   5. prints this node's host key line for Admin > Clusters > pinned host keys.
set -eu

STORAGE=local
KEY=""
USER_NAME=pvmss

while [ $# -gt 0 ]; do
	case "$1" in
	--storage) STORAGE="$2"; shift 2 ;;
	--key) KEY="$2"; shift 2 ;;
	--user) USER_NAME="$2"; shift 2 ;;
	*) echo "unknown option: $1" >&2; exit 2 ;;
	esac
done

[ "$(id -u)" -eq 0 ] || { echo "run as root" >&2; exit 1; }
[ -n "$KEY" ] || { echo "--key is required (copy it from Admin > Clusters)" >&2; exit 2; }
command -v pvesm >/dev/null || { echo "pvesm not found: run this on a Proxmox VE node" >&2; exit 1; }

if ! pvesm status --content snippets 2>/dev/null | awk 'NR>1 {print $1}' | grep -qx "$STORAGE"; then
	echo "storage '$STORAGE' does not have the snippets content type enabled on this node." >&2
	echo "Enable it (Datacenter > Storage > $STORAGE > Content: Snippets) and re-run." >&2
	exit 1
fi

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
if command -v setfacl >/dev/null; then
	setfacl -m "u:$USER_NAME:rwx" "$SNIPPET_DIR"
else
	echo "setfacl not found: install the 'acl' package" >&2
	exit 1
fi

# 4. key with forced command
HOME_DIR=$(getent passwd "$USER_NAME" | cut -d: -f6)
install -d -m 0700 -o "$USER_NAME" -g "$USER_NAME" "$HOME_DIR/.ssh"
printf 'restrict,command="/usr/local/bin/pvmss-snippet" %s\n' "$KEY" > "$HOME_DIR/.ssh/authorized_keys"
chown "$USER_NAME:$USER_NAME" "$HOME_DIR/.ssh/authorized_keys"
chmod 0600 "$HOME_DIR/.ssh/authorized_keys"

# 5. host key for Admin > Clusters
IP=$(hostname -I 2>/dev/null | awk '{print $1}')
echo "pvmss-snippet installed: $SNIPPET_DIR (storage $STORAGE), user $USER_NAME"
echo "Pinned host key line for this node (paste in Admin > Clusters, or use Scan host keys and compare):"
printf '%s %s\n' "${IP:-$(hostname)}" "$(cut -d' ' -f1,2 /etc/ssh/ssh_host_ed25519_key.pub)"
