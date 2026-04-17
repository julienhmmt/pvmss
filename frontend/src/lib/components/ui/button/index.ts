// @ts-nocheck - TypeScript cannot resolve Svelte module script exports, but build works correctly
import Root, {
  type ButtonProps,
  type ButtonSize,
  type ButtonVariant,
  buttonVariants,
} from "./button.svelte";

export {
  Root,
  type ButtonProps as Props,
  //
  Root as Button,
  buttonVariants,
  type ButtonProps,
  type ButtonSize,
  type ButtonVariant,
};
