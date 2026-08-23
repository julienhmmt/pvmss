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

/**
 * Product capabilities shown on the home page and the /about page.
 */
export const CAPABILITIES: readonly Capability[] = [
	{
		id: 'lifecycle',
		title: () => m['capabilities.card.lifecycle.title'](),
		description: () => m['capabilities.card.lifecycle.description']()
	},
	{
		id: 'consoles',
		title: () => m['capabilities.card.consoles.title'](),
		description: () => m['capabilities.card.consoles.description']()
	},
	{
		id: 'cloudinit',
		title: () => m['capabilities.card.cloudinit.title'](),
		description: () => m['capabilities.card.cloudinit.description']()
	},
	{
		id: 'snapshots',
		title: () => m['capabilities.card.snapshots.title'](),
		description: () => m['capabilities.card.snapshots.description']()
	},
	{
		id: 'storage',
		title: () => m['capabilities.card.storage.title'](),
		description: () => m['capabilities.card.storage.description']()
	},
	{
		id: 'multiCluster',
		title: () => m['capabilities.card.multiCluster.title'](),
		description: () => m['capabilities.card.multiCluster.description']()
	}
];
