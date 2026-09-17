import { describe, expect, it } from 'vitest';
import { glossary } from './glossary.svelte';
import type { Glossary } from './api';

const data: Glossary = {
	verbs: [
		{ base: 'break', past: 'broke', participle: 'broken', es: 'romper', example: 'The deploy broke the cache.' },
		{ base: 'run', past: 'ran', participle: 'run', es: 'correr, ejecutar', example: 'I ran the migration.' }
	],
	rules: [
		{ id: 'doble', title_en: 'Double the final consonant', when_es: 'Una sílaba…', examples: ['ship → shipped'] }
	],
	terms: [
		{ term: 'roll back', type: 'phrasal', es: 'volver atrás', example: 'We rolled back the release.' }
	],
	cheatsheets: [
		{
			id: 'conditionals',
			title_en: 'Conditionals',
			title_es: 'Condicionales',
			summary_es: 'Después de if no va will.',
			rows: [{ name: 'First', form: 'If + present, will + verb', use_es: 'Futuro real', example: 'If you approve…' }]
		}
	]
};

glossary.data = data;

describe('búsqueda del glosario', () => {
	it('encuentra un verbo por cualquiera de sus formas', () => {
		for (const q of ['break', 'broke', 'broken']) {
			const hit = glossary.search(q)[0];
			expect(hit.kind).toBe('verb');
			expect(hit.kind === 'verb' && hit.verb.base).toBe('break');
		}
	});

	it('encuentra un verbo buscando en castellano', () => {
		const hit = glossary.search('romper')[0];
		expect(hit.kind === 'verb' && hit.verb.base).toBe('break');
	});

	it('encuentra phrasal verbs y chuletas', () => {
		expect(glossary.search('roll back')[0].kind).toBe('term');
		expect(glossary.search('condicional')[0].kind).toBe('cheatsheet');
	});

	it('prioriza lo que empieza con lo buscado', () => {
		const first = glossary.search('run')[0];
		expect(first.kind === 'verb' && first.verb.base).toBe('run');
	});

	it('no busca con menos de dos letras', () => {
		expect(glossary.search('r')).toEqual([]);
	});

	it('devuelve vacío cuando no hay nada', () => {
		expect(glossary.search('zzzz')).toEqual([]);
	});
});

describe('marcado de consulta', () => {
	it('sólo marca cuando hay un ejercicio en curso', () => {
		glossary.exerciseActive = false;
		glossary.resetConsulted();
		glossary.show();
		expect(glossary.consultedNow).toBe(false);
		glossary.hide();

		glossary.exerciseActive = true;
		glossary.show();
		expect(glossary.consultedNow).toBe(true);
		glossary.hide();
		glossary.resetConsulted();
		expect(glossary.consultedNow).toBe(false);
	});
});
