import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock modules before importing the store
vi.mock('$lib/api/auth', () => ({
	login: vi.fn(),
	register: vi.fn(),
	logout: vi.fn(),
	refresh: vi.fn(),
	me: vi.fn()
}));

vi.mock('$lib/api/client', () => ({
	apiClient: {
		setStoredTokens: vi.fn(),
		clearStoredTokens: vi.fn(),
		updateTokens: vi.fn()
	}
}));

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

describe('AuthStore', () => {
	let authStore: {
		user: unknown;
		accessToken: unknown;
		refreshToken: unknown;
		expiresAt: unknown;
		isLoading: unknown;
		error: unknown;
		isAuthenticated: unknown;
		login: (email: string, password: string) => Promise<void>;
		register: (email: string, password: string) => Promise<void>;
		logout: () => Promise<void>;
		refreshTokens: () => Promise<void>;
		ensureValidToken: () => Promise<void>;
		init: () => void;
		clear: () => void;
	};

	const mockUser = {
		id: '1',
		email: 'test@test.com',
		role: 'user' as const,
		status: 'active' as const,
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	};

	const mockTokenResponse = {
		access_token: 'access-token-123',
		refresh_token: 'refresh-token-456',
		user: mockUser,
		expires_in: 3600
	};

	beforeEach(async () => {
		vi.clearAllMocks();
		// Reset localStorage mock
		const storage: Record<string, string> = {};
		vi.stubGlobal('localStorage', {
			getItem: vi.fn((key: string) => storage[key] ?? null),
			setItem: vi.fn((key: string, value: string) => {
				storage[key] = value;
			}),
			removeItem: vi.fn((key: string) => {
				delete storage[key];
			})
		});

		const { authStore: store } = await import('$lib/stores/auth.svelte');
		authStore = store as unknown as typeof authStore;
	});

	afterEach(() => {
		authStore.clear();
	});

	describe('login', () => {
		it('stores tokens and user on successful login', async () => {
			const { login } = await import('$lib/api/auth');
			vi.mocked(login).mockResolvedValue(mockTokenResponse);

			await authStore.login('test@test.com', 'password123');

			expect(login).toHaveBeenCalledWith('test@test.com', 'password123');
			expect(authStore.user).toEqual(mockUser);
			expect(authStore.accessToken).toBe('access-token-123');
			expect(authStore.refreshToken).toBe('refresh-token-456');
			expect(authStore.error).toBeNull();
		});

		it('stores error on failed login', async () => {
			const { login } = await import('$lib/api/auth');
			vi.mocked(login).mockRejectedValue(new Error('Invalid credentials'));

			await expect(
				authStore.login('test@test.com', 'wrongpassword')
			).rejects.toThrow('Invalid credentials');
			expect(authStore.error).toBe('Invalid credentials');
		});
	});

	describe('logout', () => {
		it('clears storage and resets state', async () => {
			const { logout } = await import('$lib/api/auth');
			const { goto } = await import('$app/navigation');
			const { apiClient } = await import('$lib/api/client');

			vi.mocked(logout).mockResolvedValue(undefined);

			// Setup state
			authStore.user = mockUser;
			authStore.accessToken = 'token';
			authStore.refreshToken = 'refresh';

			await authStore.logout();

			expect(logout).toHaveBeenCalledWith('refresh');
			expect(apiClient.clearStoredTokens).toHaveBeenCalled();
			expect(authStore.user).toBeNull();
			expect(authStore.accessToken).toBe('');
			expect(authStore.refreshToken).toBe('');
			expect(goto).toHaveBeenCalledWith('/login');
		});
	});

	describe('refreshTokens', () => {
		it('updates tokens on successful refresh', async () => {
			const { refresh } = await import('$lib/api/auth');
			const { apiClient } = await import('$lib/api/client');

			const newTokenResponse = {
				access_token: 'new-access-token',
				refresh_token: 'new-refresh-token',
				user: mockUser,
				expires_in: 7200
			};
			vi.mocked(refresh).mockResolvedValue(newTokenResponse);

			authStore.refreshToken = 'old-refresh';
			await authStore.refreshTokens();

			expect(refresh).toHaveBeenCalledWith('old-refresh');
			expect(authStore.accessToken).toBe('new-access-token');
			expect(authStore.refreshToken).toBe('new-refresh-token');
			expect(apiClient.updateTokens).toHaveBeenCalledWith(
				'new-access-token',
				'new-refresh-token',
				7200
			);
		});

		it('clears and redirects on failed refresh', async () => {
			const { refresh } = await import('$lib/api/auth');
			const { goto } = await import('$app/navigation');

			vi.mocked(refresh).mockRejectedValue(new Error('Invalid refresh token'));

			authStore.refreshToken = 'bad-token';
			authStore.user = mockUser;

			await expect(authStore.refreshTokens()).rejects.toThrow('Token refresh failed');
			expect(authStore.user).toBeNull();
			expect(goto).toHaveBeenCalledWith('/login');
		});
	});

	describe('init', () => {
		it('restores state from localStorage if tokens valid', async () => {
			const { me } = await import('$lib/api/auth');
			const { apiClient } = await import('$lib/api/client');

			const futureExpiry = Date.now() + 60_000;
			localStorage.setItem('ace_access_token', 'stored-access');
			localStorage.setItem('ace_refresh_token', 'stored-refresh');
			localStorage.setItem('ace_expires_at', String(futureExpiry));

			vi.mocked(me).mockResolvedValue(mockUser);

			authStore.init();

			expect(apiClient.setStoredTokens).toHaveBeenCalledWith(
				'stored-access',
				'stored-refresh',
				futureExpiry
			);
			expect(authStore.isLoading).toBe(true);
			// Wait for async me() call
			await new Promise((r) => setTimeout(r, 10));
			expect(authStore.user).toEqual(mockUser);
			expect(authStore.isLoading).toBe(false);
		});

		it('clears if no tokens in storage', () => {
			authStore.init();

			expect(authStore.user).toBeNull();
			expect(authStore.accessToken).toBe('');
		});

		it('attempts refresh if token expired during init', async () => {
			const { refresh } = await import('$lib/api/auth');
			const { apiClient } = await import('$lib/api/client');

			const pastExpiry = Date.now() - 1000;
			localStorage.setItem('ace_access_token', 'expired-access');
			localStorage.setItem('ace_refresh_token', 'stored-refresh');
			localStorage.setItem('ace_expires_at', String(pastExpiry));

			vi.mocked(refresh).mockResolvedValue(mockTokenResponse);

			authStore.init();

			// Wait for the refresh attempt
			await new Promise((r) => setTimeout(r, 10));
			expect(refresh).toHaveBeenCalledWith('stored-refresh');
		});
	});

	describe('clear', () => {
		it('resets all state to defaults', async () => {
			const { apiClient } = await import('$lib/api/client');

			authStore.user = mockUser;
			authStore.accessToken = 'token';
			authStore.refreshToken = 'refresh';
			authStore.expiresAt = Date.now() + 60_000;
			authStore.error = 'some error';

			authStore.clear();

			expect(authStore.user).toBeNull();
			expect(authStore.accessToken).toBe('');
			expect(authStore.refreshToken).toBe('');
			expect(authStore.expiresAt).toBe(0);
			expect(authStore.error).toBeNull();
			expect(apiClient.clearStoredTokens).toHaveBeenCalled();
			expect(localStorage.getItem('ace_access_token')).toBeNull();
		});
	});
});
