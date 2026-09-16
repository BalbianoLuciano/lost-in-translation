import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { AA, AAA, contrast } from './contrast';

// "Si se puede chequear con un assert, no lo decide nadie a ojo."
// Lee los tokens reales de tokens.css y verifica la regla de legibilidad.

const css = readFileSync(new URL('../styles/tokens.css', import.meta.url), 'utf8');

function block(selector: string): Record<string, string> {
	const start = css.indexOf(selector);
	if (start === -1) throw new Error(`no está el bloque ${selector}`);
	const body = css.slice(css.indexOf('{', start) + 1, css.indexOf('}', start));
	return Object.fromEntries(
		[...body.matchAll(/--([\w-]+):\s*(#[0-9a-f]{6})\s*;/gi)].map((m) => [m[1], m[2]])
	);
}

const themes = {
	dark: block(":root[data-theme='dark']"),
	light: block(":root[data-theme='light']")
};

// [color, fondo, mínimo]
const rules: [string, string, number][] = [
	['text', 'bg', AAA],
	['text', 'surface', AAA],
	['text-muted', 'bg', AA],
	['text-muted', 'surface', AA],
	['baranda', 'bg', AA],
	['baranda', 'surface', AA],
	['oxido', 'bg', AA],
	['oxido', 'surface', AA]
];

describe('contraste de los tokens', () => {
	for (const [name, tokens] of Object.entries(themes)) {
		describe(`tema ${name}`, () => {
			for (const [fg, bg, min] of rules) {
				it(`${fg} sobre ${bg} ≥ ${min}:1`, () => {
					expect(tokens[fg], `falta --${fg}`).toBeDefined();
					expect(tokens[bg], `falta --${bg}`).toBeDefined();
					expect(contrast(tokens[fg], tokens[bg])).toBeGreaterThanOrEqual(min);
				});
			}
		});
	}

	it('la lectura larga en claro va sobre surface, que da AAA', () => {
		expect(contrast(themes.light.text, themes.light.surface)).toBeGreaterThanOrEqual(10);
	});
});

describe('contrast()', () => {
	it('negro sobre blanco es 21:1', () => {
		expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 5);
	});
	it('es simétrico', () => {
		expect(contrast('#15130f', '#a8a49b')).toBeCloseTo(contrast('#a8a49b', '#15130f'), 10);
	});
	it('rechaza colores que no son hex de 6 dígitos', () => {
		expect(() => contrast('red', '#ffffff')).toThrow();
	});
});
