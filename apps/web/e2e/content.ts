import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

/** El mismo banco que embebe la API: el test necesita las respuestas. */
export type Item = {
	id: string;
	skill: string;
	type: 'cloze' | 'choice' | 'explain_why' | 'fix_error';
	text: string;
	answers?: string[];
	options?: string[];
	answer?: number;
	wrong?: string;
	corrections?: string[];
};

const bundlePath = fileURLToPath(
	new URL('../../../services/api/internal/content/bundle.json', import.meta.url)
);

const bundle = JSON.parse(readFileSync(bundlePath, 'utf8')) as {
	items: Item[];
	skills: { id: string }[];
	obras: { pieces?: unknown[] }[];
};

/** Cuántas habilidades tiene el banco: el mapa dibuja una pieza por cada una. */
export const skillCount = bundle.skills.length;

export const items: Record<string, Item> = Object.fromEntries(
	bundle.items.map((i) => [i.id, i])
);

/** Índice de la palabra mal, con la misma tokenización que Go y Python. */
export function wrongTokenIndex(item: Item): number {
	const core = (t: string) => t.replace(/^[.,!?;:"()]+|[.,!?;:"()]+$/g, '').toLowerCase();
	return item.text.split(' ').findIndex((t) => core(t) === item.wrong!.toLowerCase());
}

export type Drill = {
	id: string;
	skill: string;
	context: string;
	prompt_en: string;
	expect: string[];
	avoid: string[];
};

/**
 * Los drills en el mismo orden en que los ofrece la API a un usuario nuevo:
 * por orden de currículum, y dentro de cada tema, en el orden del archivo.
 *
 * Se deriva del banco en vez de escribirse a mano porque ya nos mordió: agregar
 * drills de otro tema cambió cuál venía primero y el test se cayó por nombrar a
 * Sofía en una constante.
 */
export const drills: Drill[] = (() => {
	const todos = (bundle as unknown as { drills: Drill[] }).drills ?? [];
	const porTema = new Map<string, Drill[]>();
	for (const d of todos) {
		porTema.set(d.skill, [...(porTema.get(d.skill) ?? []), d]);
	}
	return bundle.skills.flatMap((s) => porTema.get(s.id) ?? []);
})();

/** El nombre en inglés de cada tema, como lo rotula la pantalla. */
export const skillNames: Record<string, string> = Object.fromEntries(
	(bundle as unknown as { skills: { id: string; name_en: string }[] }).skills.map((s) => [
		s.id,
		s.name_en
	])
);
