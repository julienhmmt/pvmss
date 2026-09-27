import { describe, expect, it } from 'vitest';
import { accountInitials } from './initials';

describe('accountInitials', () => {
	it.each([
		['alice', 'AL'],
		['Jane Doe', 'JD'],
		['jane.doe@corp', 'JD'],
		['x', 'X'],
		['  ', '?']
	])('%s -> %s', (name, initials) => {
		expect(accountInitials(name)).toBe(initials);
	});
});
