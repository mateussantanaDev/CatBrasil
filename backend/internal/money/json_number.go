package money

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var jsonNumberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(?:\.([0-9]+))?(?:[eE]([+-]?[0-9]+))?$`)

// ParseJSONNumber validates the original JSON number, before float64 can erase
// sub-cent digits. Exponents and extra trailing zeros are accepted only when
// they represent an exact integer number of cents within NUMERIC(15,2).
func ParseJSONNumber(raw string) (Cents, error) {
	raw = strings.TrimSpace(raw)
	match := jsonNumberPattern.FindStringSubmatch(raw)
	if match == nil {
		return 0, fmt.Errorf("número monetário JSON inválido")
	}
	exponent := int64(0)
	if match[3] != "" {
		var err error
		exponent, err = strconv.ParseInt(match[3], 10, 32)
		if err != nil || exponent < -100 || exponent > 100 {
			return 0, fmt.Errorf("expoente monetário fora do limite")
		}
	}
	digits := match[1] + match[2]
	mantissa, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return 0, fmt.Errorf("número monetário inválido")
	}
	shift := 2 + exponent - int64(len(match[2]))
	cents := new(big.Int).Set(mantissa)
	if shift >= 0 {
		cents.Mul(cents, new(big.Int).Exp(big.NewInt(10), big.NewInt(shift), nil))
	} else {
		divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(-shift), nil)
		remainder := new(big.Int)
		cents.QuoRem(cents, divisor, remainder)
		if remainder.Sign() != 0 {
			return 0, fmt.Errorf("valor monetário contém fração de centavo")
		}
	}
	if cents.Cmp(big.NewInt(MaxCents)) > 0 {
		return 0, fmt.Errorf("valor monetário fora do limite")
	}
	if strings.HasPrefix(raw, "-") {
		cents.Neg(cents)
	}
	return Cents(cents.Int64()), nil
}
