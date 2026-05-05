package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ScrapeResult struct {
	Name             string
	Brand            string
	Price            float64
	ListPrice        float64
	Stock            int
	SKU              string
	ProductID        string
	Description      string
	ImageURL         string
	NutritionInfoURL string
	ExtraImages      []string // Imágenes adicionales (sin la nutricional)
	Category         string
	IsAvailable      bool
}

// vtexProduct representa la respuesta de la API de VTEX
type vtexProduct struct {
	ProductID   string   `json:"productId"`
	ProductName string   `json:"productName"`
	Brand       string   `json:"brand"`
	LinkText    string   `json:"linkText"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
	Items       []struct {
		ItemID string `json:"itemId"`
		Name   string `json:"name"`
		Images []struct {
			ImageURL string `json:"imageUrl"`
		} `json:"images"`
		Sellers []struct {
			CommertialOffer struct {
				Price             float64 `json:"Price"`
				ListPrice         float64 `json:"ListPrice"`
				AvailableQuantity int     `json:"AvailableQuantity"`
				IsAvailable       bool    `json:"IsAvailable"`
			} `json:"commertialOffer"`
		} `json:"sellers"`
	} `json:"items"`
}

// GetSupletechData extrae datos de un producto de Supletech via API VTEX.
// Acepta URLs como: https://www.supletech.cl/creatina-drive/p?skuId=663
func GetSupletechData(productURL string) (ScrapeResult, error) {
	// Extraer el slug del producto de la URL
	slug := extractSlug(productURL)
	if slug == "" {
		return ScrapeResult{}, fmt.Errorf("no se pudo extraer slug de: %s", productURL)
	}

	// Llamar a la API de VTEX
	apiURL := fmt.Sprintf("https://www.supletech.cl/api/catalog_system/pub/products/search/%s/p", slug)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return ScrapeResult{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return ScrapeResult{}, fmt.Errorf("error llamando API VTEX: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ScrapeResult{}, err
	}

	var products []vtexProduct
	if err := json.Unmarshal(body, &products); err != nil {
		return ScrapeResult{}, fmt.Errorf("error parseando JSON: %w", err)
	}

	if len(products) == 0 {
		return ScrapeResult{}, fmt.Errorf("producto no encontrado: %s", slug)
	}

	p := products[0]
	if len(p.Items) == 0 {
		return ScrapeResult{}, fmt.Errorf("producto sin items/SKUs: %s", slug)
	}

	// Buscar el SKU específico si viene en la URL, sino usar el primero
	skuID := extractSkuID(productURL)
	item := p.Items[0]
	if skuID != "" {
		for _, it := range p.Items {
			if it.ItemID == skuID {
				item = it
				break
			}
		}
	}

	var result ScrapeResult
	result.Name = p.ProductName
	result.Brand = p.Brand
	result.ProductID = p.ProductID
	result.SKU = item.ItemID
	result.Description = cleanHTML(p.Description)

	// Categoría (última parte del path)
	if len(p.Categories) > 0 {
		result.Category = p.Categories[0]
	}

	// Imágenes: [0] = producto, [1] = info nutricional, [2+] = extras
	if len(item.Images) > 0 {
		result.ImageURL = item.Images[0].ImageURL
	}
	if len(item.Images) > 1 {
		result.NutritionInfoURL = item.Images[1].ImageURL
	}
	// Capturar imágenes extra (desde la 3ra en adelante, si existen)
	for i := 2; i < len(item.Images); i++ {
		result.ExtraImages = append(result.ExtraImages, item.Images[i].ImageURL)
	}

	// Precio y stock del primer seller
	if len(item.Sellers) > 0 {
		offer := item.Sellers[0].CommertialOffer
		result.Price = offer.Price
		result.ListPrice = offer.ListPrice
		result.Stock = offer.AvailableQuantity
		result.IsAvailable = offer.IsAvailable

		// Capear stock ilimitado de VTEX a 100
		if result.Stock > 1000 {
			result.Stock = 100
		}

		// Desactivar automáticamente si no hay stock
		if result.Stock == 0 {
			result.IsAvailable = false
		}
	}

	return result, nil
}

// extractSlug extrae el slug del producto de una URL de Supletech.
// Ej: "https://www.supletech.cl/creatina-drive/p?skuId=663" → "creatina-drive"
func extractSlug(rawURL string) string {
	// Quitar query params
	url := strings.Split(rawURL, "?")[0]
	// Quitar trailing /p
	url = strings.TrimSuffix(url, "/p")
	url = strings.TrimSuffix(url, "/")
	// Obtener última parte del path
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// extractSkuID extrae el skuId de la URL si existe.
// Ej: "...?skuId=663" → "663"
func extractSkuID(rawURL string) string {
	re := regexp.MustCompile(`skuId=(\d+)`)
	match := re.FindStringSubmatch(rawURL)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// cleanHTML remueve tags HTML y retorna texto plano.
func cleanHTML(html string) string {
	// Remover tags HTML
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")
	// Limpiar espacios múltiples y saltos de línea
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	// Decodificar entidades HTML comunes
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&#8203;", "")
	return text
}
