declare module '*.svelte' {
	import type { ComponentType, SvelteComponentTyped } from 'svelte';
	
	interface ModuleExports {
		[key: string]: any;
	}
	
	const component: ComponentType & SvelteComponentTyped;
	export default component;
	
	// Allow named exports from module scripts
	export * from '*.svelte#module';
}

declare module '*.svelte#module' {
	const module: Record<string, any>;
	export default module;
}

// Svelte 5 runes declarations
declare function $state<T>(initial: T): T;
declare function $state<T>(): T;
declare function $derived<T>(fn: () => T): T;
declare function $effect(fn: () => void | (() => void)): void;
declare function $props<T>(): T;
declare function $bindable<T>(initial?: T): T;
