import { testProvider } from '$lib/api/providers';
import type { ProviderTestResult } from '$lib/api/types';

export type TestState = 'idle' | 'testing' | 'success' | 'error';

export function createTestButtonState(getProviderId: () => string) {
	let state = $state<TestState>('idle');
	let result = $state<ProviderTestResult | null>(null);
	let errorMessage = $state('');

	async function handleTest() {
		state = 'testing';
		result = null;
		errorMessage = '';
		try {
			result = await testProvider(getProviderId());
			state = 'success';
		} catch (err) {
			errorMessage = err instanceof Error ? err.message : 'Test failed';
			state = 'error';
		}
	}

	return {
		get state() {
			return state;
		},
		get result() {
			return result;
		},
		get errorMessage() {
			return errorMessage;
		},
		handleTest
	};
}
