<!--
  Política de privacidad. La ruta es en castellano como el resto del repo; el
  texto va en inglés porque la interfaz está en inglés.

  Regla del texto: nada de plantilla. Cada párrafo tiene que ser verdad de esta
  app en particular, y si algo es incómodo (la caché del profesor, que no se
  borra) se dice igual.
-->
<svelte:head>
	<title>Privacy · Lost in Translation</title>
</svelte:head>

<header class="cabecera tapa">
	<a class="etiqueta volver" href="/">← Back</a>
	<p class="etiqueta num">Last updated 2026-09-24</p>
</header>

<main class="hoja tapa">
	<h1 class="titular">Privacy.</h1>
	<p class="bajada">
		Short version: your email, your answers and the transcripts of what you say out loud. The audio
		itself is never stored. You can download all of it, and you can delete all of it, from the home
		screen.
	</p>

	<section class="bloque">
		<h2 class="sub">What is stored</h2>
		<dl class="lista">
			<dt>Your account</dt>
			<dd>
				Email address, display name and the user id Google gives you when you sign in, plus your
				theme setting and the dates you signed up and last showed up.
			</dd>

			<dt>What you answer</dt>
			<dd>
				Every exercise you answer: which item, what you typed or picked, whether it was right, how
				many milliseconds it took, and whether you opened the glossary while answering. Also your
				review schedule, how well you know each topic, which lessons you finished, and a per-day
				count of minutes and exercises.
			</dd>

			<dt>What you say out loud</dt>
			<dd>
				The <strong>transcript</strong> of each speaking drill — the text, and which words you were
				supposed to use and did or didn't. Nothing else.
			</dd>

			<dt>How much you use the AI</dt>
			<dd>
				A number per day: how many tutor questions and how many speaking drills. It exists to stop
				one person from spending the whole budget, and it doesn't record what the questions were.
			</dd>
		</dl>
	</section>

	<section class="bloque">
		<h2 class="sub">What is not stored</h2>
		<p>
			<strong>The audio is never saved.</strong> Your recording goes straight to the transcription
			service, comes back as text, and is gone. It doesn't touch the database and it isn't written
			to disk.
		</p>
		<p>
			There are no analytics, no advertising, no third-party trackers and no payment details. The
			only thing in your browser's storage is the sign-in session and your theme.
		</p>
	</section>

	<section class="bloque">
		<h2 class="sub">Who else sees it</h2>
		<dl class="lista">
			<dt>Google Firebase — who you are</dt>
			<dd>
				Sign-in runs on Firebase Authentication. Google knows your email and that you signed in
				here. This app never sees your Google password and never asks for one.
			</dd>

			<dt>Groq — the tutor and the transcription</dt>
			<dd>
				When you ask the tutor something, the question and the exercise it refers to are sent to
				Groq. When you record a speaking drill, the audio is sent to Groq to be turned into text.
				Your email, your name and your account id are never part of that request.
			</dd>

			<dt>Hosting</dt>
			<dd>
				The web app is served by Vercel; the API and the Postgres database run on Railway. They see
				the traffic and hold the data at rest, like any host does.
			</dd>
		</dl>
		<p>Nothing is sold, rented or shared with anyone else.</p>
	</section>

	<section class="bloque">
		<h2 class="sub">The one thing deleting your account does not remove</h2>
		<p>
			Tutor answers are cached so the same question isn't paid for twice. The cache is keyed by a
			hash of the prompt and is shared by everyone: it holds the text of the question and the
			answer, and <strong>no user id</strong> — there is no column, no index and no join that ties a
			cached question back to a person.
		</p>
		<p>
			So deleting your account does not delete those rows, because by then they belong to nobody and
			there is no way to tell which ones were yours. Keeping them is what makes the tutor affordable
			for the next person who asks the same thing. It is also the one place where free text you
			typed outlives your account, which is why it says so here: ask the tutor about English, not
			about your life.
		</p>
	</section>

	<section class="bloque">
		<h2 class="sub">How long it is kept</h2>
		<p>
			For as long as the account exists. Delete the account and everything above goes with it in the
			same request — the database is set up so that removing the user removes every row that points
			at it.
		</p>
		<p>
			Server logs keep the request path, the status and how long it took, for a short while. They
			don't contain your answers or your questions.
		</p>
	</section>

	<section class="bloque">
		<h2 class="sub">Taking it with you, or ending it</h2>
		<dl class="lista">
			<dt>Export</dt>
			<dd>
				<strong>Download my data</strong> on the home screen gives you a JSON file with your
				profile, every attempt, your review cards, your mastery per topic, your lessons and your
				daily log.
			</dd>

			<dt>Delete</dt>
			<dd>
				<strong>Delete my account</strong> on the home screen, confirmed by typing your email
				address. It happens immediately and cannot be undone. Signing in again afterwards starts a
				new, empty account — if the invite list still has you on it.
			</dd>
		</dl>
	</section>

	<section class="bloque">
		<h2 class="sub">Nothing is on a blockchain</h2>
		<p>
			Today there is no wallet, no token and nothing published on any chain. If that ever changes it
			will be opt-in, it will never carry your email or your name, and you will be told the
			uncomfortable part before you sign anything: what goes on a public chain cannot be deleted,
			not even by deleting your account here.
		</p>
	</section>

	<p class="cierre">
		This is a personal project run by one person, open to a handful of people by invitation. If you
		were invited, you know who to ask.
	</p>

	<p class="etiqueta"><a href="/terminos">Terms →</a></p>
</main>

<style>
	.cabecera {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		min-height: 44px;
		border-bottom: 1px solid var(--line);
	}

	.volver {
		text-decoration: none;
		display: inline-flex;
		align-items: center;
		min-height: 44px;
	}

	.hoja {
		display: grid;
		gap: 28px;
		max-width: 68ch;
		padding-block: calc(var(--module) * 1.5) calc(var(--module) * 3);
	}

	p {
		margin: 0;
	}

	.titular {
		font-size: clamp(40px, 8vw, 56px);
	}

	.bajada {
		font-size: 20px;
		line-height: 1.45;
		font-weight: 500;
	}

	.bloque {
		display: grid;
		gap: 12px;
		padding-top: 20px;
		border-top: 1px solid var(--line);
	}

	.sub {
		margin: 0;
		font-size: 24px;
		font-weight: 700;
		letter-spacing: -0.02em;
	}

	.lista {
		display: grid;
		gap: 4px;
		margin: 0;
	}

	.lista dt {
		font-weight: 600;
		margin-top: 12px;
	}

	.lista dt:first-child {
		margin-top: 0;
	}

	.lista dd {
		margin: 0;
		color: var(--text-muted);
	}

	.cierre {
		padding-top: 20px;
		border-top: 1px solid var(--line);
		color: var(--text-muted);
	}

	a {
		color: inherit;
	}

	.etiqueta a {
		text-decoration: none;
	}

	.etiqueta a:hover {
		color: var(--baranda);
	}
</style>
