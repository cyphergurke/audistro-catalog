package model

// Report is a moderation signal submitted by a reporter.
type Report struct {
	ReportID        string
	ReporterSubject string
	TargetType      string
	TargetID        string
	ReportType      string
	Evidence        string
	CreatedAt       int64
}
