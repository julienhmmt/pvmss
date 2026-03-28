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
