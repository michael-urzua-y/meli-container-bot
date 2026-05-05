package main

import (
	"fmt"
	"log"

	"github.com/michael-urzua-y/meli-container-bot/internal/scraper"
)

func main() {
	url := "https://www.supletech.cl/creatina-drive/p?skuId=663"

	fmt.Printf("🔍 Debug: %s\n\n", url)

	data, err := scraper.GetSupletechData(url)
	if err != nil {
		log.Fatal("❌ Error:", err)
	}

	fmt.Printf("Nombre:     %s\n", data.Name)
	fmt.Printf("Marca:      %s\n", data.Brand)
	fmt.Printf("SKU:        %s\n", data.SKU)
	fmt.Printf("ProductID:  %s\n", data.ProductID)
	fmt.Printf("Precio:     $%.0f\n", data.Price)
	fmt.Printf("ListPrice:  $%.0f\n", data.ListPrice)
	fmt.Printf("Stock:      %d\n", data.Stock)
	fmt.Printf("Disponible: %v\n", data.IsAvailable)
	fmt.Printf("Categoría:  %s\n", data.Category)
	fmt.Printf("Imagen:     %s\n", data.ImageURL)
	fmt.Printf("Info Nutr:  %s\n", data.NutritionInfoURL)
	fmt.Printf("Desc:       %.100s...\n", data.Description)
}
