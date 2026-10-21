package money

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// NUMERIC(15,2) stores up to 13 integral digits and two fractional digits.
// Its maximum in cents is below 2^53, so each cent maps to a distinct float64
// value even though arithmetic and unvalidated JSON lexemes remain unsafe.
const MaxCents = 999_999_999_999_999

// Cents is the exact, signed monetary representation used for calculations.
type Cents int64

// FromFloat64 canonicalizes a legacy value that has already been validated at
// ingress. Exact JSON lexeme validation must happen before conversion to float64.
func FromFloat64(value float64) (Cents, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) >= 10_000_000_000_001 {
		return 0, fmt.Errorf("valor monetário fora do limite")
	}
	canonical := strconv.FormatFloat(value, 'f', 2, 64)
	parsed, err := strconv.ParseFloat(canonical, 64)
	if err != nil || math.Abs(value-parsed) > 1e-7 {
		return 0, fmt.Errorf("valor monetário contém fração de centavo")
	}
	negative := strings.HasPrefix(canonical, "-")
	if negative {
		canonical = canonical[1:]
	}
	parts := strings.SplitN(canonical, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	fraction, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, err
	}
	cents := Cents(whole*100 + fraction)
	if negative {
		cents = -cents
	}
	if cents > MaxCents || cents < -MaxCents {
		return 0, fmt.Errorf("valor monetário fora do limite")
	}
	return cents, nil
}

func Positive(value float64) (Cents, error) {
	cents, err := FromFloat64(value)
	if err != nil || cents <= 0 {
		return 0, fmt.Errorf("valor monetário deve ser positivo e em centavos")
	}
	return cents, nil
}

func EqualFloat64(left, right float64) bool {
	a, err := FromFloat64(left)
	if err != nil {
		return false
	}
	b, err := FromFloat64(right)
	return err == nil && a == b
}

func (c Cents) String() string {
	negative := c < 0
	if negative {
		c = -c
	}
	s := fmt.Sprintf("%d.%02d", c/100, c%100)
	if negative {
		return "-" + s
	}
	return s
}

func (c Cents) Float64() float64 { return float64(c) / 100 }

// Percent rounds half up to the nearest cent. percentage is represented in
// hundredths of one percent (500 basis points means 5.00%).
func (c Cents) Percent(basisPoints int64) (Cents, error) {
	if c < 0 || basisPoints < 0 || basisPoints > 10_000 || c > Cents(MaxCents) {
		return 0, fmt.Errorf("percentual ou valor monetário inválido")
	}
	whole := int64(c) / 10_000
	remainder := int64(c) % 10_000
	result := whole*basisPoints + (remainder*basisPoints+5_000)/10_000
	if result > MaxCents {
		return 0, fmt.Errorf("resultado monetário fora do limite")
	}
	return Cents(result), nil
}

func BasisPoints(percent float64) (int64, error) {
	if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 || percent > 100 || math.Abs(percent*100-math.Round(percent*100)) > 1e-7 {
		return 0, fmt.Errorf("percentual inválido")
	}
	return int64(math.Round(percent * 100)), nil
}
