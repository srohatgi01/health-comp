package repositories

import (
	"database/sql"

	"github.com/srohatgi/health-comp/models"
)

type BodyMetricsRepo struct {
	db *sql.DB
}

func NewBodyMetricsRepo(db *sql.DB) *BodyMetricsRepo {
	return &BodyMetricsRepo{db: db}
}

func (r *BodyMetricsRepo) Create(metric *models.BodyMetrics) error {
	query := `
	INSERT INTO body_metrics (metric_name, value, unit)
	VALUES (?, ?, ?)
	`

	result, err := r.db.Exec(
		query,
		metric.MetricName,
		metric.Value,
		metric.Unit,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	metric.ID = id
	return nil
}
