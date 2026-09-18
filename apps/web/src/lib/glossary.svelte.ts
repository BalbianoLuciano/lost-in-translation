import {
	api,
	ApiError,
	type Ask,
	type Cheatsheet,
	type Glossary,
	type SpellingRule,
	type Term,
	type Verb
} from '$lib/api';

const CACHE_KEY = 'lit-glossary';

export type Hit =
	| { kind: 'verb'; score: number; verb: Verb }
	| { kind: 'term'; score: number; term: Term }
	| { kind: 'rule'; score: number; rule: SpellingRule }
	| { kind: 'cheatsheet'; score: number; sheet: Cheatsheet };

const norm = (s: string) => s.toLowerCase().replace(/[‘’]/g, "'").trim();

/** Puntaje simple: empieza con lo buscado > lo contiene. 0 = no coincide. */
function match(query: string, fields: string[]): number {
	let best = 0;
	for (const f of fields) {
		const v = norm(f);
		if (v.startsWith(query)) best = Math.max(best, 3);
		else if (new RegExp(`\\b${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`).test(v)) best = Math.max(best, 2);
		else if (v.includes(query)) best = Math.max(best, 1);
	}
	return best;
}

/**
 * El glosario: se busca al instante y sin internet. Se descarga una vez por
 * versión de contenido y queda en localStorage.
 */
class GlossaryStore {
	data = $state<Glossary | null>(null);
	loading = $state(false);
	error = $state<string | null>(null);

	/** Panel abierto. */
	open = $state(false);
	/** Hay un ejercicio esperando respuesta en pantalla. */
	exerciseActive = $state(false);
	/** El ejercicio que se está respondiendo: es el contexto de las preguntas a la IA. */
	currentItemId = $state<string | null>(null);

	/** El chat con IA depende de que el servidor tenga la clave del proveedor. */
	aiEnabled = $state(false);
	asking = $state(false);
	answer = $state<Ask | null>(null);
	askError = $state<string | null>(null);
	/** Se consultó mientras había un ejercicio sin responder. */
	consultedNow = $state(false);

	#loaded = false;

	/** Pregunta lo que el glosario no cubre. El contexto lo pone el servidor. */
	async ask(question: string): Promise<void> {
		this.asking = true;
		this.askError = null;
		this.answer = null;
		try {
			this.answer = await api.ask({ question, itemId: this.currentItemId ?? undefined });
		} catch (err) {
			if (err instanceof ApiError) {
				this.askError =
					err.status === 429
						? 'Llegaste al límite de preguntas por hoy.'
						: err.status === 503
							? 'El chat todavía no está configurado en el servidor.'
							: `No se pudo preguntar (${err.status}).`;
			} else {
				this.askError = 'No se pudo contactar la API.';
			}
		} finally {
			this.asking = false;
		}
	}

	async load(): Promise<void> {
		if (this.#loaded || this.loading) return;
		this.loading = true;
		this.error = null;
		try {
			api
				.health()
				.then((h) => (this.aiEnabled = Boolean(h.ai)))
				.catch(() => (this.aiEnabled = false));
			const cached = this.#fromCache();
			if (cached) this.data = cached;
			const { contentVersion, glossary } = await api.glossary();
			this.data = glossary;
			this.#loaded = true;
			try {
				localStorage.setItem(CACHE_KEY, JSON.stringify({ contentVersion, glossary }));
			} catch {
				// sin storage: se vuelve a pedir la próxima vez
			}
		} catch {
			if (!this.data) this.error = 'No se pudo cargar el glosario.';
		} finally {
			this.loading = false;
		}
	}

	#fromCache(): Glossary | null {
		try {
			const raw = localStorage.getItem(CACHE_KEY);
			return raw ? (JSON.parse(raw).glossary as Glossary) : null;
		} catch {
			return null;
		}
	}

	/** Abre el panel. Si hay un ejercicio en curso, marca el intento como consultado. */
	show(): void {
		this.open = true;
		if (this.exerciseActive) this.consultedNow = true;
		void this.load();
	}

	hide(): void {
		this.open = false;
		this.answer = null;
		this.askError = null;
	}

	/** Se llama al pasar al siguiente ejercicio. */
	resetConsulted(): void {
		this.consultedNow = false;
	}

	search(raw: string): Hit[] {
		const q = norm(raw);
		if (!this.data || q.length < 2) return [];
		const hits: Hit[] = [];

		for (const verb of this.data.verbs ?? []) {
			const score = match(q, [verb.base, verb.past, verb.participle, verb.es]);
			if (score) hits.push({ kind: 'verb', score: score + 1, verb }); // los verbos pesan un poco más
		}
		for (const term of this.data.terms ?? []) {
			const score = match(q, [term.term, term.es]);
			if (score) hits.push({ kind: 'term', score, term });
		}
		for (const rule of this.data.rules ?? []) {
			const score = match(q, [rule.title_en, rule.when_es, ...rule.examples]);
			if (score) hits.push({ kind: 'rule', score, rule });
		}
		for (const sheet of this.data.cheatsheets ?? []) {
			const score = match(q, [
				sheet.title_en,
				sheet.title_es,
				sheet.summary_es,
				...sheet.rows.map((r) => `${r.name} ${r.form}`)
			]);
			if (score) hits.push({ kind: 'cheatsheet', score: score + 1, sheet });
		}

		return hits.sort((a, b) => b.score - a.score).slice(0, 40);
	}
}

export const glossary = new GlossaryStore();
