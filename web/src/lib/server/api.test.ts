import { describe, expect, it, vi } from 'vitest';
import { createApiFetch, type Fetch } from './api.ts';

function recordingFetch() {
	const fn = vi.fn<Fetch>(async () => new Response('{}'));
	const call = () => {
		const [url, init] = fn.mock.calls[0];
		return { url: String(url), init: init ?? {}, headers: new Headers(init?.headers) };
	};
	return { fn, call };
}

describe('createApiFetch', () => {
	it('prefixes the base URL', async () => {
		const f = recordingFetch();
		const api = createApiFetch({ baseUrl: 'http://api:8080' });
		await api(f.fn, '/healthz');
		expect(f.call().url).toBe('http://api:8080/healthz');
	});

	it('adds the leading slash when the path has none', async () => {
		const f = recordingFetch();
		await createApiFetch({ baseUrl: 'http://api:8080/v1' })(f.fn, 'healthz');
		expect(f.call().url).toBe('http://api:8080/v1/healthz');
	});

	it('sends the bearer token when a key is configured', async () => {
		const f = recordingFetch();
		await createApiFetch({ baseUrl: 'http://api', apiKey: 'k-123' })(f.fn, '/x');
		expect(f.call().headers.get('authorization')).toBe('Bearer k-123');
	});

	it('sends no Authorization header without a key', async () => {
		const f = recordingFetch();
		await createApiFetch({ baseUrl: 'http://api' })(f.fn, '/x');
		expect(f.call().headers.has('authorization')).toBe(false);
	});

	it('keeps caller headers and method', async () => {
		const f = recordingFetch();
		await createApiFetch({ baseUrl: 'http://api', apiKey: 'k' })(f.fn, '/x', {
			method: 'POST',
			headers: { 'content-type': 'application/json' }
		});
		expect(f.call().init.method).toBe('POST');
		expect(f.call().headers.get('content-type')).toBe('application/json');
	});

	it('aborts after the timeout', async () => {
		const hang: Fetch = (_url, init) =>
			new Promise((_resolve, reject) => {
				init?.signal?.addEventListener('abort', () => reject(init.signal?.reason));
			});
		const api = createApiFetch({ baseUrl: 'http://api', timeoutMs: 30 });
		const started = Date.now();
		await expect(api(hang, '/x')).rejects.toThrow();
		expect(Date.now() - started).toBeLessThan(1000);
	});

	it('still honours a caller abort signal', async () => {
		const f = recordingFetch();
		const caller = new AbortController();
		await createApiFetch({ baseUrl: 'http://api' })(f.fn, '/x', { signal: caller.signal });
		caller.abort();
		expect(f.call().init.signal?.aborted).toBe(true);
	});
});
