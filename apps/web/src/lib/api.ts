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
	if (init.body) headers.set('Content-Type', 'application/json');

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

export type Health = { status: string; db: string; content?: string };

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
	placement: () => request<{ parts: PartView[] }>('/v1/placement/'),
	startPlacement: (part: string) =>
		request<RunState>(`/v1/placement/parts/${encodeURIComponent(part)}/runs`, { method: 'POST' }),
	answerPlacement: (
		runId: string,
		body: { itemId: string; response: ItemResponse; latencyMs: number }
	) =>
		request<AnswerResult>(`/v1/placement/runs/${encodeURIComponent(runId)}/answers`, {
			method: 'POST',
			body: JSON.stringify(body)
		})
};
