/// <reference types="svelte" />
/// <reference types="vite/client" />

declare module "*.svelte" {
  export { SvelteComponentDev as default } from "svelte/internal";
  export interface ComponentType {
    [key: string]: any;
  }
}

declare module "*.svelte#module" {
  import { SvelteComponentTyped } from "svelte";
  const component: SvelteComponentTyped;
  export default component;
}
