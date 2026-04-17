// @ts-nocheck - TypeScript cannot resolve Svelte module script exports, but build works correctly
import Root, {
  toggleVariants,
  type ToggleSize,
  type ToggleVariant,
  type ToggleVariants,
} from "./toggle.svelte";

export {
  Root,
  //
  Root as Toggle,
};
