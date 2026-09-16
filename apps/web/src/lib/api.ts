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

export type Health = { status: string; db: string };

export const api = {
	health: () => request<Health>('/healthz'),
	me: () => request<Me>('/v1/me'),
	updateSettings: (settings: { theme: ThemePref }) =>
		request<Me>('/v1/me/settings', { method: 'PATCH', body: JSON.stringify(settings) })
};
