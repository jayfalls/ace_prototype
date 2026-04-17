import { describe, it, expect, vi, beforeEach } from 'vitest';
import { authStore } from '$lib/stores/auth.svelte';

// Mock the auth API
vi.mock('$lib/api/auth', () => ({
	login: vi.fn().mockResolvedValue({
		access_token: 'test-access-token',
		refresh_token: 'test-refresh-token',
		expires_in: 3600,
		user: {
			id: '1',
			username: 'testuser',
			role: 'user',
			status: 'active',
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		}
	}),
	logout: vi.fn().mockResolvedValue(undefined),
	refresh: vi.fn().mockResolvedValue({
		access_token: 'new-access-token',
		refresh_token: 'new-refresh-token',
		expires_in: 3600,
		user: {
			id: '1',
			username: 'testuser',
			role: 'user',
			status: 'active',
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		}
	}),
	me: vi.fn().mockResolvedValue({
		id: '1',
		username: 'testuser',
		role: 'user',
		status: 'active',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	})
}));

// Mock goto
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

describe('Auth Flow Integration', () => {
	beforeEach(() => {
		authStore.clear();
		vi.clearAllMocks();
	});

	it('authStore should initialize with empty state', () => {
		expect(authStore.user).toBeNull();
		expect(authStore.accessToken).toBe('');
		expect(authStore.isAuthenticated).toBe(false);
	});

	it('authStore should handle login successfully', async () => {
		await authStore.login('testuser', '123456');
		expect(authStore.user).not.toBeNull();
		expect(authStore.user?.username).toBe('testuser');
		expect(authStore.isAuthenticated).toBe(true);
	});

	it('authStore should handle logout', async () => {
		await authStore.login('testuser', '123456');
		expect(authStore.isAuthenticated).toBe(true);

		await authStore.logout();
		expect(authStore.user).toBeNull();
		expect(authStore.isAuthenticated).toBe(false);
	});

	it('authStore should track loading state during login', async () => {
		expect(authStore.isLoading).toBe(false);

		const loginPromise = authStore.login('testuser', '123456');
		expect(authStore.isLoading).toBe(true);

		await loginPromise;
		expect(authStore.isLoading).toBe(false);
	});

	it('authStore should handle login errors', async () => {
		const { login } = await import('$lib/api/auth');
		vi.mocked(login).mockRejectedValueOnce(new Error('Invalid credentials'));

		await expect(authStore.login('wronguser', 'wrongpin')).rejects.toThrow('Invalid credentials');
		expect(authStore.user).toBeNull();
	});
});
