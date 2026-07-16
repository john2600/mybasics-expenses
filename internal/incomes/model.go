package incomes

import "time"

// Config holds the fixed monthly income and billing cycle cut day
// for a specific month. The effective config for any period is the
// most recent entry whose year_month is <= that period's month.
type Config struct {
	YearMonth   time.Time `json:"year_month"`
	Amount      float64   `json:"amount"`
	CutDay      int       `json:"cut_day"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// UpdateRequest is the payload to create or update an income config entry.
// If YearMonth is omitted, the current month is used.
type UpdateRequest struct {
	YearMonth   *string  `json:"year_month"`  // "YYYY-MM"; defaults to current month
	Amount      *float64 `json:"amount"`
	CutDay      *int     `json:"cut_day"`
	Description *string  `json:"description"`
}
