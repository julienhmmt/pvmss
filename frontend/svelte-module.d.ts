declare module "*.svelte" {
  import type { ComponentType, SvelteComponentTyped } from "svelte";

  interface ModuleExports {
    [key: string]: any;
  }

  const component: ComponentType & SvelteComponentTyped;
  export default component;

  // Allow named exports from module scripts
  export * from "*.svelte#module";
}

declare module "*.svelte#module" {
  const module: Record<string, any>;
  export default module;
}

// Svelte 5 runes declarations - these are global in Svelte 5
declare const $state: {
  <T>(initial: T): T;
  <T>(): T;
};
declare const $derived: {
  <T>(fn: () => T): T;
};
declare const $effect: {
  (fn: () => void | (() => void)): void;
};
declare const $props: {
  <T>(): T;
};
declare const $bindable: {
  <T>(initial?: T): T;
};
