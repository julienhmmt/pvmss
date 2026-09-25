import { describe, expect, it } from 'vitest';
import { nodeSetupCommand } from './node-setup-command';

describe('nodeSetupCommand', () => {
	const key = 'ssh-ed25519 AAAA pvmss';

	it.each([
		{
			name: 'uses the values given',
			input: { origin: 'https://pvmss.example', storage: 'snip', user: 'pvmss-snippets', port: 22, publicKey: key },
			url: 'https://pvmss.example/api/v1/pvmss-node-setup.sh',
			args: "--storage snip --user pvmss-snippets --key 'ssh-ed25519 AAAA pvmss'",
			local: false
		},
		{
			name: 'falls back to the script defaults and trims the origin',
			input: { origin: 'http://10.0.0.5:50000/', storage: ' ', user: '', port: 22, publicKey: key },
			url: 'http://10.0.0.5:50000/api/v1/pvmss-node-setup.sh',
			args: "--storage local --user pvmss --key 'ssh-ed25519 AAAA pvmss'",
			local: false
		},
		{
			name: 'passes a non-default port',
			input: { origin: 'https://pvmss.example:8443', storage: 'local', user: 'pvmss', port: 2222, publicKey: key },
			url: 'https://pvmss.example:8443/api/v1/pvmss-node-setup.sh',
			args: "--storage local --user pvmss --port 2222 --key 'ssh-ed25519 AAAA pvmss'",
			local: false
		},
		{
			name: 'flags a loopback origin',
			input: { origin: 'http://localhost:50000', storage: 'local', user: 'pvmss', port: 22, publicKey: key },
			url: 'http://localhost:50000/api/v1/pvmss-node-setup.sh',
			args: "--storage local --user pvmss --key 'ssh-ed25519 AAAA pvmss'",
			local: true
		},
		{
			name: "flags Vite's dev port",
			input: { origin: 'http://192.168.1.20:5173', storage: 'local', user: 'pvmss', port: 22, publicKey: key },
			url: 'http://192.168.1.20:5173/api/v1/pvmss-node-setup.sh',
			args: "--storage local --user pvmss --key 'ssh-ed25519 AAAA pvmss'",
			local: true
		}
	])('$name', ({ input, url, args, local }) => {
		const got = nodeSetupCommand(input);
		expect(got.scriptUrl).toBe(url);
		expect(got.command).toBe(`curl -fsSL ${url} | sh -s -- ${args}`);
		expect(got.originLooksLocal).toBe(local);
	});
});
