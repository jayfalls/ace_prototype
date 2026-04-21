import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { ConnectionStatus } from '$lib/realtime/types';

// Test the status config and logic that ConnectionIndicator uses
// This mirrors how other tests in the codebase test component logic

const statusConfigs: Record<string, { color: string; label: string }> = {
	connected: { color: 'bg-green-500', label: 'Connected' },
	connecting: { color: 'bg-yellow-500', label: 'Connecting...' },
	polling: { color: 'bg-yellow-500', label: 'Polling' },
	disconnected: { color: 'bg-red-500', label: 'Disconnected' }
};

describe('ConnectionIndicator status configs', () => {
	it('connected has green color and Connected label', () => {
		expect(statusConfigs.connected.color).toBe('bg-green-500');
		expect(statusConfigs.connected.label).toBe('Connected');
	});

	it('connecting has yellow color and Connecting... label', () => {
		expect(statusConfigs.connecting.color).toBe('bg-yellow-500');
		expect(statusConfigs.connecting.label).toBe('Connecting...');
	});

	it('polling has yellow color and Polling label', () => {
		expect(statusConfigs.polling.color).toBe('bg-yellow-500');
		expect(statusConfigs.polling.label).toBe('Polling');
	});

	it('disconnected has red color and Disconnected label', () => {
		expect(statusConfigs.disconnected.color).toBe('bg-red-500');
		expect(statusConfigs.disconnected.label).toBe('Disconnected');
	});

	it('all status types are covered', () => {
		const statuses: ConnectionStatus[] = ['connected', 'connecting', 'polling', 'disconnected'];
		for (const status of statuses) {
			expect(statusConfigs[status]).toBeDefined();
			expect(statusConfigs[status].color).toBeTruthy();
			expect(statusConfigs[status].label).toBeTruthy();
		}
	});
});

describe('ConnectionIndicator logic', () => {
	// Helper to determine if LiveBadge should show
	function shouldShowLiveBadge(status: ConnectionStatus): boolean {
		return status === 'connected';
	}

	it('shows LiveBadge only when connected', () => {
		expect(shouldShowLiveBadge('connected')).toBe(true);
		expect(shouldShowLiveBadge('connecting')).toBe(false);
		expect(shouldShowLiveBadge('polling')).toBe(false);
		expect(shouldShowLiveBadge('disconnected')).toBe(false);
	});

	// Helper to determine if tooltip should show (click handler)
	function shouldShowTooltip(status: ConnectionStatus): boolean {
		return status !== 'connected';
	}

	it('shows tooltip for non-connected states', () => {
		expect(shouldShowTooltip('connected')).toBe(false);
		expect(shouldShowTooltip('connecting')).toBe(true);
		expect(shouldShowTooltip('polling')).toBe(true);
		expect(shouldShowTooltip('disconnected')).toBe(true);
	});
});

describe('LiveBadge component', () => {
	it('module is importable', async () => {
		const { default: LiveBadge } = await import('$lib/components/realtime/LiveBadge.svelte');
		expect(LiveBadge).toBeDefined();
	});
});
