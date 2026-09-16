// Contraste WCAG 2.x entre dos colores hex opacos.

function channel(c: number): number {
	const s = c / 255;
	return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

export function luminance(hex: string): number {
	const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
	if (!m) throw new Error(`color hex inválido: ${hex}`);
	const n = parseInt(m[1], 16);
	const [r, g, b] = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map(channel);
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

export function contrast(a: string, b: string): number {
	const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
	return (hi + 0.05) / (lo + 0.05);
}

/** AAA para lo que se lee; AA para etiquetas y marcas cortas (design.md §4). */
export const AAA = 7;
export const AA = 4.5;
