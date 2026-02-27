package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
)

var _ repo.ReportsRepository = (*ReportsRepo)(nil)

type ReportsRepo struct {
	db *sql.DB
}

func NewReportsRepo(db *sql.DB) *ReportsRepo {
	return &ReportsRepo{db: db}
}

func (r *ReportsRepo) InsertReport(ctx context.Context, params repo.InsertReportParams) (model.Report, error) {
	report := model.Report{
		ReportID:        params.ReportID,
		ReporterSubject: params.ReporterSubject,
		TargetType:      params.TargetType,
		TargetID:        params.TargetID,
		ReportType:      params.ReportType,
		Evidence:        params.Evidence,
		CreatedAt:       time.Now().Unix(),
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO reports(report_id, reporter_subject, target_type, target_id, report_type, evidence, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return model.Report{}, fmt.Errorf("prepare insert report: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		report.ReportID,
		report.ReporterSubject,
		report.TargetType,
		report.TargetID,
		report.ReportType,
		report.Evidence,
		report.CreatedAt,
	)
	if err != nil {
		return model.Report{}, fmt.Errorf("insert report: %w", err)
	}

	return report, nil
}

func (r *ReportsRepo) CountReports(ctx context.Context, targetType string, targetID string, reportType string, sinceUnix int64) (int64, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT COUNT(*)
		FROM reports
		WHERE target_type = ? AND target_id = ? AND report_type = ? AND created_at >= ?
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare count reports: %w", err)
	}
	defer stmt.Close()

	var count int64
	if err := stmt.QueryRowContext(ctx, targetType, targetID, reportType, sinceUnix).Scan(&count); err != nil {
		return 0, fmt.Errorf("count reports: %w", err)
	}
	return count, nil
}
