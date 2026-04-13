import { describe, it, expect } from 'vitest';
import {
	formatDate,
	formatDateTime,
	formatRelativeTime,
	formatDuration,
	formatCost,
	formatNumber,
	parseUserAgent,
	roleBadgeVariant,
	statusBadgeVariant,
} from '$lib/utils/formatter';
import type { UserRole, UserStatus } from '$lib/api/types';

describe('formatDate', () => {
	it('formats ISO date string correctly', () => {
		const result = formatDate('2026-01-15T10:30:00Z');
		expect(result).toBe('Jan 15, 2026');
	});

	it('formats date with single digit day', () => {
		const result = formatDate('2026-03-05T00:00:00Z');
		expect(result).toBe('Mar 5, 2026');
	});

	it('formats date with single digit month', () => {
		const result = formatDate('2026-12-25T00:00:00Z');
		expect(result).toBe('Dec 25, 2026');
	});

	it('returns dash for null', () => {
		expect(formatDate(null)).toBe('-');
	});

	it('returns dash for undefined', () => {
		expect(formatDate(undefined)).toBe('-');
	});

	it('returns dash for empty string', () => {
		expect(formatDate('')).toBe('-');
	});

	it('returns dash for invalid date', () => {
		expect(formatDate('not-a-date')).toBe('-');
	});
});

describe('formatDateTime', () => {
	it('formats datetime correctly', () => {
		const result = formatDateTime('2026-01-15T14:30:00Z');
		expect(result).toBe('Jan 15, 2026, 2:30 PM');
	});

	it('formats AM time correctly', () => {
		const result = formatDateTime('2026-01-15T09:15:00Z');
		expect(result).toBe('Jan 15, 2026, 9:15 AM');
	});

	it('formats midnight correctly', () => {
		const result = formatDateTime('2026-01-15T00:00:00Z');
		expect(result).toBe('Jan 15, 2026, 12:00 AM');
	});

	it('returns dash for null', () => {
		expect(formatDateTime(null)).toBe('-');
	});

	it('returns dash for undefined', () => {
		expect(formatDateTime(undefined)).toBe('-');
	});

	it('returns dash for empty string', () => {
		expect(formatDateTime('')).toBe('-');
	});

	it('returns dash for invalid date', () => {
		expect(formatDateTime('not-a-date')).toBe('-');
	});
});

describe('formatRelativeTime', () => {
	it('returns just now for recent times', () => {
		const now = new Date();
		const result = formatRelativeTime(now.toISOString());
		expect(result).toBe('just now');
	});

	it('returns minutes ago for times in the past few minutes', () => {
		const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
		const result = formatRelativeTime(fiveMinutesAgo.toISOString());
		expect(result).toBe('5 minutes ago');
	});

	it('returns hours ago for times in the past few hours', () => {
		const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000);
		const result = formatRelativeTime(twoHoursAgo.toISOString());
		expect(result).toBe('2 hours ago');
	});

	it('returns days ago for times in the past few days', () => {
		const threeDaysAgo = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000);
		const result = formatRelativeTime(threeDaysAgo.toISOString());
		expect(result).toBe('3 days ago');
	});

	it('returns dash for null', () => {
		expect(formatRelativeTime(null)).toBe('-');
	});

	it('returns dash for undefined', () => {
		expect(formatRelativeTime(undefined)).toBe('-');
	});

	it('returns dash for empty string', () => {
		expect(formatRelativeTime('')).toBe('-');
	});
});

describe('formatDuration', () => {
	it('formats milliseconds', () => {
		expect(formatDuration(250)).toBe('250ms');
	});

	it('formats seconds', () => {
		expect(formatDuration(1500)).toBe('1.5s');
	});

	it('formats minutes and seconds', () => {
		expect(formatDuration(65000)).toBe('1m 5s');
	});

	it('formats just minutes when no remaining seconds', () => {
		expect(formatDuration(60000)).toBe('1m');
	});

	it('formats hours and minutes', () => {
		expect(formatDuration(90 * 60 * 1000)).toBe('1h 30m');
	});

	it('formats just hours when no remaining minutes', () => {
		expect(formatDuration(2 * 60 * 60 * 1000)).toBe('2h');
	});

	it('returns dash for null', () => {
		expect(formatDuration(null)).toBe('-');
	});

	it('returns dash for undefined', () => {
		expect(formatDuration(undefined)).toBe('-');
	});

	it('returns dash for negative values', () => {
		expect(formatDuration(-1000)).toBe('-');
	});

	it('returns dash for zero', () => {
		expect(formatDuration(0)).toBe('-');
	});
});

describe('formatCost', () => {
	it('formats dollars correctly', () => {
		expect(formatCost(12.5)).toBe('$12.50');
	});

	it('formats small costs with more precision', () => {
		expect(formatCost(0.001234)).toBe('$0.0012');
	});

	it('formats zero as zero dollars', () => {
		expect(formatCost(0)).toBe('$0.00');
	});

	it('formats negative as dash', () => {
		expect(formatCost(-5)).toBe('-');
	});

	it('returns dash for null', () => {
		expect(formatCost(null)).toBe('-');
	});

	it('returns dash for undefined', () => {
		expect(formatCost(undefined)).toBe('-');
	});
});

describe('formatNumber', () => {
	it('formats thousands with comma', () => {
		expect(formatNumber(1234)).toBe('1,234');
	});

	it('formats millions with M suffix', () => {
		expect(formatNumber(1500000)).toBe('1.5M');
	});

	it('formats thousands with K suffix', () => {
		expect(formatNumber(15000)).toBe('15.0K');
	});

	it('formats zero', () => {
		expect(formatNumber(0)).toBe('0');
	});

	it('returns dash for negative', () => {
		expect(formatNumber(-100)).toBe('-');
	});

	it('returns dash for null', () => {
		expect(formatNumber(null)).toBe('-');
	});

	it('returns dash for undefined', () => {
		expect(formatNumber(undefined)).toBe('-');
	});
});

describe('parseUserAgent', () => {
	it('parses Chrome on macOS', () => {
		const ua =
			'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36';
		const result = parseUserAgent(ua);
		expect(result.browser).toBe('Chrome');
		expect(result.os).toBe('macOS');
		expect(result.device).toBe('Desktop');
	});

	it('parses Safari on macOS', () => {
		const ua =
			'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15';
		const result = parseUserAgent(ua);
		expect(result.browser).toBe('Safari');
		expect(result.os).toBe('macOS');
	});

	it('parses Firefox on Windows', () => {
		const ua =
			'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0';
		const result = parseUserAgent(ua);
		expect(result.browser).toBe('Firefox');
		expect(result.os).toBe('Windows');
	});

	it('parses Edge on Windows', () => {
		const ua =
			'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0';
		const result = parseUserAgent(ua);
		expect(result.browser).toBe('Edge');
		expect(result.os).toBe('Windows');
	});

	it('parses mobile Chrome on Android', () => {
		const ua =
			'Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36';
		const result = parseUserAgent(ua);
		expect(result.browser).toBe('Chrome');
		expect(result.os).toBe('Android');
		expect(result.device).toBe('Mobile');
	});

	it('parses iOS Safari on iPhone', () => {
		const ua =
			'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1';
		const result = parseUserAgent(ua);
		expect(result.browser).toBe('Safari');
		expect(result.os).toBe('iOS');
		expect(result.device).toBe('Mobile');
	});

	it('returns Unknown for null', () => {
		const result = parseUserAgent(null);
		expect(result.browser).toBe('Unknown');
		expect(result.os).toBe('Unknown');
		expect(result.device).toBe('Unknown');
	});

	it('returns Unknown for undefined', () => {
		const result = parseUserAgent(undefined);
		expect(result.browser).toBe('Unknown');
	});

	it('returns Unknown for empty string', () => {
		const result = parseUserAgent('');
		expect(result.browser).toBe('Unknown');
	});
});

describe('roleBadgeVariant', () => {
	it('returns error for admin', () => {
		expect(roleBadgeVariant('admin' as UserRole)).toBe('error');
	});

	it('returns success for user', () => {
		expect(roleBadgeVariant('user' as UserRole)).toBe('success');
	});

	it('returns default for viewer', () => {
		expect(roleBadgeVariant('viewer' as UserRole)).toBe('default');
	});
});

describe('statusBadgeVariant', () => {
	it('returns success for active', () => {
		expect(statusBadgeVariant('active' as UserStatus)).toBe('success');
	});

	it('returns warning for pending', () => {
		expect(statusBadgeVariant('pending' as UserStatus)).toBe('warning');
	});

	it('returns error for suspended', () => {
		expect(statusBadgeVariant('suspended' as UserStatus)).toBe('error');
	});

	it('returns default for unknown status', () => {
		expect(statusBadgeVariant('unknown' as UserStatus)).toBe('default');
	});
});
