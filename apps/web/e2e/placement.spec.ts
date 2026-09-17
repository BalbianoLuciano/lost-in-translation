import { expect, test, type Page } from '@playwright/test';
import { items, wrongTokenIndex } from './content';

// Usuario distinto por corrida: el login falso acepta cualquier uid, así que
// cada test arranca con el diagnóstico sin empezar.
function freshUser(page: Page, tag: string) {
	const uid = `e2e-${tag}-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
	return page.addInitScript(
		(user) => localStorage.setItem('lit-dev-user', JSON.stringify(user)),
		{ uid, email: `${uid}@dev.local`, name: 'E2E' }
	);
}

test('el login falso de desarrollo entra al tablero', async ({ page }) => {
	await page.goto('/');
	await page.getByRole('button', { name: /Enter/ }).click();
	await expect(page.getByRole('link', { name: /Verb tenses/ })).toBeVisible();
	await expect(page.getByText(/API ok/i)).toBeVisible();
});

test('el diagnóstico se completa y deja el mapa y los errores', async ({ page }, testInfo) => {
	await freshUser(page, testInfo.project.name);
	await page.goto('/');

	// El tablero muestra las tres partes sin empezar.
	await expect(page.getByRole('link', { name: /English at work/ })).toBeVisible();
	await page.locator('a[href="/placement/work"]').click();

	const wrongEvery = 3; // se erra a propósito 1 de cada 3 para probar el feedback
	let answered = 0;
	let wrongs = 0;

	while (true) {
		// Espera a que se pinte el próximo ejercicio o el resumen final.
		await page.locator('form.ejercicio, .resumen').first().waitFor({ state: 'visible' });
		if (await page.locator('.resumen').isVisible()) break;

		const id = await page.locator('form.ejercicio').getAttribute('data-item');
		const item = items[id!];
		expect(item, `ítem desconocido: ${id}`).toBeTruthy();

		const beWrong = answered % wrongEvery === wrongEvery - 1;

		if (item.type === 'cloze') {
			await page.locator('.hueco input').fill(beWrong ? 'zzz' : item.answers![0]);
		} else if (item.type === 'fix_error') {
			await page.locator('.token').nth(wrongTokenIndex(item)).click();
			await page.locator('.correccion input').fill(beWrong ? item.wrong! : item.corrections![0]);
		} else {
			const choice = beWrong ? (item.answer! + 1) % item.options!.length : item.answer!;
			await page.locator('.opcion').nth(choice).click();
		}
		await page.getByRole('button', { name: 'Check', exact: true }).click();

		const verdict = page.locator('.veredicto');
		await expect(verdict).toBeVisible();
		await expect(verdict).toHaveText(beWrong ? /Not quite|Right word/ : 'Correct.');
		// Ley 02: la regla se muestra siempre, bien o mal
		await expect(page.locator('.feedback .regla')).not.toBeEmpty();

		if (beWrong) wrongs++;
		answered++;
		expect(answered, 'demasiadas preguntas: el test no termina').toBeLessThan(40);

		await page.getByRole('button', { name: /Next|See results/ }).click();
	}

	// Resumen: mapa de habilidades y lista de errores
	await expect(page.getByRole('heading', { name: /Here's your map/ })).toBeVisible();
	await expect(page.locator('.resumen .cifras')).toBeVisible();
	await expect(page.locator('.errores li')).toHaveCount(wrongs);

	// La explicación en castellano se abre en el primer error
	await page.locator('.errores li').first().getByRole('button', { name: /Explicámelo/ }).click();
	await expect(page.locator('.errores .castellano')).toBeVisible();

	// El tablero refleja la parte terminada
	await page.getByRole('link', { name: /Back to the board/ }).click();
	await expect(page.locator('a[href="/placement/work"] .estado')).toContainText(/Done/i);

	// Volver a entrar muestra el resumen, no reinicia el test
	await page.locator('a[href="/placement/work"]').click();
	await expect(page.getByRole('heading', { name: /Placement done/ })).toBeVisible();
	await expect(page.locator('.errores li')).toHaveCount(wrongs);
});

test('el glosario se consulta desde un ejercicio y marca el intento', async ({ page }, testInfo) => {
	await freshUser(page, `gloss-${testInfo.project.name}`);
	await page.goto('/placement/tenses');
	await page.locator('form.ejercicio').waitFor();

	// El panel se abre desde el botón flotante y busca sin recargar nada
	await page.getByRole('button', { name: 'Abrir el glosario' }).click();
	const panel = page.getByRole('dialog', { name: 'Glossary' });
	await expect(panel).toBeVisible();

	await panel.getByRole('searchbox').fill('broke');
	await expect(panel.getByText('broken').first()).toBeVisible();

	// Una chuleta se abre y se vuelve
	await panel.getByRole('searchbox').fill('');
	await panel.getByRole('button', { name: /Conditionals/ }).click();
	await expect(panel.getByText(/If \+ present, will/)).toBeVisible();

	await page.keyboard.press('Escape'); // vuelve al glosario
	await page.keyboard.press('Escape'); // cierra el panel
	await expect(panel).toBeHidden();

	// Responder bien después de consultar: acierto, pero marcado
	const id = await page.locator('form.ejercicio').getAttribute('data-item');
	const item = items[id!];
	if (item.type === 'cloze') await page.locator('.hueco input').fill(item.answers![0]);
	else if (item.type === 'fix_error') {
		await page.locator('.token').nth(wrongTokenIndex(item)).click();
		await page.locator('.correccion input').fill(item.corrections![0]);
	} else await page.locator('.opcion').nth(item.answer!).click();
	await page.getByRole('button', { name: 'Check', exact: true }).click();

	await expect(page.locator('.veredicto')).toHaveText('Correct.');
	await expect(page.getByText(/cuenta como medio acierto/)).toBeVisible();
});
