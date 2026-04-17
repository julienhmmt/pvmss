import { init, addMessages, locale } from "svelte-i18n";
import { browser } from "$app/environment";
import en from "./en.json";
import fr from "./fr.json";

// getLocaleFromNavigator is available at runtime but TypeScript types may not expose it
const getLocaleFromNavigator = (): string | null => {
  if (typeof window === "undefined") return null;
  // navigator.language is the first element of navigator.languages per spec
  // Use it directly instead of accessing languages[0] which could be undefined
  return window.navigator.language || null;
};

addMessages("en", en);
addMessages("fr", fr);

function getInitialLocale(): string {
  if (!browser) return "en";

  const stored = localStorage.getItem("pvmss_lang");
  if (stored === "fr" || stored === "en") return stored;

  const cookie = document.cookie.match(/pvmss_lang=(en|fr)/)?.[1];
  if (cookie) return cookie;

  const nav = getLocaleFromNavigator() ?? "en";
  return nav.startsWith("fr") ? "fr" : "en";
}

export function setLocale(lang: "en" | "fr") {
  locale.set(lang);
  if (browser) {
    localStorage.setItem("pvmss_lang", lang);
    document.cookie = `pvmss_lang=${lang};path=/;max-age=31536000;SameSite=Lax`;
  }
}

init({ fallbackLocale: "en", initialLocale: getInitialLocale() });
