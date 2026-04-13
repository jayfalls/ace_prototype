export interface Toast {
	id: string;
	variant: 'success' | 'error' | 'warning' | 'info';
	title: string;
	description?: string;
	duration: number;
	createdAt: number;
}

class NotificationStore {
	toasts = $state<Toast[]>([]);

	add(
		title: string,
		variant: Toast['variant'] = 'info',
		description?: string,
		duration: number = 5000
	): string {
		const id = crypto.randomUUID();
		const toast: Toast = {
			id,
			variant,
			title,
			description,
			duration,
			createdAt: Date.now()
		};
		this.toasts.push(toast);

		if (duration > 0) {
			setTimeout(() => this.dismiss(id), duration);
		}

		return id;
	}

	dismiss(id: string): void {
		const index = this.toasts.findIndex((t) => t.id === id);
		if (index !== -1) {
			this.toasts.splice(index, 1);
		}
	}

	success(title: string, description?: string): string {
		return this.add(title, 'success', description);
	}

	error(title: string, description?: string): string {
		return this.add(title, 'error', description, 8000);
	}

	warning(title: string, description?: string): string {
		return this.add(title, 'warning', description);
	}

	info(title: string, description?: string): string {
		return this.add(title, 'info', description);
	}
}

export const notificationStore = new NotificationStore();
