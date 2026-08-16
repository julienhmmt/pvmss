import { describe, it, expect } from 'vitest';
import { getErrorMessage } from './error-messages';

describe('getErrorMessage', () => {
	it('returns 404 message', () => {
		const message = getErrorMessage(404);
		expect(message.title()).toBe('Page introuvable');
		expect(message.description()).toContain('existe');
	});

	it('returns 403 message', () => {
		const message = getErrorMessage(403);
		expect(message.title()).toBe('Accès refusé');
		expect(message.description()).toBeTruthy();
	});

	it('returns server error message for 500, 502, 503', () => {
		for (const status of [500, 502, 503]) {
			const message = getErrorMessage(status);
			expect(message.title()).toBe('Erreur serveur');
			expect(message.description()).toBeTruthy();
		}
	});

	it('returns a generic message for unknown status codes', () => {
		const message = getErrorMessage(418);
		expect(message.title()).toContain('418');
		expect(message.description()).toBeTruthy();
	});
});
