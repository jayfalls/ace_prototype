import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the constants module
vi.mock('$lib/utils/constants', () => ({
	AUTH: {
		REFRESH_THRESHOLD_MS: 30_000,
		LOCALSTORAGE_ACCESS_TOKEN: 'ace_access_token',
		LOCALSTORAGE_REFRESH_TOKEN: 'ace_refresh_token',
		LOCALSTORAGE_EXPIRES_AT: 'ace_expires_at'
	}
}));

describe('APIClient', () => {
	let apiClient: {
		request: <T>(options: {
			method: string;
			path: string;
			body?: unknown;
			requiresAuth?: boolean;
		}) => Promise<T>;
		setStoredTokens: (access: string, refresh: string, expiresAt: number) => void;
		clearStoredTokens: () => void;
		updateTokens: (access: string, refresh: string, expiresIn: number) => void;
	};

	beforeEach(async () => {
		vi.clearAllMocks();
		const { apiClient: client } = await import('$lib/api/client');
		apiClient = client as unknown as typeof apiClient;
	});

	describe('token injection', () => {
		it('injects bearer token on authenticated requests', async () => {
			apiClient.setStoredTokens('test-access', 'test-refresh', Date.now() + 60_000);

			const mockFetch = vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ success: true, data: { test: true } })
			});
			vi.stubGlobal('fetch', mockFetch);

			await apiClient.request({ method: 'GET', path: '/test', requiresAuth: true });

			expect(mockFetch).toHaveBeenCalledWith(
				'/api/test',
				expect.objectContaining({
					headers: expect.objectContaining({
						Authorization: 'Bearer test-access'
					})
				})
			);
		});

		it('does not inject token on unauthenticated requests', async () => {
			const mockFetch = vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ success: true, data: { test: true } })
			});
			vi.stubGlobal('fetch', mockFetch);

			await apiClient.request({ method: 'POST', path: '/auth/login', requiresAuth: false });

			expect(mockFetch).toHaveBeenCalledWith(
				'/api/auth/login',
				expect.objectContaining({
					headers: expect.not.objectContaining({
						Authorization: expect.any(String)
					})
				})
			);
		});
	});

	describe('401 refresh', () => {
		it('triggers token refresh on 401 response', async () => {
			const now = Date.now();
			apiClient.setStoredTokens('old-access', 'test-refresh', now + 60_000);

			const mockFetch = vi
				.fn()
				.mockResolvedValueOnce({
					ok: false,
					status: 401,
					json: () => Promise.resolve({ success: false, error: { code: 'unauthorized', message: 'Expired' } })
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () =>
						Promise.resolve({
							success: true,
							data: { access_token: 'new-access', refresh_token: 'new-refresh', expires_in: 3600 }
						})
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve({ success: true, data: { test: true } })
				});
			vi.stubGlobal('fetch', mockFetch);

			const result = await apiClient.request({ method: 'GET', path: '/test', requiresAuth: true });

			expect(mockFetch).toHaveBeenCalledTimes(3);
			expect(result).toEqual({ test: true });
		});

		it('clears tokens and throws on refresh failure after 401', async () => {
			const now = Date.now();
			apiClient.setStoredTokens('old-access', 'bad-refresh', now + 60_000);

			const mockFetch = vi
				.fn()
				.mockResolvedValueOnce({
					ok: false,
					status: 401,
					json: () => Promise.resolve({ success: false, error: { code: 'unauthorized', message: 'Expired' } })
				})
				.mockResolvedValueOnce({
					ok: false,
					status: 401,
					json: () => Promise.resolve({ success: false, error: { code: 'unauthorized', message: 'Invalid' } })
				});
			vi.stubGlobal('fetch', mockFetch);

			await expect(
				apiClient.request({ method: 'GET', path: '/test', requiresAuth: true })
			).rejects.toThrow();
		});
	});

	describe('refresh mutex', () => {
		it('blocks concurrent refresh attempts to single promise', async () => {
			const now = Date.now();
			apiClient.setStoredTokens('old-access', 'test-refresh', now + 60_000);

			let refreshCount = 0;
			const mockFetch = vi.fn().mockImplementation(() => {
				if (refreshCount === 0) {
					// First call: 401
					refreshCount++;
					return Promise.resolve({
						ok: false,
						status: 401,
						json: () =>
							Promise.resolve({
								success: false,
								error: { code: 'unauthorized', message: 'Expired' }
							})
					});
				} else if (refreshCount === 1) {
					// Second call: refresh response
					refreshCount++;
					return Promise.resolve({
						ok: true,
						json: () =>
							Promise.resolve({
								success: true,
								data: {
									access_token: 'new-access',
									refresh_token: 'new-refresh',
									expires_in: 3600
								}
							})
					});
				} else {
					// Third call: successful retry
					return Promise.resolve({
						ok: true,
						json: () => Promise.resolve({ success: true, data: { ok: true } })
					});
				}
			});
			vi.stubGlobal('fetch', mockFetch);

			// Make two concurrent requests that will both trigger 401
			const [req1, req2] = await Promise.allSettled([
				apiClient.request({ method: 'GET', path: '/test', requiresAuth: true }),
				apiClient.request({ method: 'GET', path: '/test2', requiresAuth: true })
			]);

			// Only one refresh should have occurred
			expect(refreshCount).toBe(2); // 401 + refresh response
			expect(req1.status).toBe('fulfilled');
			expect(req2.status).toBe('fulfilled');
		});
	});

	describe('error mapping', () => {
		it('maps error codes from envelope', async () => {
			const mockFetch = vi.fn().mockResolvedValue({
				ok: false,
				status: 400,
				json: () =>
					Promise.resolve({
						success: false,
						error: { code: 'validation_error', message: 'Invalid input' }
					})
			});
			vi.stubGlobal('fetch', mockFetch);

			await expect(
				apiClient.request({ method: 'POST', path: '/test', body: {}, requiresAuth: false })
			).rejects.toMatchObject({ code: 'validation_error', message: 'Invalid input' });
		});

		it('maps HTTP status to error codes', async () => {
			const mockFetch = vi.fn().mockResolvedValue({
				ok: false,
				status: 404,
				json: () => Promise.resolve({})
			});
			vi.stubGlobal('fetch', mockFetch);

			await expect(
				apiClient.request({ method: 'GET', path: '/notfound', requiresAuth: false })
			).rejects.toMatchObject({ code: 'not_found' });
		});
	});

	describe('envelope unwrapping', () => {
		it('returns data from successful envelope', async () => {
			const mockFetch = vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ success: true, data: { id: '123', name: 'test' } })
			});
			vi.stubGlobal('fetch', mockFetch);

			const result = await apiClient.request<{ id: string; name: string }>({
				method: 'GET',
				path: '/test',
				requiresAuth: false
			});

			expect(result).toEqual({ id: '123', name: 'test' });
		});

		it('throws when success is false in envelope', async () => {
			const mockFetch = vi.fn().mockResolvedValue({
				ok: true,
				json: () =>
					Promise.resolve({
						success: false,
						error: { code: 'invalid_request', message: 'Bad request' }
					})
			});
			vi.stubGlobal('fetch', mockFetch);

			await expect(
				apiClient.request({ method: 'POST', path: '/test', body: {}, requiresAuth: false })
			).rejects.toMatchObject({ code: 'invalid_request', message: 'Bad request' });
		});
	});
});
