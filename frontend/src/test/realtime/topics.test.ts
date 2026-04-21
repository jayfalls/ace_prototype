import { describe, it, expect } from 'vitest';
import { parseTopic, buildTopic, isValidTopic, getResyncEndpoint } from '$lib/realtime/topics';

describe('topics', () => {
	describe('parseTopic', () => {
		it('parses valid topic strings', () => {
			expect(parseTopic('agent:123:status')).toEqual({
				resourceType: 'agent',
				resourceId: '123',
				subType: 'status'
			});
			expect(parseTopic('agent:abc-def:logs')).toEqual({
				resourceType: 'agent',
				resourceId: 'abc-def',
				subType: 'logs'
			});
			expect(parseTopic('system:health:check')).toEqual({
				resourceType: 'system',
				resourceId: 'health',
				subType: 'check'
			});
			expect(parseTopic('usage:user123:cost')).toEqual({
				resourceType: 'usage',
				resourceId: 'user123',
				subType: 'cost'
			});
		});

		it('returns null for invalid topic strings', () => {
			expect(parseTopic('')).toBeNull();
			expect(parseTopic('invalid')).toBeNull();
			expect(parseTopic('too:many:parts')).toBeNull();
			expect(parseTopic('UPPER:CASE:topic')).toBeNull();
			expect(parseTopic('agent:123')).toBeNull();
			expect(parseTopic('agent::status')).toBeNull();
			expect(parseTopic(':123:status')).toBeNull();
		});
	});

	describe('buildTopic', () => {
		it('builds topic strings from parts', () => {
			expect(buildTopic('agent', '123', 'status')).toBe('agent:123:status');
			expect(buildTopic('system', 'health', 'check')).toBe('system:health:check');
			expect(buildTopic('usage', 'user123', 'cost')).toBe('usage:user123:cost');
		});

		it('round-trips with parseTopic', () => {
			const original = 'agent:456:logs';
			const parsed = parseTopic(original);
			expect(parsed).not.toBeNull();
			expect(buildTopic(parsed!.resourceType, parsed!.resourceId, parsed!.subType)).toBe(original);
		});
	});

	describe('isValidTopic', () => {
		it('returns true for valid topics', () => {
			expect(isValidTopic('agent:123:status')).toBe(true);
			expect(isValidTopic('system:health:ok')).toBe(true);
			expect(isValidTopic('usage:abc-123:cost')).toBe(true);
		});

		it('returns false for invalid topics', () => {
			expect(isValidTopic('')).toBe(false);
			expect(isValidTopic('invalid')).toBe(false);
			expect(isValidTopic('a:b')).toBe(false);
			expect(isValidTopic('a:b:c:d')).toBe(false);
		});
	});

	describe('getResyncEndpoint', () => {
		it('maps agent topics to /agents/:id', () => {
			expect(getResyncEndpoint('agent:123:status')).toBe('/agents/123');
			expect(getResyncEndpoint('agent:abc-def:logs')).toBe('/agents/abc-def');
		});

		it('maps system topics to /health', () => {
			expect(getResyncEndpoint('system:health:ok')).toBe('/health');
		});

		it('maps usage topics to /usage/:id', () => {
			expect(getResyncEndpoint('usage:user123:cost')).toBe('/usage/user123');
		});

		it('returns null for unknown resource types', () => {
			expect(getResyncEndpoint('unknown:type:name')).toBeNull();
		});

		it('returns null for invalid topics', () => {
			expect(getResyncEndpoint('invalid')).toBeNull();
			expect(getResyncEndpoint('')).toBeNull();
		});
	});
});
