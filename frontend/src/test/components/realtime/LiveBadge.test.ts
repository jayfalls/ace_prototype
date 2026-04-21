import { describe, it, expect, vi, beforeEach } from 'vitest';

describe('LiveBadge', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('module is importable', async () => {
		const { default: LiveBadge } = await import('$lib/components/realtime/LiveBadge.svelte');
		expect(LiveBadge).toBeDefined();
	});
});
