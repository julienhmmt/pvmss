import { describe, it, expect, vi, afterEach } from 'vitest';
import { reveal } from './reveal';

describe('reveal', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('leaves content visible and fires at once without IntersectionObserver', () => {
		vi.stubGlobal('IntersectionObserver', undefined);
		const node = document.createElement('div');
		const onShow = vi.fn();
		reveal(onShow)(node);
		expect(node.dataset.reveal).toBeUndefined();
		expect(onShow).toHaveBeenCalledOnce();
	});

	it('hides until the element intersects, then shows it once', () => {
		let fire: (entries: Partial<IntersectionObserverEntry>[]) => void = () => {};
		const disconnect = vi.fn();
		vi.stubGlobal(
			'IntersectionObserver',
			class {
				constructor(cb: typeof fire) {
					fire = cb;
				}
				observe() {}
				disconnect = disconnect;
			}
		);
		const node = document.createElement('div');
		const onShow = vi.fn();
		reveal(onShow)(node);
		expect(node.dataset.reveal).toBe('pending');
		fire([{ isIntersecting: false }]);
		expect(onShow).not.toHaveBeenCalled();
		fire([{ isIntersecting: true }]);
		expect(node.dataset.reveal).toBe('shown');
		expect(onShow).toHaveBeenCalledOnce();
		expect(disconnect).toHaveBeenCalled();
	});
});
