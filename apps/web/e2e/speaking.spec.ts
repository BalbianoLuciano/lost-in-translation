import { expect, test, type Page } from '@playwright/test';

function freshUser(page: Page, tag: string) {
	const uid = `e2e-${tag}-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
	return page.addInitScript(
		(user) => localStorage.setItem('lit-dev-user', JSON.stringify(user)),
		{ uid, email: `${uid}@dev.local`, name: 'E2E' }
	);
}

test('un drill oral se graba, se transcribe y se corrige', async ({ page }, testInfo) => {
	await freshUser(page, `speak-${testInfo.project.name}`);
	await page.goto('/speaking');

	// El primer drill es el de Sofía: hay que hablar de ella en tercera persona
	await expect(page.getByText('He, she, they')).toBeVisible();
	await expect(page.locator('.consigna')).toContainText('Sofía');

	// La pista está a pedido, como las explicaciones en castellano
	await page.getByRole('button', { name: /Dame una pista/ }).click();
	await expect(page.locator('.pista')).toBeVisible();

	// Grabar, esperar un momento y cortar
	await page.getByRole('button', { name: 'Grabar' }).click();
	await expect(page.getByRole('button', { name: /Parar la grabación/ })).toBeVisible();
	await page.waitForTimeout(1200);
	await page.getByRole('button', { name: /Parar la grabación/ }).click();

	// La transcripción de mentira dice "she", que es lo que el drill pedía
	const veredicto = page.locator('.veredicto');
	await expect(veredicto).toBeVisible({ timeout: 30000 });
	await expect(veredicto).toHaveText('Correct.');
	await expect(page.locator('.transcripcion')).toContainText('She found the bug');
	await expect(page.getByText(/\+5 colada/)).toBeVisible();

	// Y sigue con el próximo
	await page.getByRole('button', { name: /Next drill/ }).click();
	await expect(page.locator('.consigna')).toContainText('Diego');
});

test('el error de género se marca en la transcripción', async ({ page }, testInfo) => {
	await freshUser(page, `speakbad-${testInfo.project.name}`);
	// El drill de Diego espera "he": la transcripción de mentira dice "she"
	await page.goto('/speaking');
	await page.getByRole('button', { name: 'Grabar' }).click();
	await page.waitForTimeout(1000);
	await page.getByRole('button', { name: /Parar la grabación/ }).click();
	await expect(page.locator('.veredicto')).toBeVisible({ timeout: 30000 });
	await page.getByRole('button', { name: /Next drill/ }).click();

	await expect(page.locator('.consigna')).toContainText('Diego');
	await page.getByRole('button', { name: 'Grabar' }).click();
	await page.waitForTimeout(1000);
	await page.getByRole('button', { name: /Parar la grabación/ }).click();

	const veredicto = page.locator('.veredicto');
	await expect(veredicto).toBeVisible({ timeout: 30000 });
	await expect(veredicto).toContainText('she');
	await expect(page.locator('.transcripcion .mal').first()).toBeVisible();
});
