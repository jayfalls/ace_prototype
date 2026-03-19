import {
    trace,
    context,
    propagation,
    SpanStatusCode,
    ROOT_CONTEXT,
    type Span,
    type SpanOptions,
} from '@opentelemetry/api';
import {
    WebTracerProvider,
} from '@opentelemetry/sdk-trace-web';
import {
    BatchSpanProcessor,
} from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import {
    Resource,
} from '@opentelemetry/resources';
import {
    ATTR_SERVICE_NAME,
    SEMRESATTRS_DEPLOYMENT_ENVIRONMENT,
} from '@opentelemetry/semantic-conventions';
import { ZoneContextManager } from '@opentelemetry/context-zone';

let provider: WebTracerProvider | null = null;
let globalTraceId: string | null = null;

/**
 * Initialize OpenTelemetry tracing for the browser
 * @param serviceName - The name of the service (e.g., "ace-frontend")
 * @param otelCollectorUrl - The URL of the OTel Collector (e.g., "http://localhost:4318")
 */
export function init(serviceName: string, otelCollectorUrl: string): void {
    // Create the OTLP HTTP exporter
    const exporter = new OTLPTraceExporter({
        url: `${otelCollectorUrl}/v1/traces`,
    });

    // Create the resource with service information
    const resource = new Resource({
        [ATTR_SERVICE_NAME]: serviceName,
        [SEMRESATTRS_DEPLOYMENT_ENVIRONMENT]: import.meta.env.MODE || 'development',
    });

    // Create the WebTracerProvider with the resource
    provider = new WebTracerProvider({
        resource,
    });

    // Add the batch span processor with the exporter
    provider.addSpanProcessor(new BatchSpanProcessor(exporter));

    // Set up the context manager for zone.js support
    const contextManager = new ZoneContextManager();
    provider.register({
        contextManager,
    });

    // Set global trace provider
    trace.setGlobalTracerProvider(provider);

    console.log(`[telemetry] Initialized tracing for ${serviceName} with OTel Collector at ${otelCollectorUrl}`);
}

/**
 * Get the current trace ID from the active span
 * @returns The trace ID as a hex string, or null if no active span
 */
export function getTraceId(): string | null {
    const activeSpan = trace.getActiveSpan();
    
    if (activeSpan) {
        return activeSpan.spanContext().traceId;
    }
    
    return globalTraceId;
}

/**
 * Set a global trace ID that persists across requests
 * This is useful for correlating frontend and backend traces
 * @param traceId - The trace ID to set
 */
export function setGlobalTraceId(traceId: string): void {
    globalTraceId = traceId;
}

/**
 * Start a new span with the given name
 * @param name - The name of the span
 * @param options - Optional span options
 * @returns The created span
 */
export function startSpan(name: string, options?: {
    attributes?: Record<string, string | number | boolean>;
}): Span {
    const tracer = trace.getTracer('ace-frontend');
    return tracer.startSpan(name, {
        attributes: options?.attributes,
    } as SpanOptions);
}

/**
 * Run a function within a span
 * @param name - The name of the span
 * @param fn - The function to run
 * @param options - Optional span options
 * @returns The result of the function
 */
export async function withSpan<T>(
    name: string,
    fn: () => Promise<T>,
    options?: {
        attributes?: Record<string, string | number | boolean>;
    }
): Promise<T> {
    const tracer = trace.getTracer('ace-frontend');
    return tracer.startActiveSpan(name, async (span) => {
        try {
            if (options?.attributes) {
                span.setAttributes(options.attributes);
            }
            const result = await fn();
            span.setStatus({ code: SpanStatusCode.OK });
            return result;
        } catch (error) {
            span.setStatus({
                code: SpanStatusCode.ERROR,
                message: error instanceof Error ? error.message : String(error),
            });
            span.recordException(error instanceof Error ? error : new Error(String(error)));
            throw error;
        } finally {
            span.end();
        }
    });
}

/**
 * Get the current context
 * @returns The current context
 */
export function getCurrentContext(): Context {
    return context.active();
}

/**
 * Context type for the API
 */
type Context = ReturnType<typeof context.active>;

/**
 * Run a function with a specific context
 * @param ctx - The context to use
 * @param fn - The function to run
 * @returns The result of the function
 */
export function withContext<T>(ctx: typeof ROOT_CONTEXT, fn: () => T): T {
    return context.with(ctx, fn);
}

/**
 * Inject trace context into a carrier (for manual propagation)
 * @param carrier - The carrier object to inject into
 * @returns The carrier with trace context headers
 */
export function injectContext(carrier: Record<string, string>): Record<string, string> {
    propagation.inject(context.active(), carrier);
    return carrier;
}

/**
 * Extract trace context from a carrier
 * @param carrier - The carrier to extract from
 * @returns The extracted context
 */
export function extractContext(carrier: Record<string, string | string[]>): typeof ROOT_CONTEXT {
    return propagation.extract(context.active(), carrier as Record<string, string>);
}

/**
 * Shutdown the tracer provider
 * This should be called when the application is shutting down
 * @returns Promise that resolves when shutdown is complete
 */
export async function shutdown(): Promise<void> {
    if (provider) {
        await provider.shutdown();
        provider = null;
        console.log('[telemetry] Tracer provider shut down');
    }
}
