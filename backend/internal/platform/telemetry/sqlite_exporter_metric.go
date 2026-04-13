package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// SQLiteMetricExporter implements metric.Exporter for SQLite storage.
type SQLiteMetricExporter struct {
	db *sql.DB
}

// NewSQLiteMetricExporter creates a new SQLite metric exporter.
func NewSQLiteMetricExporter(db *sql.DB) (*SQLiteMetricExporter, error) {
	return &SQLiteMetricExporter{db: db}, nil
}

// Temporality returns the temporality for an instrument kind.
func (e *SQLiteMetricExporter) Temporality(k metric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

// Aggregation returns the aggregation to use for an instrument kind.
func (e *SQLiteMetricExporter) Aggregation(k metric.InstrumentKind) metric.Aggregation {
	return metric.DefaultAggregationSelector(k)
}

// Export exports metrics to the SQLite database.
func (e *SQLiteMetricExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	if rm == nil {
		return nil
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ott_metrics (name, type, labels, value, timestamp)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	ts := time.Now().Format(time.RFC3339)

	for _, scopeMetrics := range rm.ScopeMetrics {
		for _, m := range scopeMetrics.Metrics {
			// Convert metric data to appropriate representation
			var name string
			var metricType string
			var value float64
			var labels map[string]string

			name = m.Name

			switch data := m.Data.(type) {
			case metricdata.Sum[float64]:
				metricType = "counter"
				if len(data.DataPoints) > 0 {
					value = data.DataPoints[len(data.DataPoints)-1].Value
				}
				labels = collectMetricLabels(data.DataPoints[len(data.DataPoints)-1].Attributes)
			case metricdata.Sum[int64]:
				metricType = "counter"
				if len(data.DataPoints) > 0 {
					value = float64(data.DataPoints[len(data.DataPoints)-1].Value)
				}
				labels = collectMetricLabels(data.DataPoints[len(data.DataPoints)-1].Attributes)
			case metricdata.Gauge[float64]:
				metricType = "gauge"
				if len(data.DataPoints) > 0 {
					value = data.DataPoints[len(data.DataPoints)-1].Value
				}
				labels = collectMetricLabels(data.DataPoints[len(data.DataPoints)-1].Attributes)
			case metricdata.Gauge[int64]:
				metricType = "gauge"
				if len(data.DataPoints) > 0 {
					value = float64(data.DataPoints[len(data.DataPoints)-1].Value)
				}
				labels = collectMetricLabels(data.DataPoints[len(data.DataPoints)-1].Attributes)
			case metricdata.Histogram[float64]:
				metricType = "histogram"
				if len(data.DataPoints) > 0 {
					value = data.DataPoints[len(data.DataPoints)-1].Sum
				}
				labels = collectMetricLabels(data.DataPoints[len(data.DataPoints)-1].Attributes)
			default:
				metricType = "unknown"
				value = 0
				labels = make(map[string]string)
			}

			labelsJSON, err := json.Marshal(labels)
			if err != nil {
				labelsJSON = []byte("{}")
			}

			_, err = stmt.ExecContext(ctx, name, metricType, string(labelsJSON), value, ts)
			if err != nil {
				return fmt.Errorf("insert metric: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// collectMetricLabels extracts labels from an attribute.Set.
func collectMetricLabels(set attribute.Set) map[string]string {
	labels := make(map[string]string)
	iter := set.Iter()
	for iter.Next() {
		attr := iter.Attribute()
		labels[string(attr.Key)] = fmt.Sprintf("%v", attr.Value)
	}
	return labels
}

// Shutdown shuts down the exporter.
func (e *SQLiteMetricExporter) Shutdown(ctx context.Context) error {
	return nil
}

// ForceFlush flushes any pending data.
func (e *SQLiteMetricExporter) ForceFlush(ctx context.Context) error {
	return nil
}
