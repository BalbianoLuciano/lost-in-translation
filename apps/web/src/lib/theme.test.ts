import { describe, expect, it } from 'vitest';
import { isThemePref, resolveTheme } from './theme';

describe('resolveTheme', () => {
	it('system sigue al sistema, con oscuro por defecto', () => {
		expect(resolveTheme('system', false)).toBe('dark');
		expect(resolveTheme('system', true)).toBe('light');
	});
	it('una elección explícita le gana al sistema', () => {
		expect(resolveTheme('dark', true)).toBe('dark');
		expect(resolveTheme('light', false)).toBe('light');
	});
});

describe('isThemePref', () => {
	it('acepta sólo los tres valores', () => {
		expect(isThemePref('system')).toBe(true);
		expect(isThemePref('sepia')).toBe(false);
		expect(isThemePref(null)).toBe(false);
	});
});
