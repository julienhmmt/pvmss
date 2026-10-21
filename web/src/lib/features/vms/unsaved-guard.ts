import type { BeforeNavigate } from '$app/navigation';

/**
 * What to do with a navigation while an inline editor holds unsaved text:
 * - `allow`: nothing to lose, or only the query changes (tabs keep the draft);
 * - `native`: the page unloads, so only the browser's own prompt can ask;
 * - `confirm`: cancel it and ask in-app before replaying it.
 */
export type GuardDecision = 'allow' | 'native' | 'confirm';

export function guardDecision(
	navigation: Pick<BeforeNavigate, 'type' | 'to' | 'willUnload'>,
	dirty: boolean,
	currentPath: string
): GuardDecision {
	if (!dirty) return 'allow';
	if (navigation.willUnload) return 'native';
	if (navigation.to?.url.pathname === currentPath) return 'allow';
	return 'confirm';
}
