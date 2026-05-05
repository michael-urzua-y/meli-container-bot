package logic

import (
	"os"
	"strconv"
)

type PricingResult struct {
	SalePrice float64
	MeliFee   float64
	NetProfit float64
	TotalCost float64
}

// CalculateIdealPrice calcula a cuánto vender en Meli para ganar un margen fijo.
// Si el costo del producto supera el umbral de envío gratis de Supletech,
// no se suma el costo de envío al cálculo.
func CalculateIdealPrice(costPrice, shippingToMe, desiredProfit float64) float64 {
	commission := envFloat("MELI_COMMISSION", 0.15)
	fixedFee := envFloat("MELI_FIXED_FEE", 950)
	freeShipThreshold := envFloat("FREE_SHIPPING_THRESHOLD", 19990)
	meliShipCost := envFloat("MELI_SHIPPING_COST", 4500)
	supletechFreeShip := envFloat("SUPLETECH_FREE_SHIP", 49990)

	// Si el producto cuesta ≥ $49.990 en Supletech, no cobran envío
	shipping := shippingToMe
	if costPrice >= supletechFreeShip {
		shipping = 0
	}

	price := (costPrice + shipping + desiredProfit + fixedFee) / (1 - commission)

	// Si supera umbral MeLi, se activa envío gratis obligatorio
	if price >= freeShipThreshold {
		price = (costPrice + shipping + desiredProfit + meliShipCost) / (1 - commission)
	}

	// Redondeo chileno (ej: 14990)
	return float64(int(price/100)*100 + 90)
}

func envFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}
