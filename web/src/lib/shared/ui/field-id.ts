/**
 * field-id — stable unique id generator for form controls. A module-scoped
 * counter keeps ids short and deterministic per render pass. Prefer passing
 * an explicit id for prerendered pages (login, marketing) to avoid any
 * SSR/client hydration drift; the generator is a convenience for
 * authenticated, client-rendered forms.
 */
let counter = 0;

export function nextFieldId(prefix = 'field'): string {
	counter += 1;
	return `${prefix}-${counter}`;
}
