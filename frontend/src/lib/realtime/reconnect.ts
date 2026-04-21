const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;
const RECONNECT_MAX_ATTEMPTS = 5;

export class ReconnectManager {
	private attempt = 0;

	shouldRetry(attempt: number): boolean {
		return attempt < RECONNECT_MAX_ATTEMPTS;
	}

	getDelay(attempt: number): number {
		const delay = RECONNECT_BASE_MS * Math.pow(2, attempt - 1);
		return Math.min(delay, RECONNECT_MAX_MS);
	}

	reset(): void {
		this.attempt = 0;
	}

	getAttempt(): number {
		return this.attempt;
	}

	incrementAttempt(): number {
		this.attempt++;
		return this.attempt;
	}
}
