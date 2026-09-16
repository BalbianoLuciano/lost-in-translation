# Deploy

Tres servicios. Se configuran una vez, en este orden, porque cada uno necesita un
dato del anterior.

```
Firebase (auth) ──▶ Railway (API + Postgres) ──▶ Vercel (web) ──▶ volver a cerrar CORS y dominios
```

## 1. Firebase: sólo autenticación

1. [console.firebase.google.com](https://console.firebase.google.com) → **Agregar
   proyecto** → `lost-in-translation`. Google Analytics: desactivado.
2. **Build → Authentication → Comenzar** → pestaña **Método de acceso** →
   **Google** → Habilitar → Guardar.
3. **Configuración del proyecto → Tus apps → Web (`</>`)** → registrar la app (sin
   Hosting). Anotar `apiKey`, `authDomain`, `projectId` y `appId`.

La API sólo necesita el `projectId`: verifica los ID tokens contra las claves
públicas de Google, sin cuenta de servicio.

## 2. Railway: API + Postgres

1. [railway.com](https://railway.com) → **New Project → Deploy from GitHub repo** →
   `lost-in-translation`.
2. En el servicio creado → **Settings**:
   - **Root Directory:** `services/api`
   - **Config file path:** `services/api/railway.json` (Dockerfile, healthcheck en
     `/healthz`).
3. En el proyecto → **+ New → Database → PostgreSQL**.
4. En el servicio de la API → **Variables**:

   | Variable | Valor |
   |---|---|
   | `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` |
   | `APP_ENV` | `production` |
   | `AUTH_MODE` | `firebase` |
   | `FIREBASE_PROJECT_ID` | el `projectId` del paso 1 |
   | `CORS_ORIGINS` | por ahora vacío; se completa en el paso 4 |

   `PORT` lo inyecta Railway. Las migraciones corren solas al arrancar.
5. **Settings → Networking → Generate Domain**. Anotar la URL.
6. Verificar: `curl https://<api>.up.railway.app/healthz` → `{"db":"up","status":"ok"}`.

## 3. Vercel: web

1. [vercel.com/new](https://vercel.com/new) → importar `lost-in-translation`.
2. **Root Directory:** `apps/web`. Framework: lo toma de `vercel.json` (build
   estático en `build/`).
3. **Environment Variables:**

   | Variable | Valor |
   |---|---|
   | `PUBLIC_API_URL` | la URL de Railway (paso 2.5), sin `/` final |
   | `PUBLIC_FIREBASE_API_KEY` | del paso 1 |
   | `PUBLIC_FIREBASE_AUTH_DOMAIN` | del paso 1 |
   | `PUBLIC_FIREBASE_PROJECT_ID` | del paso 1 |
   | `PUBLIC_FIREBASE_APP_ID` | del paso 1 |

   Son públicas a propósito: la config web de Firebase no es un secreto. Lo que
   protege es la lista de dominios autorizados.
4. Deploy. Anotar el dominio (`https://<app>.vercel.app`).

## 4. Cerrar el círculo

1. **Railway** → `CORS_ORIGINS=https://<app>.vercel.app` (varios, separados por
   coma). Railway redeploya solo.
2. **Firebase → Authentication → Configuración → Dominios autorizados** → agregar
   `<app>.vercel.app`.
3. Entrar a la web → **Sign in with Google** → el pie tiene que decir
   `API OK · DB UP`.

## Notas

- **Preview deploys de Vercel** tienen otro dominio por rama: el login de Google
  falla ahí hasta agregar ese dominio en Firebase y en `CORS_ORIGINS`. Para uso
  personal alcanza con producción.
- **Login en iPhone con la PWA instalada:** si el popup falla, la app cae a
  redirect. Si el redirect también falla (Safari bloquea cookies de terceros
  entre `vercel.app` y `firebaseapp.com`), la solución es un dominio propio con
  `authDomain` en ese mismo dominio. Se resuelve cuando haga falta.
