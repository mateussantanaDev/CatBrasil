package domain

import (
	"context"
	"math"
	"time"
)

// RouletteSlice representa uma fatia individual da Roleta Premiada
type RouletteSlice struct {
	ID          int       `json:"id"`
	Label       string    `json:"label"`
	Amount      float64   `json:"amount"`
	Probability float64   `json:"probability"` // em porcentagem (ex: 20.00)
	Color       string    `json:"color"`
	TextColor   string    `json:"textColor"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// RouletteRules representa as regras comportamentais da roleta
type RouletteRules struct {
	StrictModalMode      bool `json:"strictModalMode"`
	RequireDepositToSpin bool `json:"requireDepositToSpin"`
}

// RouletteMetrics representa as métricas matemáticas e o Valor Esperado (EV) da matriz
type RouletteMetrics struct {
	TotalProb       float64 `json:"totalProb"`
	ExpectedValueEV float64 `json:"expectedValueEV"`
	IsProbValid     bool    `json:"isProbValid"`
}

// RouletteMatrix consolida a matriz completa retornada ao ERP
type RouletteMatrix struct {
	Slices  []*RouletteSlice `json:"slices"`
	Metrics RouletteMetrics  `json:"metrics"`
	Rules   RouletteRules    `json:"rules"`
}

// CalculateExpectedValue calcula o Valor Esperado EV em R$: EV = Σ(amount_i * (prob_i / 100))
func CalculateExpectedValue(slices []*RouletteSlice) float64 {
	ev := 0.0
	for _, s := range slices {
		ev += s.Amount * (s.Probability / 100.0)
	}
	return math.Round(ev*100) / 100
}

// ValidateProbabilities verifica se a soma das probabilidades totaliza 100.0%
func ValidateProbabilities(slices []*RouletteSlice) (totalProb float64, isValid bool) {
	total := 0.0
	for _, s := range slices {
		total += s.Probability
	}
	totalProb = math.Round(total*1000) / 1000
	isValid = math.Abs(totalProb-100.0) < 0.001
	return totalProb, isValid
}

// RouletteRepository define o contrato da camada de persistência para a Roleta Diária
type RouletteRepository interface {
	GetSlices(ctx context.Context) ([]*RouletteSlice, error)
	SaveSlices(ctx context.Context, slices []*RouletteSlice) error
	GetSliceByID(ctx context.Context, id int) (*RouletteSlice, error)
}
