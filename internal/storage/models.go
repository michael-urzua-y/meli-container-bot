package storage

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	SKU       string `gorm:"uniqueIndex"`
	ProductID string
	MeliID    string
	Name      string
	Brand     string
	SourceURL string
	Category  string

	// Imágenes (separadas por coma)
	ImageURL         string // Imagen principal del producto
	NutritionInfoURL string // Imagen de información nutricional
	ExtraImages      string // Imágenes adicionales separadas por coma

	// Precios
	CostPrice    float64
	ListPrice    float64
	ShippingCost float64
	TaxRate      float64 `gorm:"default:0.19"`

	// Descripción para MercadoLibre
	Description string `gorm:"type:text"`

	// Logística
	DeliveryTime string

	// Código de barras (EAN/UPC/GTIN)
	EAN string

	// Meli Data
	MeliPrice float64
	IsActive  bool `gorm:"default:true"`
	Stock     int
}
