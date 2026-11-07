import { describe, expect, it, vi } from 'vitest';
import { createApiFetch, type Fetch } from './api.ts';
import { getHealth } from './health.ts';

const json = (status: number, body: unknown) =>
	new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});

const respond = (res: Response | (() => Response)) => () =>
	Promise.resolve(typeof res === 'function' ? res() : res);

describe('getHealth', () => {
	it('is ok on 200 {"status":"ok"}', async () => {
		expect(await getHealth(respond(json(200, { status: 'ok' })))).toBe('ok');
	});

	it.each([
		['503 unavailable (DB down)', () => json(503, { status: 'unavailable' })],
		['500', () => json(500, { status: 'ok' })],
		['404', () => new Response('not found', { status: 404 })],
		['200 with non-JSON body', () => new Response('<html>hi</html>', { status: 200 })],
		['200 with another status', () => json(200, { status: 'nope' })],
		['200 with JSON null', () => json(200, null)],
		['200 with an array', () => json(200, ['ok'])]
	])('is down on %s', async (_name, make) => {
		expect(await getHealth(respond(make))).toBe('down');
	});

	it('is down, without throwing, when the request fails', async () => {
		const refused = () => Promise.reject(new TypeError('fetch failed'));
		await expect(getHealth(refused)).resolves.toBe('down');
	});

	it('asks /healthz', async () => {
		const call = vi.fn(respond(json(200, { status: 'ok' })));
		await getHealth(call);
		expect(call).toHaveBeenCalledWith('/healthz');
	});

	it('reflects each call, with no caching', async () => {
		const answers = [
			json(200, { status: 'ok' }),
			json(503, { status: 'unavailable' }),
			json(200, { status: 'ok' })
		];
		const call = () => Promise.resolve(answers.shift()!);
		const got = [await getHealth(call), await getHealth(call), await getHealth(call)];
		expect(got).toEqual(['ok', 'down', 'ok']);
	});

	// Through the real client: the timeout has to come from createApiFetch.
	it('is down when the API hangs past the client timeout', async () => {
		const hang: Fetch = (_url, init) =>
			new Promise((_resolve, reject) => {
				init?.signal?.addEventListener('abort', () => reject(init.signal?.reason));
			});
		const api = createApiFetch({ baseUrl: 'http://api:8080', timeoutMs: 30 });
		const started = Date.now();
		expect(await getHealth((path) => api(hang, path))).toBe('down');
		expect(Date.now() - started).toBeLessThan(1000);
	});

	it('is down when the body stalls past the client timeout', async () => {
		const stalledBody: Fetch = (_url, init) => {
			const body = new ReadableStream({
				start(controller) {
					controller.enqueue(new TextEncoder().encode('{"sta'));
					init?.signal?.addEventListener('abort', () => controller.error(init.signal?.reason));
				}
			});
			return Promise.resolve(new Response(body, { status: 200 }));
		};
		const api = createApiFetch({ baseUrl: 'http://api:8080', timeoutMs: 30 });
		expect(await getHealth((path) => api(stalledBody, path))).toBe('down');
	});

	it('builds the right URL through the real client, trailing slash removed upstream', async () => {
		const fetchFn = vi.fn<Fetch>(async () => json(200, { status: 'ok' }));
		const api = createApiFetch({ baseUrl: 'http://api:8080' });
		await getHealth((path) => api(fetchFn, path));
		expect(String(fetchFn.mock.calls[0][0])).toBe('http://api:8080/healthz');
	});
});
