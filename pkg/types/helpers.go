package types

import "github.com/shopspring/decimal"

func PriceFromFloat(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f)
}

func MaxPrice(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

func MinPrice(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

func DecimalToFloat(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
