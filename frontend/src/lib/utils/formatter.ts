import type { UserRole, UserStatus } from '$lib/api/types';

const MONTHS = [
	'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
	'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
];

function padZero(n: number): string {
	return n < 10 ? `0${n}` : `${n}`;
}

export function formatDate(isoString: string | null | undefined): string {
	if (!isoString) return '-';
	const d = new Date(isoString);
	if (isNaN(d.getTime())) return '-';
	return `${MONTHS[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`;
}

export function formatDateTime(isoString: string | null | undefined): string {
	if (!isoString) return '-';
	const d = new Date(isoString);
	if (isNaN(d.getTime())) return '-';
	const hours = d.getUTCHours();
	const minutes = padZero(d.getUTCMinutes());
	const ampm = hours >= 12 ? 'PM' : 'AM';
	const displayHours = hours % 12 || 12;
	return `${MONTHS[d.getUTCMonth()]} ${d.getUTCDate()}, ${d.getUTCFullYear()}, ${displayHours}:${minutes} ${ampm}`;
}

export function formatRelativeTime(isoString: string | null | undefined): string {
	if (!isoString) return '-';
	const date = new Date(isoString);
	if (isNaN(date.getTime())) return '-';
	const now = Date.now();
	const diff = now - date.getTime();

	const seconds = Math.floor(diff / 1000);
	if (seconds < 60) return 'just now';

	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} ago`;

	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} ago`;

	const days = Math.floor(hours / 24);
	if (days < 30) return `${days} day${days === 1 ? '' : 's'} ago`;

	const months = Math.floor(days / 30);
	if (months < 12) return `${months} month${months === 1 ? '' : 's'} ago`;

	const years = Math.floor(months / 12);
	return `${years} year${years === 1 ? '' : 's'} ago`;
}

export function formatDuration(ms: number | null | undefined): string {
	if (ms === null || ms === undefined) return '-';
	if (ms <= 0) return '-';
	if (ms < 1000) return `${ms}ms`;

	const seconds = ms / 1000;
	if (seconds < 60) {
		const rounded = Math.round(seconds * 10) / 10;
		return `${rounded}s`;
	}

	const minutes = Math.floor(seconds / 60);
	const remainingSeconds = Math.round(seconds % 60);
	if (minutes < 60) {
		if (remainingSeconds === 0) return `${minutes}m`;
		return `${minutes}m ${remainingSeconds}s`;
	}

	const hours = Math.floor(minutes / 60);
	const remainingMinutes = minutes % 60;
	if (remainingMinutes === 0) return `${hours}h`;
	return `${hours}h ${remainingMinutes}m`;
}

export function formatCost(cost: number | null | undefined): string {
	if (cost === null || cost === undefined) return '-';
	if (cost < 0) return '-';
	if (cost === 0) return '$0.00';
	if (cost < 0.01) {
		return `$${cost.toFixed(4)}`;
	}
	return `$${cost.toFixed(2)}`;
}

export function formatNumber(n: number | null | undefined): string {
	if (n === null || n === undefined) return '-';
	if (n === 0) return '0';
	if (n < 0) return '-';
	if (n >= 1_000_000) {
		return `${(n / 1_000_000).toFixed(1)}M`;
	}
	if (n >= 10_000) {
		return `${(n / 1_000).toFixed(1)}K`;
	}
	return n.toLocaleString('en-US');
}

interface UserAgentInfo {
	browser: string;
	os: string;
	device: string;
}

export function parseUserAgent(ua: string | null | undefined): UserAgentInfo {
	if (!ua) {
		return { browser: 'Unknown', os: 'Unknown', device: 'Unknown' };
	}

	let browser = 'Unknown';
	let os = 'Unknown';
	let device = 'Desktop';

	if (ua.includes('Firefox')) {
		browser = 'Firefox';
	} else if (ua.includes('Chrome') && !ua.includes('Edg')) {
		browser = 'Chrome';
	} else if (ua.includes('Safari') && !ua.includes('Chrome')) {
		browser = 'Safari';
	} else if (ua.includes('Edg')) {
		browser = 'Edge';
	} else if (ua.includes('Opera') || ua.includes('OPR')) {
		browser = 'Opera';
	}

	if (ua.includes('Windows') || ua.includes('Win64') || ua.includes('Win32')) {
		os = 'Windows';
	} else if (ua.includes('iPhone') || ua.includes('iPad') || ua.includes('iOS')) {
		os = 'iOS';
		device = 'Mobile';
	} else if (ua.includes('Android')) {
		os = 'Android';
		device = 'Mobile';
	} else if (ua.includes('Mac OS X') || ua.includes('Macintosh')) {
		os = 'macOS';
	} else if (ua.includes('Linux')) {
		os = 'Linux';
	}

	if (ua.includes('Mobile') || ua.includes('mobi')) {
		device = 'Mobile';
	}

	return { browser, os, device };
}

export function roleBadgeVariant(role: UserRole): 'default' | 'success' | 'warning' | 'error' {
	switch (role) {
		case 'admin':
			return 'error';
		case 'user':
			return 'success';
		case 'viewer':
			return 'default';
		default:
			return 'default';
	}
}

export function statusBadgeVariant(status: UserStatus): 'default' | 'success' | 'warning' | 'error' {
	switch (status) {
		case 'active':
			return 'success';
		case 'pending':
			return 'warning';
		case 'suspended':
			return 'error';
		default:
			return 'default';
	}
}
