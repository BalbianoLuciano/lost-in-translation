import { defineConfig, devices } from '@playwright/test';

// El E2E levanta la API en Go (modo auth dev) y la web, contra un Postgres real.
// Local: `docker compose up -d` o un Postgres propio, y
//   E2E_DATABASE_URL=postgres://localhost:5432/lit_test?sslmode=disable pnpm test:e2e
const DATABASE_URL =
	process.env.E2E_DATABASE_URL ?? 'postgres://lit:lit@localhost:54329/lit?sslmode=disable';

const API_PORT = 18080;
const WEB_PORT = 5174;
const WEB_URL = `http://127.0.0.1:${WEB_PORT}`;

export default defineConfig({
	testDir: 'e2e',
	timeout: 120_000,
	expect: { timeout: 10_000 },
	fullyParallel: false,
	workers: 1,
	retries: process.env.CI ? 1 : 0,
	reporter: process.env.CI ? [['github'], ['list']] : [['list']],
	use: { baseURL: WEB_URL, trace: 'retain-on-failure' },
	projects: [
		{ name: 'mobile', use: { ...devices['Pixel 7'] } },
		{ name: 'desktop', use: { ...devices['Desktop Chrome'] } }
	],
	webServer: [
		{
			command: 'go run ./cmd/api',
			cwd: '../../services/api',
			url: `http://127.0.0.1:${API_PORT}/healthz`,
			reuseExistingServer: !process.env.CI,
			timeout: 180_000,
			env: {
				APP_ENV: 'development',
				AUTH_MODE: 'dev',
				PORT: String(API_PORT),
				DATABASE_URL,
				CORS_ORIGINS: WEB_URL
			}
		},
		{
			command: `pnpm dev --host 127.0.0.1 --port ${WEB_PORT} --strictPort`,
			url: WEB_URL,
			reuseExistingServer: !process.env.CI,
			timeout: 120_000,
			env: {
				PUBLIC_API_URL: `http://127.0.0.1:${API_PORT}`,
				// vacío → la web ofrece el login falso de desarrollo
				PUBLIC_FIREBASE_API_KEY: ''
			}
		}
	]
});
