import { ApiRequestError } from '$lib/types/api';

/**
 * Configuration options for the polling composable.
 */
export type PollingConfig = {
	/**
	 * The async function to execute on each polling interval.
	 */
	fn: () => Promise<void>;
	/**
	 * Base interval in milliseconds between successful polls. Defaults to 5000ms.
	 */
	baseInterval?: number;
	/**
	 * Maximum interval in milliseconds for exponential backoff. Defaults to 60000ms.
	 */
	maxInterval?: number;
	/**
	 * Multiplier for exponential backoff calculation. Must be positive. Defaults to 2.
	 */
	multiplier?: number;
	/**
	 * Maximum number of consecutive non-backoff errors before stopping polling.
	 * Defaults to 10.
	 */
	maxConsecutiveNonBackoffErrors?: number;
};

/**
 * Handle returned by usePolling for controlling polling behavior.
 */
export type PollingHandle = {
	/**
	 * Whether polling is currently active.
	 */
	readonly isPolling: boolean;
	/**
	 * Whether polling is paused due to tab visibility.
	 */
	readonly isPaused: boolean;
	/**
	 * Whether exponential backoff is currently active.
	 */
	readonly isBackedOff: boolean;
	/**
	 * Current backoff interval in milliseconds.
	 */
	readonly backoffInterval: number;
	/**
	 * Seconds until next retry (0 if not backed off).
	 */
	readonly nextRetryIn: number;
	/**
	 * Start or resume polling.
	 */
	start(): void;
	/**
	 * Stop polling and reset state. Can be restarted later.
	 */
	stop(): void;
	/**
	 * Stop polling and prevent future restarts.
	 */
	destroy(): void;
};

const DEFAULT_BASE = 5_000;
const DEFAULT_MAX = 60_000;
const DEFAULT_MULT = 2;
const DEFAULT_MAX_CONSECUTIVE_NON_BACKOFF = 10;
const COUNTDOWN_TICK_MS = 1_000;

/** Programming errors that should fail fast, not trigger backoff. */
const FAST_FAIL_ERRORS = [TypeError, ReferenceError, SyntaxError];

/**
 * Creates a polling handle with exponential backoff and smart error classification.
 *
 * Features:
 * - Exponential backoff for server errors (5xx)
 * - Fast-fail for programming errors (TypeError, ReferenceError, SyntaxError)
 * - Stops on authentication errors (401, 403)
 * - Limits consecutive non-backoff errors (4xx client errors)
 * - Pauses polling when tab is hidden (Page Visibility API)
 * - Respects backoff interval when resuming from hidden
 *
 * @param config - Polling configuration options
 * @returns Polling handle with state and control methods
 * @throws Error if multiplier is not positive
 */
export function usePolling(config: PollingConfig): PollingHandle {
	const base = config.baseInterval ?? DEFAULT_BASE;
	const max = config.maxInterval ?? DEFAULT_MAX;
	const mult = config.multiplier ?? DEFAULT_MULT;

	if (base <= 0) {
		throw new Error('baseInterval must be positive');
	}
	if (max <= 0) {
		throw new Error('maxInterval must be positive');
	}
	if (max < base) {
		throw new Error('maxInterval must be >= baseInterval');
	}
	if (mult <= 0) {
		throw new Error('multiplier must be positive');
	}

	let isPolling = $state(false);
	let isPaused = $state(false);
	let backoffCount = $state(0);
	let nextRetryIn = $state(0);
	let consecutiveNonBackoffErrors = $state(0);
	let destroyed = false;

	const backoffInterval = $derived(Math.min(base * Math.pow(mult, backoffCount), max));
	const isBackedOff = $derived(backoffCount > 0);

	let pollingTimer: ReturnType<typeof setTimeout> | null = null;
	let countdownTimer: ReturnType<typeof setInterval> | null = null;
	let countdownEndTime: number | null = null;
	let pauseTimestamp: number | null = null;
	let listenerAttached = false;

	const maxConsecutiveNonBackoffErrors = config.maxConsecutiveNonBackoffErrors ?? DEFAULT_MAX_CONSECUTIVE_NON_BACKOFF;

	if (maxConsecutiveNonBackoffErrors <= 0) {
		throw new Error('maxConsecutiveNonBackoffErrors must be positive');
	}

	function clearTimers() {
		if (pollingTimer !== null) {
			clearTimeout(pollingTimer);
			pollingTimer = null;
		}
		if (countdownTimer !== null) {
			clearInterval(countdownTimer);
			countdownTimer = null;
		}
	}

	function startCountdown(ms: number) {
		if (countdownTimer !== null) clearInterval(countdownTimer);
		countdownEndTime = Date.now() + ms;
		nextRetryIn = Math.ceil(ms / COUNTDOWN_TICK_MS);
		countdownTimer = setInterval(() => {
			const remaining = countdownEndTime ? Math.max(0, countdownEndTime - Date.now()) : 0;
			nextRetryIn = Math.ceil(remaining / COUNTDOWN_TICK_MS);
			if (remaining <= 0 && countdownTimer !== null) {
				clearInterval(countdownTimer);
				countdownTimer = null;
				countdownEndTime = null;
			}
		}, COUNTDOWN_TICK_MS);
	}

	function scheduleNext() {
		if (!isPolling || isPaused) return;
		const delay = backoffCount > 0 ? backoffInterval : base;
		if (countdownTimer !== null) {
			clearInterval(countdownTimer);
			countdownTimer = null;
		}
		if (backoffCount > 0) startCountdown(delay);
		pollingTimer = setTimeout(run, delay);
	}

	async function run() {
		if (!isPolling || isPaused) return;
		pollingTimer = null;

		try {
			await config.fn();
			backoffCount = 0;
			nextRetryIn = 0;
			consecutiveNonBackoffErrors = 0;
		} catch (err: unknown) {
			if (err instanceof ApiRequestError && (err.status === 401 || err.status === 403)) {
				console.error('Polling stopped due to authentication error:', err);
				stop();
				return;
			}
			if (shouldBackoff(err)) {
				backoffCount += 1;
				consecutiveNonBackoffErrors = 0;
			} else {
				consecutiveNonBackoffErrors += 1;
				if (consecutiveNonBackoffErrors >= maxConsecutiveNonBackoffErrors) {
					stop();
					return;
				}
			}
		}

		scheduleNext();
	}

	function shouldBackoff(err: unknown): boolean {
		if (err instanceof ApiRequestError) return err.status >= 500;
		if (FAST_FAIL_ERRORS.some((ctor) => err instanceof ctor)) return false;
		if (err instanceof Error && err.name === 'AbortError') return false;
		return true;
	}

	function onVisibilityChange() {
		if (document.hidden) {
			isPaused = true;
			pauseTimestamp = countdownEndTime ? Date.now() : null;
			clearTimers();
		} else {
			isPaused = false;
			if (isPolling) {
				// If in backoff, resume the backoff timer from remaining time
				// Otherwise, fetch immediately
				if (backoffCount > 0) {
					// Calculate remaining time from pause
					let remaining = backoffInterval;
					if (pauseTimestamp && countdownEndTime) {
						const elapsed = Date.now() - pauseTimestamp;
						remaining = Math.max(0, countdownEndTime - Date.now());
					}
					// Schedule next with remaining time
					if (remaining > 0) {
						startCountdown(remaining);
						pollingTimer = setTimeout(run, remaining);
					} else {
						// Backoff already expired, fetch immediately
						run();
					}
				} else {
					clearTimers();
					run();
				}
			}
			pauseTimestamp = null;
		}
	}

	function start() {
		if (isPolling || destroyed) return;
		isPolling = true;
		isPaused = false;
		if (typeof document !== 'undefined' && !listenerAttached) {
			document.addEventListener('visibilitychange', onVisibilityChange);
			listenerAttached = true;
		}
		run();
	}

	function stop() {
		isPolling = false;
		isPaused = false;
		backoffCount = 0;
		nextRetryIn = 0;
		consecutiveNonBackoffErrors = 0;
		clearTimers();
		if (typeof document !== 'undefined' && listenerAttached) {
			document.removeEventListener('visibilitychange', onVisibilityChange);
			listenerAttached = false;
		}
	}

	function destroy() {
		stop();
		destroyed = true;
	}

	return {
		get isPolling() {
			return isPolling;
		},
		get isPaused() {
			return isPaused;
		},
		get isBackedOff() {
			return isBackedOff;
		},
		get backoffInterval() {
			return backoffInterval;
		},
		get nextRetryIn() {
			return nextRetryIn;
		},
		start,
		stop,
		destroy
	};
}
