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

// Fix for svelte-i18n module recognition
declare module "svelte-i18n" {
  export const t: any;
  export const locale: any;
  export const setLocale: any;
  export const waitLocale: any;
  export const register: any;
  export const init: any;
  export const isLoading: any;
  export const locales: any;
  export const loadLocale: any;
  export const addMessages: any;
  export const hasLocale: any;
  export const removeLocale: any;
  export const getMessage: any;
  export const getLocale: any;
  export const dictionary: any;
  export const formats: any;
  export const missing: any;
  export const warn: any;
  export const fallbackLocale: any;
  export const loadingDelay: any;
  export const initialLocale: any;
}

// Fix for svelte-sonner module recognition
declare module "svelte-sonner" {
  export const toast: any;
  export const Toaster: any;
  export default any;
}

// svelteHTML namespace for HTML attributes in Svelte templates
declare namespace svelteHTML {
  interface HTMLAttributes<T> {
    [key: string]: any;
  }
}

// Svelte 5 runes declarations - these are global in Svelte 5
declare const $state: {
  <T>(initial: T): T;
  <T>(): T;
};
declare const $derived: {
  <T>(fn: () => T): T;
  <T>(value: T): T;
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
