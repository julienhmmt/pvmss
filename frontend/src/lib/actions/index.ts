export interface DebounceOptions {
	readonly handler: () => void;
	readonly delay?: number;
	readonly events?: readonly string[];
}

export function debounce(
	node: HTMLElement,
	options: DebounceOptions
): { update: (opts: DebounceOptions) => void; destroy: () => void } {
	let timer: ReturnType<typeof setTimeout> | null = null;
	let current = options;
	let events = current.events ?? ['input'];

	const run = (): void => {
		if (timer) clearTimeout(timer);
		timer = setTimeout(current.handler, current.delay ?? 300);
	};

	for (const event of events) {
		node.addEventListener(event, run);
	}
	return {
		update(opts: DebounceOptions): void {
			current = opts;
			const newEvents = current.events ?? ['input'];
			for (const event of events) {
				if (!newEvents.includes(event)) {
					node.removeEventListener(event, run);
				}
			}
			events = newEvents;
			for (const event of newEvents) {
				if (!events.includes(event)) {
					node.addEventListener(event, run);
				}
			}
		},
		destroy(): void {
			if (timer) clearTimeout(timer);
			for (const event of events) {
				node.removeEventListener(event, run);
			}
		}
	};
}

export function clickOutside(
	node: HTMLElement,
	callback: () => void
): { update: (cb: () => void) => void; destroy: () => void } {
	let current = callback;

	const handle = (event: MouseEvent): void => {
		if (!node.contains(event.target as Node)) current();
	};

	document.addEventListener('mousedown', handle, true);
	return {
		update(cb: () => void): void {
			current = cb;
		},
		destroy(): void {
			document.removeEventListener('mousedown', handle, true);
		}
	};
}

export function autofocus(node: HTMLElement): void {
	node.focus();
}

export interface IntersectOptions {
	readonly callback: (id: string) => void;
	readonly rootMargin?: string;
	readonly threshold?: number;
}

export function intersect(
	node: HTMLElement,
	options: IntersectOptions
): { update: (opts: IntersectOptions) => void; destroy: () => void } {
	let current = options;
	let observer: IntersectionObserver | null = null;

	const setup = (): void => {
		observer?.disconnect();
		observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting && entry.target.id) {
						current.callback(entry.target.id);
					}
				}
			},
			{
				rootMargin: current.rootMargin ?? '-150px 0px -80% 0px',
				threshold: current.threshold ?? 0
			}
		);
		observer.observe(node);
	};

	setup();
	return {
		update(opts: IntersectOptions): void {
			current = opts;
			setup();
		},
		destroy(): void {
			observer?.disconnect();
		}
	};
}
