package repo

import (
	"context"

	"github.com/cyphergurke/audistro-catalog/internal/model"
)

type InsertReportParams struct {
	ReportID        string
	ReporterSubject string
	TargetType      string
	TargetID        string
	ReportType      string
	Evidence        string
}

type ReportsRepository interface {
	InsertReport(ctx context.Context, params InsertReportParams) (model.Report, error)
	CountReports(ctx context.Context, targetType string, targetID string, reportType string, sinceUnix int64) (int64, error)
}
