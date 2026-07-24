package analytics

// Filter holds common query parameters for analytics endpoints.
type Filter struct {
	Months int
	// UserID scopes every analytics query to a single user's movements.
	UserID int
}

// PeakMonth holds the month with the highest spend.
type PeakMonth struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Label string  `json:"label"`
	Total float64 `json:"total"`
}

// Summary is the response for GET /analytics/summary.
type Summary struct {
	PeriodFrom     string    `json:"period_from"`
	PeriodTo       string    `json:"period_to"`
	Months         int       `json:"months"`
	TotalSpent     float64   `json:"total_spent"`
	MonthlyAverage float64   `json:"monthly_average"`
	PeakMonth      PeakMonth `json:"peak_month"`
	ExpenseCount   int       `json:"expense_count"`
}

// CategoryBreakdown holds spend data for a single category.
type CategoryBreakdown struct {
	CategoryID int64   `json:"category_id"`
	Category   string  `json:"category"`
	Total      float64 `json:"total"`
	Percentage float64 `json:"percentage"`
	Count      int     `json:"count"`
}

// ByCategory is the response for GET /analytics/by-category.
type ByCategory struct {
	PeriodFrom string              `json:"period_from"`
	PeriodTo   string              `json:"period_to"`
	TotalSpent float64             `json:"total_spent"`
	Categories []CategoryBreakdown `json:"categories"`
}

// TrendPoint holds aggregated spend for a single month.
type TrendPoint struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Label string  `json:"label"`
	Total float64 `json:"total"`
	Count int     `json:"count"`
}

// Trend is the response for GET /analytics/trend.
type Trend struct {
	PeriodFrom string       `json:"period_from"`
	PeriodTo   string       `json:"period_to"`
	Points     []TrendPoint `json:"points"`
}

// TopExpense represents a single high-value expense.
type TopExpense struct {
	ID          int64   `json:"id"`
	Date        string  `json:"date"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

// TopExpenses is the response for GET /analytics/top-expenses.
type TopExpenses struct {
	PeriodFrom string       `json:"period_from"`
	PeriodTo   string       `json:"period_to"`
	Expenses   []TopExpense `json:"expenses"`
}

// MonthIVE holds income, expense and balance for a single month.
type MonthIVE struct {
	Year    int     `json:"year"`
	Month   int     `json:"month"`
	Label   string  `json:"label"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Balance float64 `json:"balance"`
}

// Totals holds the period-level aggregates for IncomeVsExpense.
type Totals struct {
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Balance float64 `json:"balance"`
}

// IncomeVsExpense is the response for GET /analytics/income-vs-expense.
type IncomeVsExpense struct {
	PeriodFrom string     `json:"period_from"`
	PeriodTo   string     `json:"period_to"`
	Months     []MonthIVE `json:"months"`
	Totals     Totals     `json:"totals"`
}
