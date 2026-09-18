import { expect, test, type Page } from '@playwright/test';

function freshUser(page: Page, tag: string) {
	const uid = `e2e-${tag}-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
	return page.addInitScript(
		(user) => localStorage.setItem('lit-dev-user', JSON.stringify(user)),
		{ uid, email: `${uid}@dev.local`, name: 'E2E' }
	);
}

test('el mapa muestra la obra y lleva a la lección', async ({ page }, testInfo) => {
	await freshUser(page, `map-${testInfo.project.name}`);
	await page.goto('/map');

	await expect(page.getByRole('heading', { name: /Your obra/ })).toBeVisible();
	// Una pieza por tema: 33 habilidades en el banco
	const piezas = page.locator('.pieza');
	await expect(piezas).toHaveCount(33);

	// Sin diagnóstico, todo está en plano: dibujado, no construido
	await expect(page.locator('.pieza.plano').first()).toBeVisible();

	// Tocar una pieza abre su detalle
	await page.locator('.toque').first().click();
	const detalle = page.locator('.detalle');
	await expect(detalle).toBeVisible();
	await expect(detalle.getByText(/Plano|Suspendida|Calzada|Oxidada/)).toBeVisible();

	// Desde el tablero se llega al mapa
	await page.goto('/');
	await page.locator('a[href="/map"]').first().click();
	await expect(page.getByRole('heading', { name: /Your obra/ })).toBeVisible();
});

test('una pieza con lección lleva a estudiarla', async ({ page }, testInfo) => {
	await freshUser(page, `maplesson-${testInfo.project.name}`);
	await page.goto('/map');
	await expect(page.getByRole('heading', { name: /Your obra/ })).toBeVisible();

	// Los condicionales son de los pocos temas que ya tienen lección escrita
	await page.getByRole('button', { name: 'First and second conditional' }).click();
	const abrir = page.getByRole('link', { name: /Open the lesson/ });
	await expect(abrir).toBeVisible();
	await abrir.click();
	await expect(page.locator('main .titular')).toBeVisible();
});
