import { describe, expect, it } from 'vitest';
import { PowerActionRegistry } from './power-actions.svelte';

describe('PowerActionRegistry', () => {
	it('tracks one action per machine and forgets it on end', () => {
		const registry = new PowerActionRegistry();
		registry.begin({ cluster: 'a', vmid: 1, name: 'web', action: 'start' });
		registry.begin({ cluster: 'b', vmid: 1, name: 'db', action: 'shutdown' });
		expect(registry.get('a', 1)).toBe('start');
		expect(registry.get('b', 1)).toBe('shutdown');
		expect(registry.size).toBe(2);

		registry.begin({ cluster: 'a', vmid: 1, name: 'web', action: 'reboot' });
		expect(registry.get('a', 1)).toBe('reboot');
		expect(registry.size).toBe(2);

		registry.end('a', 1);
		registry.end('a', 1);
		expect(registry.get('a', 1)).toBeNull();
		expect(registry.entries.map((entry) => entry.name)).toEqual(['db']);
	});
});
