<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError, type Health, type MapObra, type Me, type PartView, type SessionState } from '$lib/api';
	import Pilar from '$lib/components/Pilar.svelte';
	import type { Piece } from '$lib/pilar';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import { session } from '$lib/session.svelte';
	import { applyThemePref, readStoredPref, type ThemePref } from '$lib/theme';

	let themePref = $state<ThemePref>(readStoredPref());
	let me = $state<Me | null>(null);
	let health = $state<Health | null>(null);
	let apiError = $state<string | null>(null);
	let parts = $state<PartView[]>([]);
	let today = $state<SessionState | null>(null);
	let obras = $state<MapObra[]>([]);
	let speaking = $state<{ enabled: boolean; left: number } | null>(null);

	// La obra que se ve en el tablero es la del tema que estás estudiando.
	const obra = $derived(
		obras.find((o) => o.id === today?.lesson?.obra) ?? obras.find((o) => o.pieces.length > 0) ?? null
	);
	const piezasObra = $derived<Piece[]>(
		obra
			? obra.pieces.flatMap((p) =>
					p.skills.map((s) => ({
						id: s.id,
						name: s.nameEn,
						state: s.state,
						mastery: s.mastery,
						items: s.items
					}))
				)
			: []
	);

	// En el tablero entra un tramo: el tema de hoy con sus vecinos. La obra
	// entera, en /map.
	const VENTANA = 5;
	const piezas = $derived.by(() => {
		if (piezasObra.length <= VENTANA) return piezasObra;
		const actual = piezasObra.findIndex((p) => p.id === today?.lesson?.skill);
		const centro = actual === -1 ? 0 : actual;
		const desde = Math.min(Math.max(centro - 2, 0), piezasObra.length - VENTANA);
		return piezasObra.slice(desde, desde + VENTANA);
	});

	const placementDone = $derived(parts.length > 0 && parts.every((p) => p.status === 'done'));
	const nextPart = $derived(parts.find((p) => p.status !== 'done'));

	$effect(() => {
		api
			.health()
			.then((h) => (health = h))
			.catch(() => (health = null));
	});

	$effect(() => {
		if (!session.user) {
			me = null;
			return;
		}
		syncProfile();
	});

	async function syncProfile() {
		apiError = null;
		try {
			const profile = await api.me();
			// El tema guardado en la cuenta manda, salvo que la cuenta siga en
			// "system" y en este dispositivo ya se haya elegido uno.
			if (profile.theme === 'system' && themePref !== 'system') {
				me = await api.updateSettings({ theme: themePref });
			} else {
				me = profile;
				setTheme(profile.theme, false);
			}
			parts = (await api.placement()).parts;
			today = await api.session();
			obras = (await api.map()).obras.filter((o) => o.pieces.length > 0);
			const sp = await api.speakingNext();
			speaking = { enabled: sp.enabled, left: sp.next ? sp.next.left + 1 : 0 };
		} catch (err) {
			apiError = err instanceof ApiError ? `API ${err.status}: ${err.message}` : 'API unreachable';
		}
	}

	async function setTheme(pref: ThemePref, persist = true) {
		themePref = pref;
		applyThemePref(pref);
		if (persist && session.user) {
			try {
				me = await api.updateSettings({ theme: pref });
			} catch {
				// el tema ya se aplicó local; se reintenta en el próximo cambio
			}
		}
	}
</script>

{#if !session.ready}
	<p class="etiqueta">Loading</p>
{:else if !session.user}
	<main class="entrada">
		<p class="etiqueta tapa">Lost in Translation · Obra 0</p>

		<h1 class="titular tapa">Stop translating.<br />Start building.</h1>

		<p class="bajada lectura tapa">
			One hour a day. Every answer comes with its rule, and when something doesn't click, the
			explanation is one tap away — in Spanish.
		</p>

		<div class="acciones tapa">
			{#if session.mode === 'unconfigured'}
				<p class="etiqueta aviso">
					Firebase is not configured. Set the PUBLIC_FIREBASE_* variables.
				</p>
			{:else}
				<button class="primario" type="button" onclick={() => session.signIn()}>
					{session.mode === 'dev' ? 'Enter · dev mode' : 'Sign in with Google'}
				</button>
			{/if}
			{#if session.error}
				<p class="aviso" role="alert">{session.error}</p>
			{/if}
		</div>
	</main>
{:else}
	<header class="cabecera tapa">
		<p class="etiqueta num">
			Jornal {today?.jornal ?? 0} · Colada {today?.coladaTotal ?? 0}
			{#if today?.lesson?.obraName}· Obra {today.lesson.obra} — {today.lesson.obraName}{/if}
		</p>
		<button class="etiqueta salir" type="button" onclick={() => session.signOut()}>Sign out</button>
	</header>

	<main class="tablero">
		<section class="tapa">
			<h1 class="titular">Hi, {me?.displayName?.split(' ')[0] || session.user.name || 'there'}.</h1>
			<p class="bajada lectura">
				{#if placementDone}
					Your foundation is poured. Daily sessions start with the next obra.
				{:else}
					First, the foundation: a placement test in three parts, so we don't spend time on what you
					already know. Do them in one sitting or across three days.
				{/if}
			</p>

			{#if piezas.length > 0}
				<div class="obra">
					<Pilar pieces={piezas} selected={today?.lesson?.skill} width={180} onselect={() => goto('/map')} />
					<a class="etiqueta ver" href="/map">
						{obra?.name} · {piezasObra.filter((p) => p.state === 'calzada').length}/{piezasObra.length} calzadas →
					</a>
				</div>
			{/if}

			<ol class="partes">
				{#each parts as p, i (p.id)}
					<li class:siguiente={p.id === nextPart?.id}>
						<a href="/placement/{p.id}">
							<span class="etiqueta num">0.{i + 1}</span>
							<span class="nombre">
								<strong>{p.nameEn}</strong>
								<span class="desc">{p.descriptionEn}</span>
							</span>
							<span class="etiqueta num estado">
								{#if p.status === 'done'}
									Done · {p.summary?.filter((s) => s.state === 'calzada').length ?? 0}/{p.skills} calzadas
								{:else if p.status === 'in_progress'}
									Continue · {p.progress?.answered}/{p.progress?.max}
								{:else}
									Start · {p.skills} skills
								{/if}
							</span>
						</a>
					</li>
				{/each}
			</ol>
		</section>

		<section class="sesion tapa" aria-labelledby="hoy">
			<h2 id="hoy" class="etiqueta">Today's session</h2>

			{#if !today?.lesson && !today?.review.due}
				<p class="vacio">Finish the placement test and the daily session starts here.</p>
			{:else}
				<ol class="bloques">
					<li class:siguiente={(today?.review.due ?? 0) > 0}>
						{#if (today?.review.due ?? 0) > 0}
							<a href="/session?block=review">
								<span class="nombre"><strong>Review</strong><span class="desc">What's due today</span></span>
								<span class="etiqueta num estado">{today?.review.due} items</span>
							</a>
						{:else}
							<span class="hecho">
								<span class="nombre"><strong>Review</strong><span class="desc">Nothing due</span></span>
								<span class="etiqueta num estado">Clear</span>
							</span>
						{/if}
					</li>

					{#if today?.lesson}
						<li class:siguiente={today.lesson.status !== 'done' && today.review.due === 0}>
							<a href="/lesson/{today.lesson.skill}">
								<span class="nombre">
									<strong>Lesson · {today.lesson.titleEn}</strong>
									<span class="desc">{today.lesson.goalEn}</span>
								</span>
								<span class="etiqueta num estado">
									{today.lesson.status === 'done' ? 'Read' : `${today.lesson.minutes} min`}
								</span>
							</a>
						</li>
					{/if}

					{#if speaking?.enabled}
						<li class:siguiente={speaking.left > 0 && today?.review.due === 0 && today?.lesson?.status === 'done'}>
							{#if speaking.left > 0}
								<a href="/speaking">
									<span class="nombre">
										<strong>Speaking</strong>
										<span class="desc">Say it out loud, against the clock</span>
									</span>
									<span class="etiqueta num estado">{speaking.left} drills</span>
								</a>
							{:else}
								<span class="hecho">
									<span class="nombre"><strong>Speaking</strong><span class="desc">Done for today</span></span>
									<span class="etiqueta num estado">Clear</span>
								</span>
							{/if}
						</li>
					{/if}

					{#if today?.practice}
						<li>
							{#if today.practice.locked}
								<span class="hecho">
									<span class="nombre">
										<strong>Practice · {today.practice.skillEn}</strong>
										<span class="desc">Read the lesson first</span>
									</span>
									<span class="etiqueta num estado">Locked</span>
								</span>
							{:else}
								<a href="/session?block=practice">
									<span class="nombre">
										<strong>Practice · {today.practice.skillEn}</strong>
										<span class="desc">Exercises on today's topic</span>
									</span>
									<span class="etiqueta num estado">{today.practice.done}/{today.practice.total}</span>
								</a>
							{/if}
						</li>
					{/if}
				</ol>
				<p class="etiqueta">Writing and listening come next</p>
			{/if}
		</section>
	</main>

	<footer class="pie tapa">
		<ThemeToggle value={themePref} onchange={(p) => setTheme(p)} />
		<p class="etiqueta num" aria-live="polite">
			{#if apiError}
				<span class="error">{apiError}</span>
			{:else if health}
				API {health.status} · DB {health.db}
			{:else}
				API unreachable
			{/if}
		</p>
	</footer>
{/if}

<style>
	.entrada {
		flex: 1;
		display: flex;
		flex-direction: column;
		justify-content: flex-end;
		gap: var(--module);
		padding-bottom: var(--module);
	}

	.entrada .titular {
		font-size: clamp(48px, 13vw, 112px);
	}

	.tablero .titular {
		font-size: clamp(40px, 8vw, 56px);
	}

	.bajada {
		margin: 0;
		font-size: 20px;
		line-height: 1.45;
		font-weight: 500;
	}

	.acciones {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 12px;
	}

	.primario {
		min-height: 56px;
		padding: 0 28px;
		background: var(--text);
		color: var(--bg);
		border: 0;
		font-weight: 600;
		cursor: pointer;
	}

	.primario:hover {
		background: var(--baranda);
	}

	.aviso,
	.error {
		color: var(--oxido);
		margin: 0;
	}

	.cabecera {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		padding-bottom: 12px;
		border-bottom: 1px solid var(--line);
	}

	.salir {
		min-height: 44px;
		white-space: nowrap;
		background: none;
		border: 0;
		cursor: pointer;
	}

	.tablero {
		flex: 1;
		display: grid;
		align-content: start;
		gap: calc(var(--module) * 2);
		padding-block: calc(var(--module) * 2);
	}

	.tablero > section {
		display: grid;
		gap: 16px;
	}

	.sesion {
		max-width: 520px;
	}

	.bloques {
		list-style: none;
		margin: 8px 0 0;
		padding: 0;
		border-top: 1px solid var(--line);
	}

	.bloques a,
	.bloques .hecho {
		display: grid;
		grid-template-columns: 1fr auto;
		gap: 4px 16px;
		align-items: baseline;
		padding: 16px 0;
		border-bottom: 1px solid var(--line-soft);
		text-decoration: none;
		color: inherit;
	}

	.bloques .hecho {
		color: var(--text-muted);
	}

	.bloques .nombre {
		display: grid;
		gap: 2px;
	}

	.bloques .desc {
		color: var(--text-muted);
	}

	.bloques a:hover strong {
		text-decoration: underline;
		text-underline-offset: 4px;
	}

	.bloques .siguiente strong,
	.bloques .siguiente .estado {
		color: var(--baranda);
	}

	.vacio {
		margin: 0;
		color: var(--text-muted);
	}


	.obra {
		display: grid;
		justify-items: center;
		gap: 8px;
		padding: var(--module) 0;
	}

	.ver {
		text-decoration: none;
		min-height: 44px;
		display: inline-flex;
		align-items: center;
	}

	.ver:hover {
		color: var(--baranda);
	}

	.partes {
		list-style: none;
		margin: 16px 0 0;
		padding: 0;
		border-top: 1px solid var(--line);
	}

	.partes a {
		display: grid;
		grid-template-columns: 36px 1fr;
		gap: 4px 12px;
		padding: 16px 0;
		border-bottom: 1px solid var(--line-soft);
		text-decoration: none;
		color: inherit;
	}

	.partes a:hover .nombre strong {
		text-decoration: underline;
		text-underline-offset: 4px;
	}

	.partes .nombre {
		display: grid;
		gap: 2px;
	}

	.partes .desc {
		color: var(--text-muted);
	}

	.partes .estado {
		grid-column: 2;
	}

	.partes .siguiente .estado,
	.partes .siguiente strong {
		color: var(--baranda);
	}

	@media (min-width: 900px) {
		.partes a {
			grid-template-columns: 48px 1fr auto;
			align-items: baseline;
		}
		.partes .estado {
			grid-column: 3;
		}
	}

	.pie {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		padding-top: 12px;
		border-top: 1px solid var(--line);
	}

	.pie p {
		margin: 0;
	}

	@media (min-width: 900px) {
		.tablero {
			grid-template-columns: 3fr 2fr;
			align-items: start;
		}
	}
</style>
