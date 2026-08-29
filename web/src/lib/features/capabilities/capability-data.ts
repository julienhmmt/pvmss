import { m } from '$lib/paraglide/messages.js';

/**
 * Stable identifier for each PVMSS product capability.
 */
export type CapabilityId =
	| 'lifecycle'
	| 'consoles'
	| 'cloudinit'
	| 'snapshots'
	| 'storage'
	| 'multiCluster';

/**
 * A single product capability rendered as a card.
 */
export interface Capability {
	readonly id: CapabilityId;
	readonly title: () => string;
	readonly description: () => string;
}

type MessageFn = () => string;

const capabilityIds: readonly CapabilityId[] = [
	'lifecycle',
	'consoles',
	'cloudinit',
	'snapshots',
	'storage',
	'multiCluster'
];

const message = (id: CapabilityId, suffix: string): string =>
	(m[`capabilities.card.${id}.${suffix}` as keyof typeof m] as MessageFn)();

/**
 * Product capabilities shown on the home page and the /about page.
 */
export const CAPABILITIES: readonly Capability[] = capabilityIds.map((id) => ({
	id,
	title: () => message(id, 'title'),
	description: () => message(id, 'description')
}));
