import type { Attachment } from 'svelte/attachments';

/**
 * reveal - scroll-in attachment for the landing page. The element is visible
 * by default: it is only hidden (`data-reveal="pending"`) once we know an
 * IntersectionObserver will show it again, so jsdom, old browsers,
 * reduced-motion and stop-motion users always get the content.
 */
export function reveal(onShow?: () => void): Attachment<HTMLElement> {
	return (node) => {
		// The footer's "Stop motion" (html[data-motion-off]) counts as reduced motion too.
		const reduced =
			(globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false) ||
			document.documentElement.hasAttribute('data-motion-off');
		if (typeof IntersectionObserver === 'undefined' || reduced) {
			onShow?.();
			return;
		}
		node.dataset.reveal = 'pending';
		const observer = new IntersectionObserver(
			(entries) => {
				if (!entries.some((entry) => entry.isIntersecting)) return;
				node.dataset.reveal = 'shown';
				onShow?.();
				observer.disconnect();
			},
			{ rootMargin: '0px 0px -15% 0px' }
		);
		observer.observe(node);
		return () => observer.disconnect();
	};
}
