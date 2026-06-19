package database

import (
	"time"

	"go-ooo/database/models"
)

/*
  Operator-report queries (#126). Read-only bulk loads for the report aggregation in go-ooo/report.
  Kept deliberately simple - they pull the rows and the aggregation happens in Go, matching the
  existing analytics pattern and staying dialect-agnostic across sqlite and Postgres.
*/

// GetRequestsForReport returns data requests created at or after `since`, oldest-first. A zero
// `since` returns the full history.
func (d *DB) GetRequestsForReport(since time.Time) ([]models.DataRequests, error) {
	var rows []models.DataRequests
	q := d.Order("id asc")
	if !since.IsZero() {
		q = q.Where("created_at >= ?", since)
	}
	err := q.Find(&rows).Error
	return rows, err
}

// GetAllFailedFulfilments returns every recorded failed/reverted fulfilment attempt. These are
// windowed in the report by whether their request is in scope, so no time filter is applied here
// (the failures table only ever holds failed attempts, so it stays small).
func (d *DB) GetAllFailedFulfilments() ([]models.FailedFulfilment, error) {
	var rows []models.FailedFulfilment
	err := d.Order("id asc").Find(&rows).Error
	return rows, err
}
