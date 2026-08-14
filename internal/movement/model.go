package movement

import "time"

// Movement represents a single financial movement (income or expense).
type Movement struct {
	ID          int64     `json:"id"`
	CategoryID  int64     `json:"category_id"`
	Category    string    `json:"category,omitempty"`
	Type        string    `json:"type"` // 'I' = ingreso, 'E' = egreso
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	Hour        *string   `json:"hour,omitempty"` // format HH:MM:SS, optional
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateRequest is the payload for creating a new movement.
type CreateRequest struct {
	CategoryID  int64   `json:"category_id"`
	Type        string  `json:"type"` // 'I' or 'E'
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Date        string  `json:"date"` // Expected format: YYYY-MM-DD
	Hour        string  `json:"hour"` // Expected format: HH:MM or HH:MM:SS, optional
}

// UpdateRequest is the payload for partially updating a movement.
type UpdateRequest struct {
	CategoryID  *int64   `json:"category_id,omitempty"`
	Type        *string  `json:"type,omitempty"`
	Amount      *float64 `json:"amount,omitempty"`
	Description *string  `json:"description,omitempty"`
	Date        *string  `json:"date,omitempty"` // Expected format: YYYY-MM-DD
	Hour        *string  `json:"hour,omitempty"`
}

// Filter holds query parameters for listing movements.
type Filter struct {
	UserID     int // owner scope; set by the handler, not from client input
	CategoryID *int64
	Type       *string
	DateFrom   *time.Time
	DateTo     *time.Time
	Limit      *int
}

// GroupedByCategory holds movements grouped by category with a subtotal.
type GroupedByCategory struct {
	Category  string     `json:"category"`
	Total     float64    `json:"total"`
	Movements []Movement `json:"movements"`
}

// MonthlySummary holds the aggregated movement total per month.
type MonthlySummary struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Total float64 `json:"total"`
}

// ExpenseList is the flat expenses listing plus the total amount of the
// expenses matching the applied filter. The total is summed in memory from the
// same expenses being returned (no extra query). With no filter it is the total
// of all expenses.
type ExpenseList struct {
	Total     float64    `json:"total"`
	Movements []Movement `json:"movements"`
}
