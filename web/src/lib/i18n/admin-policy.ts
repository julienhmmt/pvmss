import { adminPolicyCopy } from '$lib/features/admin-policy/copy';

function browserLocale(): 'en' | 'fr' {
	if (typeof navigator !== 'undefined' && navigator.language.toLowerCase().startsWith('fr')) return 'fr';
	return 'en';
}

/** Resolves the active browser locale and keeps document language accessible. */
export function resolveAdminPolicyCopy(): (typeof adminPolicyCopy)[keyof typeof adminPolicyCopy] {
	const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('pvmss-locale') : null;
	const locale: 'en' | 'fr' = stored === 'fr' || stored === 'en' ? stored : browserLocale();
	if (typeof document !== 'undefined') document.documentElement.lang = locale;
	return adminPolicyCopy[locale];
}
