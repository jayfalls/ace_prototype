import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock dependencies
vi.mock('$lib/api/client', () => ({
	apiClient: {
		request: vi.fn()
	}
}));

describe('Auth API', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('login', () => {
		it('calls POST /auth/login with username and pin', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { login } = await import('$lib/api/auth');

			vi.mocked(apiClient.request).mockResolvedValue({
				access_token: 'token',
				refresh_token: 'refresh',
				user: { id: '1', username: 'testuser', email: 'test@test.com', role: 'user', status: 'active', created_at: '', updated_at: '' },
				expires_in: 3600
			});

			const result = await login('testuser', '123456');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'POST',
				path: '/auth/login',
				body: { username: 'testuser', pin: '123456' },
				requiresAuth: false
			});
			expect(result.access_token).toBe('token');
		});
	});

	describe('register', () => {
		it('calls POST /auth/register with username, pin, and email', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { register } = await import('$lib/api/auth');

			vi.mocked(apiClient.request).mockResolvedValue({
				access_token: 'token',
				refresh_token: 'refresh',
				user: { id: '1', username: 'newuser', email: 'new@test.com', role: 'user', status: 'active', created_at: '', updated_at: '' },
				expires_in: 3600
			});

			const result = await register('newuser', '123456', 'new@test.com');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'POST',
				path: '/auth/register',
				body: { username: 'newuser', pin: '123456', email: 'new@test.com' },
				requiresAuth: false
			});
			expect(result.user.username).toBe('newuser');
		});
	});

	describe('logout', () => {
		it('calls POST /auth/logout with session_id', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { logout } = await import('$lib/api/auth');

			vi.mocked(apiClient.request).mockResolvedValue(undefined);

			await logout('session-123');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'POST',
				path: '/auth/logout',
				body: { session_id: 'session-123' }
			});
		});
	});

	describe('refresh', () => {
		it('calls POST /auth/refresh with refresh_token', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { refresh } = await import('$lib/api/auth');

			vi.mocked(apiClient.request).mockResolvedValue({
				access_token: 'new-token',
				refresh_token: 'new-refresh',
				user: { id: '1', username: 'testuser', email: 'test@test.com', role: 'user', status: 'active', created_at: '', updated_at: '' },
				expires_in: 3600
			});

			const result = await refresh('old-refresh-token');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'POST',
				path: '/auth/refresh',
				body: { refresh_token: 'old-refresh-token' },
				requiresAuth: false
			});
			expect(result.access_token).toBe('new-token');
		});
	});

	describe('me', () => {
		it('calls GET /auth/me', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { me } = await import('$lib/api/auth');

			const mockUser = {
				id: '1',
				username: 'testuser',
				email: 'test@test.com',
				role: 'user' as const,
				status: 'active' as const,
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z'
			};
			vi.mocked(apiClient.request).mockResolvedValue(mockUser);

			const result = await me();

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'GET',
				path: '/auth/me'
			});
			expect(result.username).toBe('testuser');
		});
	});

	describe('listUsers', () => {
		it('calls GET /users', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { listUsers } = await import('$lib/api/auth');

			const mockUsers = [
				{ id: '1', username: 'user1', email: 'user1@test.com', role: 'user' as const, status: 'active' as const },
				{ id: '2', username: 'user2', email: 'user2@test.com', role: 'admin' as const, status: 'active' as const }
			];
			vi.mocked(apiClient.request).mockResolvedValue(mockUsers);

			const result = await listUsers();

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'GET',
				path: '/users',
				requiresAuth: false
			});
			expect(result).toHaveLength(2);
			expect(result[0].username).toBe('user1');
		});
	});

	describe('resetPasswordRequest', () => {
		it('calls POST /auth/password/reset/request with email', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { resetPasswordRequest } = await import('$lib/api/auth');

			vi.mocked(apiClient.request).mockResolvedValue(undefined);

			await resetPasswordRequest('test@test.com');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'POST',
				path: '/auth/password/reset/request',
				body: { email: 'test@test.com' },
				requiresAuth: false
			});
		});
	});

	describe('resetPasswordConfirm', () => {
		it('calls POST /auth/password/reset/confirm with token and new_password', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { resetPasswordConfirm } = await import('$lib/api/auth');

			vi.mocked(apiClient.request).mockResolvedValue({
				access_token: 'token',
				refresh_token: 'refresh',
				user: { id: '1', username: 'testuser', email: 'test@test.com', role: 'user', status: 'active', created_at: '', updated_at: '' },
				expires_in: 3600
			});

			const result = await resetPasswordConfirm('reset-token-xyz', 'newpassword123');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'POST',
				path: '/auth/password/reset/confirm',
				body: { token: 'reset-token-xyz', new_password: 'newpassword123' },
				requiresAuth: false
			});
			expect(result.access_token).toBe('token');
		});
	});

	describe('magicLinkRequest', () => {
		it('calls POST /auth/magic-link/request with email', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { magicLinkRequest } = await import('$lib/api/auth');

			vi.mocked(apiClient.request).mockResolvedValue(undefined);

			await magicLinkRequest('test@test.com');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'POST',
				path: '/auth/magic-link/request',
				body: { email: 'test@test.com' },
				requiresAuth: false
			});
		});
	});

	describe('magicLinkVerify', () => {
		it('calls POST /auth/magic-link/verify with token', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { magicLinkVerify } = await import('$lib/api/auth');

			vi.mocked(apiClient.request).mockResolvedValue({
				access_token: 'token',
				refresh_token: 'refresh',
				user: { id: '1', username: 'testuser', email: 'test@test.com', role: 'user', status: 'active', created_at: '', updated_at: '' },
				expires_in: 3600
			});

			const result = await magicLinkVerify('magic-token-xyz');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'POST',
				path: '/auth/magic-link/verify',
				body: { token: 'magic-token-xyz' },
				requiresAuth: false
			});
			expect(result.access_token).toBe('token');
		});
	});
});
