export type ApiHealth = 'ok' | 'down';

/**
 * Asks the API's /healthz and reduces the answer to ok or down. Only a 200
 * with {"status":"ok"} counts as ok; checking the body as well as the status
 * catches some other service answering on that port. Never throws: network
 * errors and timeouts are "down". The API reports 503 when its database is
 * unreachable, so that shows as down too.
 */
export async function getHealth(call: (path: string) => Promise<Response>): Promise<ApiHealth> {
	try {
		const res = await call('/healthz');
		if (res.status !== 200) return 'down';
		const body: unknown = await res.json();
		return isOk(body) ? 'ok' : 'down';
	} catch {
		return 'down';
	}
}

function isOk(body: unknown): boolean {
	return (
		typeof body === 'object' && body !== null && (body as { status?: unknown }).status === 'ok'
	);
}
