package models

import "time"

type BodyMetrics struct {
	ID         int64     `db:"id"`
	MetricName string    `db:"metric_name"`
	Value      float64   `db:"value"`
	Unit       string    `db:"unit"`
	Timestamp  time.Time `db:"timestamp"`
}
