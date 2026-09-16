// Preferencia de tema. "system" sigue al sistema operativo; el valor resuelto
// siempre es dark o light y vive en <html data-theme>.
// app.html repite resolveTheme en línea para pintar sin parpadeo: si cambia acá,
// cambia allá.

export type ThemePref = 'system' | 'dark' | 'light';
export type Theme = 'dark' | 'light';

export const THEME_KEY = 'lit-theme';
export const THEME_PREFS: readonly ThemePref[] = ['system', 'dark', 'light'];

export function isThemePref(v: unknown): v is ThemePref {
	return typeof v === 'string' && (THEME_PREFS as readonly string[]).includes(v);
}

export function resolveTheme(pref: ThemePref, systemPrefersLight: boolean): Theme {
	if (pref === 'system') return systemPrefersLight ? 'light' : 'dark';
	return pref;
}

export function readStoredPref(): ThemePref {
	try {
		const v = localStorage.getItem(THEME_KEY);
		return isThemePref(v) ? v : 'system';
	} catch {
		return 'system';
	}
}

const lightQuery = () => window.matchMedia('(prefers-color-scheme: light)');

export function applyThemePref(pref: ThemePref): void {
	try {
		localStorage.setItem(THEME_KEY, pref);
	} catch {
		// sin storage (modo privado): el tema dura lo que la pestaña
	}
	document.documentElement.dataset.theme = resolveTheme(pref, lightQuery().matches);
}

/** Si la preferencia es "system", sigue los cambios del sistema en vivo. */
export function watchSystemTheme(getPref: () => ThemePref): () => void {
	const mq = lightQuery();
	const onChange = () => {
		if (getPref() === 'system') {
			document.documentElement.dataset.theme = resolveTheme('system', mq.matches);
		}
	};
	mq.addEventListener('change', onChange);
	return () => mq.removeEventListener('change', onChange);
}
