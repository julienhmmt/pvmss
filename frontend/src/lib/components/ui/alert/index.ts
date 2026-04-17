// @ts-nocheck - TypeScript cannot resolve Svelte module script exports, but build works correctly
import Root from "./alert.svelte";
import Description from "./alert-description.svelte";
import Title from "./alert-title.svelte";

export { alertVariants, type AlertVariant } from "./alert.svelte";

export {
  Root,
  Description,
  Title,
  //
  Root as Alert,
  Description as AlertDescription,
  Title as AlertTitle,
};
