const TOPIC_REGEX = /^[a-z0-9]+:[a-z0-9-]+:[a-z0-9_]+$/;

const VALID_RESOURCE_TYPES = new Set(['agent', 'system', 'usage']);

export interface ParsedTopic {
	resourceType: string;
	resourceId: string;
	subType: string;
}

export function parseTopic(topic: string): ParsedTopic | null {
	const parts = topic.split(':');
	if (parts.length !== 3) {
		return null;
	}
	const [resourceType, resourceId, subType] = parts;
	if (!TOPIC_REGEX.test(topic)) {
		return null;
	}
	if (!VALID_RESOURCE_TYPES.has(resourceType)) {
		return null;
	}
	return { resourceType, resourceId, subType };
}

export function buildTopic(resourceType: string, resourceId: string, subType: string): string {
	return `${resourceType}:${resourceId}:${subType}`;
}

export function isValidTopic(topic: string): boolean {
	return TOPIC_REGEX.test(topic);
}

export function getResyncEndpoint(topic: string): string | null {
	const parsed = parseTopic(topic);
	if (!parsed) return null;

	const { resourceType, resourceId } = parsed;

	switch (resourceType) {
		case 'agent':
			return `/agents/${resourceId}`;
		case 'system':
			return '/health';
		case 'usage':
			return `/usage/${resourceId}`;
		default:
			return null;
	}
}
