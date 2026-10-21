import { getContext, setContext } from 'svelte';
import { SvelteMap } from 'svelte/reactivity';

/**
 * ToastVariant maps to the Layer B soft-surface tokens. `error` uses the
 * destructive ramp, `success` the success ramp, `info` the info ramp.
 */
export type ToastVariant = 'success' | 'error' | 'info';

export interface ToastEntry {
	/** Stable id so Svelte can keyed-each and animations can run on exit. */
	id: number;
	variant: ToastVariant;
	message: string;
	/** Auto-dismiss timeout in ms; 0 keeps it until dismissed manually. */
	duration: number;
}

export interface ToastInput {
	variant: ToastVariant;
	message: string;
	/** Override the default auto-dismiss (4s, 10s for errors); 0 = sticky. */
	duration?: number;
}

const DEFAULT_DURATION_MS = 4000;
/** Errors stay longer: they are often the only trace of a failed action. */
const ERROR_DURATION_MS = 10_000;

/**
 * ToastRegion - the global toast queue (FR-019). One instance per app shell,
 * provided through context (constitution VII: no module singletons). Callers
 * push toasts via `push()`; the <Toaster /> region renders and auto-dismisses
 * them. The queue is capped so a runaway emitter cannot flood the viewport.
 */
export class ToastRegion {
	items = $state.raw<readonly ToastEntry[]>([]);

	#nextId = 1;
	#timers = new SvelteMap<number, ReturnType<typeof setTimeout>>();
	/** Hard cap to prevent a flood from burying the viewport. */
	readonly #maxItems = 6;

	/** Pushes a toast onto the queue and schedules auto-dismiss. */
	push(input: ToastInput): number {
		const id = this.#nextId;
		this.#nextId += 1;
		const entry: ToastEntry = {
			id,
			variant: input.variant,
			message: input.message,
			duration: input.duration ?? (input.variant === 'error' ? ERROR_DURATION_MS : DEFAULT_DURATION_MS)
		};
		let next = [...this.items, entry];
		// Trim beyond the cap, oldest success/info first so an unread error survives.
		while (next.length > this.#maxItems) {
			const dropped = next.find((item) => item.variant !== 'error') ?? next[0];
			if (!dropped) break;
			this.#clearTimer(dropped.id);
			next = next.filter((item) => item !== dropped);
		}
		this.items = next;
		this.#schedule(entry);
		return id;
	}

	/** Stops the auto-dismiss while the user hovers or focuses the toast. */
	hold(id: number): void {
		this.#clearTimer(id);
	}

	/** Restarts the full auto-dismiss once the user leaves the toast. */
	release(id: number): void {
		const entry = this.items.find((item) => item.id === id);
		if (entry && !this.#timers.has(id)) this.#schedule(entry);
	}

	/** Convenience wrappers for the three variants. */
	success(message: string, duration?: number): number {
		return this.push({ variant: 'success', message, ...(duration !== undefined ? { duration } : {}) });
	}
	error(message: string, duration?: number): number {
		return this.push({ variant: 'error', message, ...(duration !== undefined ? { duration } : {}) });
	}
	info(message: string, duration?: number): number {
		return this.push({ variant: 'info', message, ...(duration !== undefined ? { duration } : {}) });
	}

	/** Removes a toast by id, clearing its auto-dismiss timer. */
	dismiss(id: number): void {
		this.#clearTimer(id);
		this.items = this.items.filter((item) => item.id !== id);
	}

	/** Clears all toasts - used on route teardown if ever needed. */
	clear(): void {
		for (const id of this.#timers.keys()) this.#clearTimer(id);
		this.items = [];
	}

	#schedule(entry: ToastEntry): void {
		if (entry.duration > 0) {
			this.#timers.set(entry.id, setTimeout(() => this.dismiss(entry.id), entry.duration));
		}
	}

	#clearTimer(id: number): void {
		const timer = this.#timers.get(id);
		if (timer !== undefined) {
			clearTimeout(timer);
			this.#timers.delete(id);
		}
	}
}

const TOAST_REGION_CONTEXT_KEY = Symbol('toast-region');

/** Called once, by the app shell layout. */
export function setToastContext(): ToastRegion {
	const region = new ToastRegion();
	setContext(TOAST_REGION_CONTEXT_KEY, region);
	return region;
}

export function getToastContext(): ToastRegion {
	return getContext<ToastRegion>(TOAST_REGION_CONTEXT_KEY);
}
