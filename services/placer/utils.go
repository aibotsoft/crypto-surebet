package placer

import (
	"fmt"
	"github.com/shopspring/decimal"
)

func F3toS(f float64) string {
	return fmt.Sprintf("%.3f", f)
}
func F4toS(f float64) string {
	return fmt.Sprintf("%.4f", f)
}
func BoolPointer(s bool) *bool {
	return &s
}
func DecimalToFloat64(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
