import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import { createApiFetch } from '#lib/server/api.ts';
import Page from './+page.svelte';

const KEY = 'super-secret-key';

// Stand-in for the env-bound client: same factory, fixed config.
vi.mock('#lib/server/client.ts', () => ({
	apiFetch: createApiFetch({ baseUrl: 'http://api:8080', apiKey: KEY, timeoutMs: 50 })
}));

const { load } = await import('./+page.server.ts');

type LoadEvent = Parameters<typeof load>[0];
const runLoad = (fetch: typeof globalThis.fetch) => load({ fetch } as unknown as LoadEvent);

describe('home page load', () => {
	it('returns only the health result, never the key', async () => {
		const fetch = vi.fn(async () => Response.json({ status: 'ok' }));
		const data = await runLoad(fetch);
		expect(data).toEqual({ api: 'ok' });
		expect(JSON.stringify(data)).not.toContain(KEY);
	});

	it('uses the event fetch, with the bearer token, against /healthz', async () => {
		const fetch = vi.fn<typeof globalThis.fetch>(async () => Response.json({ status: 'ok' }));
		await runLoad(fetch);
		const [url, init] = fetch.mock.calls[0];
		expect(String(url)).toBe('http://api:8080/healthz');
		expect(new Headers(init?.headers).get('authorization')).toBe(`Bearer ${KEY}`);
	});

	it('returns down when the API is unreachable', async () => {
		const fetch = vi.fn(async () => {
			throw new TypeError('fetch failed');
		});
		expect(await runLoad(fetch)).toEqual({ api: 'down' });
	});
});

describe('home page render', () => {
	const html = (api: 'ok' | 'down') =>
		render(Page, { props: { data: { api }, params: {} } as never }).body;

	it('shows "API: ok"', () => {
		expect(html('ok')).toContain('API: ok');
	});

	it('shows "API: down"', () => {
		const body = html('down');
		expect(body).toContain('API: down');
		expect(body).not.toContain('API: ok');
	});
});
