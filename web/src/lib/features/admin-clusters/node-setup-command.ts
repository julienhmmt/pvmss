/** Where PVMSS serves tools/pvmss-node-setup.sh (embedded in the binary). */
const SCRIPT_PATH = '/api/v1/pvmss-node-setup.sh';
const DEFAULT_SSH_PORT = 22;
/** Vite's dev server: nodes must use the backend port instead. */
const VITE_DEV_PORT = '5173';
const LOOPBACK_HOSTS: ReadonlySet<string> = new Set(['localhost', '127.0.0.1', '[::1]', '::1', '0.0.0.0']);

interface NodeSetupInput {
	/** PVMSS's public origin, as the admin's browser sees it. */
	readonly origin: string;
	readonly storage: string;
	readonly user: string;
	/** SSH port PVMSS uses; the script prints the host key line for it. */
	readonly port: number;
	/** PVMSS's SSH public key (authorized_keys form). */
	readonly publicKey: string;
}

interface NodeSetupCommand {
	/** URL of the script, to show it in a new tab. */
	readonly scriptUrl: string;
	/** One-liner an admin runs as root on every node. */
	readonly command: string;
	/** True when the origin is loopback or Vite's dev port: a node cannot use it as is. */
	readonly originLooksLocal: boolean;
}

function looksLocal(origin: string): boolean {
	try {
		const url = new URL(origin);
		return LOOPBACK_HOSTS.has(url.hostname) || url.port === VITE_DEV_PORT;
	} catch {
		return true;
	}
}

/**
 * nodeSetupCommand builds the command that downloads the node setup script
 * from PVMSS and runs it with this cluster's storage, user, port and PVMSS's
 * key. Empty storage/user and the default port fall back to the script's
 * defaults.
 */
export function nodeSetupCommand({ origin, storage, user, port, publicKey }: NodeSetupInput): NodeSetupCommand {
	const scriptUrl = `${origin.replace(/\/+$/, '')}${SCRIPT_PATH}`;
	const portArg = Number.isInteger(port) && port !== DEFAULT_SSH_PORT ? ` --port ${port}` : '';
	const args = `--storage ${storage.trim() || 'local'} --user ${user.trim() || 'pvmss'}${portArg} --key '${publicKey}'`;
	return { scriptUrl, command: `curl -fsSL ${scriptUrl} | sh -s -- ${args}`, originLooksLocal: looksLocal(origin) };
}
