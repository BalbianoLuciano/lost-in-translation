import { expect, test, type Page } from '@playwright/test';
import { items, wrongTokenIndex } from './content';

function freshUser(page: Page, tag: string) {
	const uid = `e2e-${tag}-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
	return page.addInitScript(
		(user) => localStorage.setItem('lit-dev-user', JSON.stringify(user)),
		{ uid, email: `${uid}@dev.local`, name: 'E2E' }
	);
}

/** Responde el ejercicio en pantalla, bien o mal. */
async function answer(page: Page, correct: boolean) {
	const id = await page.locator('form.ejercicio').getAttribute('data-item');
	const item = items[id!];
	expect(item, `ítem desconocido: ${id}`).toBeTruthy();

	if (item.type === 'cloze') {
		await page.locator('.hueco input').fill(correct ? item.answers![0] : 'zzz');
	} else if (item.type === 'fix_error') {
		await page.locator('.token').nth(wrongTokenIndex(item)).click();
		await page.locator('.correccion input').fill(correct ? item.corrections![0] : item.wrong!);
	} else {
		const pick = correct ? item.answer! : (item.answer! + 1) % item.options!.length;
		await page.locator('.opcion').nth(pick).click();
	}
	await page.getByRole('button', { name: 'Check', exact: true }).click();
	await expect(page.locator('.veredicto')).toBeVisible();
	return item;
}

test('la sesión diaria: lección, práctica y colada', async ({ page }, testInfo) => {
	await freshUser(page, `session-${testInfo.project.name}`);
	await page.goto('/');
	await expect(page.getByRole('heading', { name: /Hi,/ })).toBeVisible();

	// La práctica arranca trabada hasta leer la lección.
	await page.goto('/session?block=practice');
	await expect(page.getByRole('heading', { name: /Read the lesson first/ })).toBeVisible();

	// Desde el tablero se entra a la lección del día.
	await page.goto('/');
	const lessonLink = page.locator('a[href^="/lesson/"]').first();
	await expect(lessonLink).toBeVisible();
	await lessonLink.click();

	await expect(page.locator('main .titular')).toBeVisible();
	// Toda lección tiene un bloque de contraste: es el corazón de la lección
	await expect(page.locator('.pares li').first()).toBeVisible();

	// La explicación en castellano se abre a pedido
	await page.getByRole('button', { name: /Explicámelo/ }).first().click();
	await expect(page.locator('.castellano').first()).toBeVisible();

	await page.getByRole('button', { name: /I've read it/ }).click();

	// Cae directo en la práctica del tema
	await expect(page).toHaveURL(/block=practice/);
	await page.locator('form.ejercicio').waitFor();

	await answer(page, true);
	await expect(page.locator('.feedback .regla')).not.toBeEmpty();
	await page.getByRole('button', { name: /Next|See results/ }).click();

	await page.locator('form.ejercicio').waitFor();
	await answer(page, false);
	await expect(page.locator('.veredicto')).toHaveText(/Not quite|Right word/);
	await page.getByRole('button', { name: /Next|See results/ }).click();

	// El tablero refleja el avance del día
	await page.goto('/');
	await expect(page.getByText(/Colada [1-9]/)).toBeVisible();
	await expect(page.locator('a[href="/session?block=practice"] .estado')).toContainText('2/10');
});
