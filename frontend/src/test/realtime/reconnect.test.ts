import { describe, it, expect, beforeEach } from 'vitest';
import { ReconnectManager } from '$lib/realtime/reconnect';

describe('ReconnectManager', () => {
	let manager: ReconnectManager;

	beforeEach(() => {
		manager = new ReconnectManager();
	});

	describe('getDelay', () => {
		it('returns base delay for attempt 1', () => {
			expect(manager.getDelay(1)).toBe(1000);
		});

		it('returns 2x base for attempt 2', () => {
			expect(manager.getDelay(2)).toBe(2000);
		});

		it('returns 4x base for attempt 3', () => {
			expect(manager.getDelay(3)).toBe(4000);
		});

		it('returns 8x base for attempt 4', () => {
			expect(manager.getDelay(4)).toBe(8000);
		});

		it('caps at max ms (30s) for attempt 5+', () => {
			expect(manager.getDelay(5)).toBe(16000);
			expect(manager.getDelay(6)).toBe(30000);
			expect(manager.getDelay(7)).toBe(30000);
		});
	});

	describe('shouldRetry', () => {
		it('returns true for attempts 0-4', () => {
			expect(manager.shouldRetry(0)).toBe(true);
			expect(manager.shouldRetry(1)).toBe(true);
			expect(manager.shouldRetry(2)).toBe(true);
			expect(manager.shouldRetry(3)).toBe(true);
			expect(manager.shouldRetry(4)).toBe(true);
		});

		it('returns false for attempt 5', () => {
			expect(manager.shouldRetry(5)).toBe(false);
		});

		it('returns false for attempts beyond max', () => {
			expect(manager.shouldRetry(6)).toBe(false);
			expect(manager.shouldRetry(10)).toBe(false);
		});
	});

	describe('reset', () => {
		it('resets attempt counter', () => {
			manager.incrementAttempt();
			manager.incrementAttempt();
			manager.incrementAttempt();
			expect(manager.getAttempt()).toBe(3);

			manager.reset();
			expect(manager.getAttempt()).toBe(0);
		});

		it('allows retry after reset', () => {
			// Burn through all retries
			for (let i = 0; i < 6; i++) {
				manager.incrementAttempt();
			}
			expect(manager.shouldRetry(manager.getAttempt())).toBe(false);

			manager.reset();
			expect(manager.shouldRetry(1)).toBe(true);
			expect(manager.getDelay(1)).toBe(1000);
		});
	});

	describe('incrementAttempt', () => {
		it('increments and returns the new attempt count', () => {
			expect(manager.incrementAttempt()).toBe(1);
			expect(manager.incrementAttempt()).toBe(2);
			expect(manager.incrementAttempt()).toBe(3);
		});
	});
});
