/// <reference types="svelte" />
/// <reference types="vite/client" />

declare module "$app/environment" {
  export const dev: boolean;
  export const building: boolean;
  export const version: string;
}

declare module "$app/navigation" {
  import { goto, prefetch, preload } from "svelte-routing";
  export { goto, prefetch, preload };
}

declare module "$app/stores" {
  import { getStores, navigating, page, updated } from "svelte-routing";
  export { getStores, navigating, page, updated };
}
