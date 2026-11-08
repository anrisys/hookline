import { describe, expect, it, vi } from 'vitest';

// If the route ever touches the API client, this mock makes the test fail.
const apiFetch = vi.fn(() => {
	throw new Error('web /healthz must not call the API');
});
vi.mock('#lib/server/client.ts', () => ({ apiFetch }));

const { GET } = await import('./+server.ts');

describe('GET /healthz', () => {
	it('answers 200 ok without calling the API', async () => {
		const res = await GET({} as Parameters<typeof GET>[0]);
		expect(res.status).toBe(200);
		expect(await res.json()).toEqual({ status: 'ok' });
		expect(apiFetch).not.toHaveBeenCalled();
	});
});
