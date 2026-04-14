import { goto } from '$app/navigation';
import * as authApi from '$lib/api/auth';
import { apiClient } from '$lib/api/client';
import type { User, TokenResponse } from '$lib/api/types';
import { AUTH, ROUTES } from '$lib/utils/constants';

class AuthStore {
	user = $state<User | null>(null);
	accessToken = $state<string>('');
	refreshToken = $state<string>('');
	expiresAt = $state<number>(0);
	isLoading = $state<boolean>(false);
	error = $state<string | null>(null);

	isAuthenticated = $derived(this.user !== null && this.accessToken !== '');

	private setTokens(access: string, refresh: string, expiresIn: number): void {
		this.accessToken = access;
		this.refreshToken = refresh;
		this.expiresAt = Date.now() + expiresIn * 1000;
		apiClient.updateTokens(access, refresh, expiresIn);
		this.persistToStorage();
	}

	private persistToStorage(): void {
		if (typeof localStorage === 'undefined') return;
		localStorage.setItem(AUTH.LOCALSTORAGE_ACCESS_TOKEN, this.accessToken);
		localStorage.setItem(AUTH.LOCALSTORAGE_REFRESH_TOKEN, this.refreshToken);
		localStorage.setItem(AUTH.LOCALSTORAGE_EXPIRES_AT, String(this.expiresAt));
	}

	private clearStorage(): void {
		if (typeof localStorage === 'undefined') return;
		localStorage.removeItem(AUTH.LOCALSTORAGE_ACCESS_TOKEN);
		localStorage.removeItem(AUTH.LOCALSTORAGE_REFRESH_TOKEN);
		localStorage.removeItem(AUTH.LOCALSTORAGE_EXPIRES_AT);
	}

	init(): void {
		if (typeof localStorage === 'undefined') return;
		const access = localStorage.getItem(AUTH.LOCALSTORAGE_ACCESS_TOKEN) ?? '';
		const refresh = localStorage.getItem(AUTH.LOCALSTORAGE_REFRESH_TOKEN) ?? '';
		const expiresAt = parseInt(localStorage.getItem(AUTH.LOCALSTORAGE_EXPIRES_AT) ?? '0', 10);

		if (!access || !refresh || !expiresAt) return;

		// If expired, attempt refresh
		if (Date.now() >= expiresAt) {
			this.accessToken = access;
			this.refreshToken = refresh;
			this.expiresAt = expiresAt;
			apiClient.setStoredTokens(access, refresh, expiresAt);
			this.refreshTokens().catch(() => this.clear());
			return;
		}

		this.accessToken = access;
		this.refreshToken = refresh;
		this.expiresAt = expiresAt;
		apiClient.setStoredTokens(access, refresh, expiresAt);

		// Fetch user data
		this.isLoading = true;
		authApi
			.me()
			.then((user) => {
				this.user = user;
			})
			.catch(() => {
				this.clear();
			})
			.finally(() => {
				this.isLoading = false;
			});
	}

	async login(username: string, pin: string): Promise<void> {
		this.isLoading = true;
		this.error = null;
		try {
			const response = await authApi.login(username, pin);
			this.handleTokenResponse(response);
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Login failed';
			throw err;
		} finally {
			this.isLoading = false;
		}
	}

	async register(username: string, pin: string): Promise<void> {
		this.isLoading = true;
		this.error = null;
		try {
			const response = await authApi.register(username, pin);
			this.handleTokenResponse(response);
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Registration failed';
			throw err;
		} finally {
			this.isLoading = false;
		}
	}

	async logout(): Promise<void> {
		try {
			await authApi.logout(this.refreshToken);
		} catch {
			// Ignore logout errors, clear state anyway
		} finally {
			this.clear();
			goto(ROUTES.LOGIN);
		}
	}

	async refreshTokens(): Promise<void> {
		if (!this.refreshToken) {
			this.clear();
			goto(ROUTES.LOGIN);
			return;
		}

		try {
			const response = await authApi.refresh(this.refreshToken);
			this.handleTokenResponse(response);
		} catch {
			this.clear();
			goto(ROUTES.LOGIN);
			throw new Error('Token refresh failed');
		}
	}

	async ensureValidToken(): Promise<void> {
		const remaining = this.expiresAt - Date.now();
		if (remaining < AUTH.REFRESH_THRESHOLD_MS) {
			await this.refreshTokens();
		}
	}

	clear(): void {
		this.user = null;
		this.accessToken = '';
		this.refreshToken = '';
		this.expiresAt = 0;
		this.error = null;
		this.clearStorage();
		apiClient.clearStoredTokens();
	}

	private handleTokenResponse(response: TokenResponse): void {
		this.user = response.user;
		this.setTokens(response.access_token, response.refresh_token, response.expires_in);
	}
}

export const authStore = new AuthStore();
