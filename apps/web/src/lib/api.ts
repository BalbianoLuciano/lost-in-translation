import { API_URL } from '$lib/config';
import { session } from '$lib/session.svelte';
import type { ThemePref } from '$lib/theme';

export class ApiError extends Error {
	constructor(
		readonly status: number,
		message: string
	) {
		super(message);
	}
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
	const headers = new Headers(init.headers);
	const token = await session.token();
	if (token) headers.set('Authorization', `Bearer ${token}`);
	// Con FormData el navegador pone el boundary: si lo pisamos, el server no lo parsea.
	if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json');

	const res = await fetch(`${API_URL}${path}`, { ...init, headers });
	const body = await res.json().catch(() => ({}));
	if (!res.ok) {
		throw new ApiError(res.status, (body as { error?: string }).error ?? res.statusText);
	}
	return body as T;
}

export type Me = {
	id: string;
	email: string;
	displayName: string;
	theme: ThemePref;
};

export type Health = {
	status: string;
	db: string;
	content?: string;
	ai?: boolean;
	speaking?: boolean;
};

// ── Práctica oral ──

export type Drill = {
	id: string;
	skill: string;
	skillEn: string;
	kind: 'pronouns' | 'free';
	seconds: number;
	context: string;
	prompt_en: string;
	hint_es?: string;
	expect?: string[];
	avoid?: string[];
	left: number;
};

export type SpeakingResult = {
	transcript: string;
	words: number;
	used: Record<string, number>;
	missing: string[];
	wrong: string[];
	correct: boolean;
	tooShort: boolean;
	drillId: string;
	hintEs?: string;
	colada: number;
	next: Drill | null;
};

/** Respuesta del profesor de IA. */
export type Ask = { answer: string; cached: boolean; left: number };

// ── Contenido y test de ubicación (services/api/internal/placement) ──

export type ItemType = 'cloze' | 'choice' | 'explain_why' | 'fix_error';

export type PublicItem = {
	id: string;
	skill: string;
	type: ItemType;
	difficulty: number;
	context?: string;
	text: string;
	options?: string[];
	focus?: string;
	question?: string;
	tokens?: string[];
};

export type ItemResponse = { text?: string; choice?: number; tokenIndex?: number };

export type SkillState = 'plano' | 'suspendida' | 'calzada' | 'oxidada';

export type SkillSummary = {
	skillId: string;
	nameEn: string;
	nameEs: string;
	piece: string;
	asked: number;
	correct: number;
	done: boolean;
	state: SkillState;
	mastery: number;
};

export type ExplainEs = { rule: string; analogy: string; why: string };

// ── Glosario (se consulta, no se practica) ──

export type Verb = {
	base: string;
	past: string;
	participle: string;
	es: string;
	example: string;
	note_es?: string;
};

export type SpellingRule = {
	id: string;
	title_en: string;
	when_es: string;
	examples: string[];
	note_es?: string;
};

export type Term = {
	term: string;
	type: 'term' | 'chunk' | 'phrasal' | 'false_friend';
	es: string;
	example: string;
	note_es?: string;
};

export type Cheatsheet = {
	id: string;
	title_en: string;
	title_es: string;
	summary_es: string;
	rows: { name: string; form: string; use_es: string; example: string }[];
	notes_es?: string[];
	skills?: string[];
};

/** Un verbo regular: lo que importa es cómo suena su pasado. */
export type RegularVerb = {
	base: string;
	past: string;
	sound: 't' | 'd' | 'id';
	es: string;
	example: string;
	note_es?: string;
};

export type Glossary = {
	verbs: Verb[];
	regular_verbs: RegularVerb[];
	rules: SpellingRule[];
	terms: Term[];
	cheatsheets: Cheatsheet[];
};

export type Progress = { answered: number; max: number };

/** Un ítem que erraste, con todo lo necesario para repasarlo. */
export type Missed = {
	itemId: string;
	skillId: string;
	skillName: string;
	text: string;
	question?: string;
	expected: string;
	rule: string;
	explainEs: ExplainEs;
};

export type RunState = {
	runId: string;
	part: string;
	partName: string;
	status: 'in_progress' | 'done';
	next: PublicItem | null;
	nextSkill?: { id: string; nameEn: string; nameEs: string };
	progress: Progress;
	summary?: SkillSummary[];
	missed?: Missed[];
};

export type AnswerResult = {
	correct: boolean;
	expected: string;
	wrongIndex?: number;
	partOk?: boolean;
	itemId: string;
	rule: string;
	explainEs: ExplainEs;
	state: RunState;
};

// ── El mapa: obras, piezas y cómo estás en cada tema ──

export type MapSkill = {
	id: string;
	nameEn: string;
	nameEs: string;
	state: SkillState;
	mastery: number;
	items: number;
	hasLesson: boolean;
};

export type MapPiece = { id: string; nameEn: string; nameEs: string; skills: MapSkill[] };

export type MapObra = {
	id: number;
	slug: string;
	name: string;
	topicEn: string;
	topicEs: string;
	pieces: MapPiece[];
};

// ── Sesión diaria (services/api/internal/session) ──

export type SessionBlock = 'review' | 'practice';

export type LessonRef = {
	skill: string;
	skillEn: string;
	titleEn: string;
	goalEn: string;
	minutes: number;
	status: 'not_started' | 'in_progress' | 'done';
	obra: number;
	obraName: string;
};

export type PracticeRef = {
	skill: string;
	skillEn: string;
	done: number;
	total: number;
	locked: boolean;
};

export type SessionState = {
	date: string;
	jornal: number;
	coladaToday: number;
	coladaTotal: number;
	minutesToday: number;
	review: { due: number; done: number };
	lesson: LessonRef | null;
	practice: PracticeRef | null;
};

export type NextItem = {
	block: SessionBlock;
	skill: string;
	skillEn: string;
	item: PublicItem;
	left: number;
};

export type SessionAnswer = {
	correct: boolean;
	expected: string;
	wrongIndex?: number;
	partOk?: boolean;
	itemId: string;
	rule: string;
	explainEs: ExplainEs;
	colada: number;
	state: SessionState;
	next: NextItem | null;
};

export type LessonPair = { a: string; b: string; difference_es: string };

export type LessonBlock = {
	kind: 'idea' | 'form' | 'contrast' | 'trap' | 'chunks';
	title_en: string;
	body_en: string;
	body_es?: string;
	examples?: string[];
	pairs?: LessonPair[];
};

export type LessonView = {
	skill: string;
	skillEn: string;
	title_en: string;
	goal_en: string;
	minutes: number;
	blocks: LessonBlock[];
	cheatsheets?: string[];
	status: 'not_started' | 'in_progress' | 'done';
};

export type PartView = {
	id: string;
	nameEn: string;
	nameEs: string;
	descriptionEn: string;
	skills: number;
	status: 'not_started' | 'in_progress' | 'done';
	runId?: string;
	progress?: Progress;
	summary?: SkillSummary[];
	missed?: Missed[];
};

export const api = {
	health: () => request<Health>('/healthz'),
	me: () => request<Me>('/v1/me'),
	updateSettings: (settings: { theme: ThemePref }) =>
		request<Me>('/v1/me/settings', { method: 'PATCH', body: JSON.stringify(settings) }),
	glossary: () => request<{ contentVersion: string; glossary: Glossary }>('/v1/glossary'),
	ask: (body: { question: string; itemId?: string }) =>
		request<Ask>('/v1/ask', { method: 'POST', body: JSON.stringify(body) }),
	map: () => request<{ contentVersion: string; obras: MapObra[] }>('/v1/map'),
	session: () => request<SessionState>('/v1/session/'),
	speakingNext: () => request<{ next: Drill | null; enabled: boolean }>('/v1/speaking/next'),
	speakingAnswer: (drillId: string, audio: Blob, seconds: number) => {
		const form = new FormData();
		const ext = audio.type.includes('mp4') ? 'mp4' : 'webm';
		form.append('audio', audio, `drill.${ext}`);
		form.append('seconds', String(seconds));
		return request<SpeakingResult>(`/v1/speaking/${encodeURIComponent(drillId)}/answers`, {
			method: 'POST',
			body: form
		});
	},
	sessionNext: (block: SessionBlock) =>
		request<{ next: NextItem | null }>(`/v1/session/next?block=${block}`),
	sessionAnswer: (body: {
		block: SessionBlock;
		itemId: string;
		response: ItemResponse;
		latencyMs: number;
		consulted: boolean;
	}) => request<SessionAnswer>('/v1/session/answers', { method: 'POST', body: JSON.stringify(body) }),
	lesson: (skill: string) => request<LessonView>(`/v1/lessons/${encodeURIComponent(skill)}`),
	completeLesson: (skill: string) =>
		request<SessionState>(`/v1/lessons/${encodeURIComponent(skill)}/complete`, { method: 'POST' }),
	placement: () => request<{ parts: PartView[] }>('/v1/placement/'),
	startPlacement: (part: string) =>
		request<RunState>(`/v1/placement/parts/${encodeURIComponent(part)}/runs`, { method: 'POST' }),
	answerPlacement: (
		runId: string,
		body: { itemId: string; response: ItemResponse; latencyMs: number; consulted: boolean }
	) =>
		request<AnswerResult>(`/v1/placement/runs/${encodeURIComponent(runId)}/answers`, {
			method: 'POST',
			body: JSON.stringify(body)
		})
};
