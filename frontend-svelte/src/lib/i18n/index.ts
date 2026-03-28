import { init, addMessages, getLocaleFromNavigator, locale } from 'svelte-i18n';
import { browser } from '$app/environment';
import en from './en.json';
import fr from './fr.json';

addMessages('en', en);
addMessages('fr', fr);

function getInitialLocale(): string {
	if (!browser) return 'en';

	const stored = localStorage.getItem('pvmss_lang');
	if (stored === 'fr' || stored === 'en') return stored;

	const cookie = document.cookie.match(/pvmss_lang=(en|fr)/)?.[1];
	if (cookie) return cookie;

	const nav = getLocaleFromNavigator() ?? 'en';
	return nav.startsWith('fr') ? 'fr' : 'en';
}

export function setLocale(lang: 'en' | 'fr') {
	locale.set(lang);
	if (browser) {
		localStorage.setItem('pvmss_lang', lang);
		document.cookie = `pvmss_lang=${lang};path=/;max-age=31536000;SameSite=Lax`;
	}
}

init({ fallbackLocale: 'en', initialLocale: getInitialLocale() });
