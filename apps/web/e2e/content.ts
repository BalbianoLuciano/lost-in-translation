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

const bundle = JSON.parse(readFileSync(bundlePath, 'utf8')) as { items: Item[] };

export const items: Record<string, Item> = Object.fromEntries(
	bundle.items.map((i) => [i.id, i])
);

/** Índice de la palabra mal, con la misma tokenización que Go y Python. */
export function wrongTokenIndex(item: Item): number {
	const core = (t: string) => t.replace(/^[.,!?;:"()]+|[.,!?;:"()]+$/g, '').toLowerCase();
	return item.text.split(' ').findIndex((t) => core(t) === item.wrong!.toLowerCase());
}
