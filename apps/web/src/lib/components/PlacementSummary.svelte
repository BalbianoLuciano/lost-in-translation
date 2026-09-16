<script lang="ts">
	import type { SkillState, SkillSummary } from '$lib/api';

	type Props = { summary: SkillSummary[] };
	let { summary }: Props = $props();

	const groups: { state: SkillState; label: string; hint: string }[] = [
		{ state: 'calzada', label: 'Calzada', hint: "You've got it. It sets in place." },
		{ state: 'suspendida', label: 'Suspendida', hint: 'Almost there. It floats until practice sets it.' },
		{ state: 'plano', label: 'Plano', hint: "Still on the drawing. We'll build it." }
	];

	const count = (state: SkillState) => summary.filter((s) => s.state === state).length;
</script>

<div class="resumen">
	<p class="cifras num">
		{#each groups as g (g.state)}
			<span class={g.state}><strong>{count(g.state)}</strong> {g.label}</span>
		{/each}
	</p>

	{#each groups as g (g.state)}
		{#if count(g.state) > 0}
			<section>
				<h3 class="etiqueta {g.state}">{g.label} · {count(g.state)}</h3>
				<p class="hint">{g.hint}</p>
				<ul>
					{#each summary.filter((s) => s.state === g.state) as s (s.skillId)}
						<li>
							<span>{s.nameEn}</span>
							<span class="etiqueta num">{s.correct}/{s.asked}</span>
						</li>
					{/each}
				</ul>
			</section>
		{/if}
	{/each}
</div>

<style>
	.resumen {
		display: grid;
		gap: 32px;
	}

	.cifras {
		display: flex;
		flex-wrap: wrap;
		gap: 8px 28px;
		margin: 0;
	}

	.cifras strong {
		font-size: 56px;
		font-weight: 800;
		letter-spacing: -0.04em;
		line-height: 1;
		margin-right: 6px;
	}

	section {
		display: grid;
		gap: 8px;
	}

	h3 {
		margin: 0;
	}

	.hint {
		margin: 0;
		color: var(--text-muted);
	}

	.calzada {
		color: var(--baranda);
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		border-top: 1px solid var(--line);
		max-width: 560px;
	}

	li {
		display: flex;
		justify-content: space-between;
		gap: 16px;
		padding: 10px 0;
		border-bottom: 1px solid var(--line-soft);
	}
</style>
