package balance

import (
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/incomes"
)

// Summary holds the computed balance for a date range.
type Summary struct {
	Expenses     float64         `json:"expenses"`
	Incomes      float64         `json:"incomes"`
	Balance      float64         `json:"balance"`
	IncomeConfig *incomes.Config `json:"income_config"`
	CarryOverOut float64		 `json:"carry_over"`
}

// PeriodSummary holds the financial summary for one billing period.
type PeriodSummary struct {
	PeriodStart       time.Time `json:"period_start"`
	PeriodEnd         time.Time `json:"period_end"`
	FixedIncome       float64   `json:"fixed_income"`        // ingreso fijo configurado
	RegisteredIncomes float64   `json:"registered_incomes"`  // ingresos type='I' del periodo
	TotalIncome       float64   `json:"total_income"`        // fixed_income + registered_incomes
	Expenses          float64   `json:"expenses"`            // gastos type='E' del periodo
	CarryOverIn       float64   `json:"carry_over_in"`       // sobrante recibido del periodo anterior
	Balance           float64   `json:"balance"`             // total_income + carry_over_in - expenses
	CarryOverOut      float64   `json:"carry_over_out"`      // sobrante que pasa al siguiente (0 si negativo)
	Deficit           float64   `json:"deficit"`             // déficit del periodo (0 si balance >= 0)
}
