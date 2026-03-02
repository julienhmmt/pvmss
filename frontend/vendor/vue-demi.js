// vue-demi shim for Vue 3 (no-build ESM importmap usage)
// pinia.esm-browser.prod.js imports from 'vue-demi' instead of 'vue'.
// For Vue 3, vue-demi is just a thin compat layer that re-exports vue.
export * from 'vue';
export const isVue2 = false;
export const isVue3 = true;
export const Vue2 = undefined;
export function install() {}
export function set(target, key, val) {
  if (Array.isArray(target)) {
    target.length = Math.max(target.length, key);
    target.splice(key, 1, val);
    return val;
  }
  target[key] = val;
  return val;
}
export function del(target, key) {
  if (Array.isArray(target)) {
    target.splice(key, 1);
    return;
  }
  delete target[key];
}
