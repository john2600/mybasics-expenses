package reports

// ExportFilter holds parameters for an export request.
type ExportFilter struct {
	Months int
	Format string
	UserID int // owner scope; set by the handler, not from client input
}

// ExportRow represents a single expense in the export.
type ExportRow struct {
	ID          int64   `json:"id"`
	Date        string  `json:"date"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

// MonthlySummary holds the aggregated expense total per month.
type MonthlySummary struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Label string  `json:"label"`
	Total float64 `json:"total"`
}

// ExportReport is the full export payload.
type ExportReport struct {
	PeriodFrom     string           `json:"period_from"`
	PeriodTo       string           `json:"period_to"`
	Total          float64          `json:"total"`
	MonthlySummary []MonthlySummary `json:"monthly_summary"`
	Expenses       []ExportRow      `json:"expenses"`
}
