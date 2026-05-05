package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/michael-urzua-y/meli-container-bot/internal/logic"
	"github.com/michael-urzua-y/meli-container-bot/internal/meli"
	"github.com/michael-urzua-y/meli-container-bot/internal/scraper"
	"github.com/michael-urzua-y/meli-container-bot/internal/storage"
	"gorm.io/gorm"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error cargando .env:", err)
	}

	db, err := storage.Connect()
	if err != nil {
		log.Fatal(err)
	}
	storage.Migrate(db)

	interval := envInt("SYNC_INTERVAL", 30)

	// Primera ejecución
	runCycle(db)

	if interval <= 0 {
		return
	}

	// Loop automático
	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	defer ticker.Stop()
	fmt.Printf("\n⏰ Próxima sincronización en %d minutos...\n", interval)

	for range ticker.C {
		fmt.Printf("\n🔄 [%s] Sincronización automática...\n", time.Now().Format("15:04:05"))
		runCycle(db)
		fmt.Printf("\n⏰ Próxima sincronización en %d minutos...\n", interval)
	}
}

// runCycle ejecuta un ciclo completo: scrape → BD → publish → sync
func runCycle(db *gorm.DB) {
	// 1. Scrape y actualizar BD
	scrapeProducts(db)

	// 2. Publicar y sincronizar en MeLi
	token, err := meli.GetValidToken()
	if err != nil {
		fmt.Printf("⚠️  Error token MeLi: %v\n", err)
		return
	}
	fmt.Printf("🔑 Token válido (User ID: %d)\n", token.UserID)

	publishNew(db, token.AccessToken)
	syncExisting(db, token.AccessToken)

	fmt.Println("\n✅ Ciclo completo.")
}

func scrapeProducts(db *gorm.DB) {
	urlsRaw := os.Getenv("PRODUCT_URLS")
	if urlsRaw == "" {
		return
	}
	urls := strings.Split(urlsRaw, ",")
	shippingToMe := envFloat("SHIPPING_COST", 3200)
	margenDeseado := envFloat("DESIRED_MARGIN", 4000)

	fmt.Printf("📦 Scrapeando %d productos...\n", len(urls))

	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}

		data, err := scraper.GetSupletechData(url)
		if err != nil {
			fmt.Printf("   ❌ %s: %v\n", extractName(url), err)
			continue
		}

		precio := logic.CalculateIdealPrice(data.Price, shippingToMe, margenDeseado)

		producto := storage.Product{
			SKU: data.SKU, ProductID: data.ProductID,
			Name: data.Name, Brand: data.Brand,
			SourceURL: url, ImageURL: data.ImageURL,
			NutritionInfoURL: data.NutritionInfoURL,
			ExtraImages:      strings.Join(data.ExtraImages, ","),
			Category:    data.Category,
			CostPrice:   data.Price, ListPrice: data.ListPrice,
			ShippingCost: shippingToMe, Description: data.Description,
			DeliveryTime: "1 día hábil",
			MeliPrice:   precio, Stock: data.Stock,
			IsActive: data.IsAvailable,
		}

		db.Where(storage.Product{SKU: data.SKU}).Assign(producto).FirstOrCreate(&producto)
		fmt.Printf("   ✅ %s | $%.0f → $%.0f | Stock: %d\n", data.Name, data.Price, precio, data.Stock)
	}
}

func publishNew(db *gorm.DB, accessToken string) {
	var productos []storage.Product
	db.Where("is_active = ? AND meli_id = ?", true, "").Find(&productos)

	if len(productos) == 0 {
		return
	}

	category := envStr("MELI_CATEGORY", "MLC435304")
	listingType := envStr("MELI_LISTING_TYPE", "gold_special")
	currency := envStr("MELI_CURRENCY", "CLP")
	warrantyType := envStr("MELI_WARRANTY_TYPE", "Garantía del vendedor")
	warrantyTime := envStr("MELI_WARRANTY_TIME", "30 días")

	fmt.Printf("\n🚀 %d productos nuevos para publicar\n", len(productos))

	for _, p := range productos {
		fmt.Printf("   Publicando: %s (%s)...", p.Name, p.Brand)

		item := meli.MeliItem{
			FamilyName:        buildFamilyName(p),
			CategoryID:        category,
			Price:             p.MeliPrice,
			CurrencyID:        currency,
			AvailableQuantity: p.Stock,
			BuyingMode:        "buy_it_now",
			Condition:         "new",
			ListingTypeID:     listingType,
			SaleTerms: []meli.MeliAttribute{
				{ID: "WARRANTY_TYPE", ValueName: warrantyType},
				{ID: "WARRANTY_TIME", ValueName: warrantyTime},
			},
			Pictures:   buildPictures(p),
			Attributes: buildAttributes(p),
		}

		result, err := meli.PublishItem(accessToken, item)
		if err != nil {
			fmt.Printf(" ❌ %v\n", err)
			continue
		}

		db.Model(&p).Update("meli_id", result.ID)

		desc := buildDescription(p)
		if desc != "" {
			meli.AddDescription(accessToken, result.ID, desc)
		}

		fmt.Printf(" ✅ %s\n", result.ID)
	}
}

func syncExisting(db *gorm.DB, accessToken string) {
	var publicados []storage.Product
	db.Where("meli_id != ?", "").Find(&publicados)
	if len(publicados) == 0 {
		return
	}

	fmt.Printf("\n🔄 Sincronizando %d productos...\n", len(publicados))

	for _, p := range publicados {
		// Actualizar fotos
		updatePictures(accessToken, p)

		// Actualizar descripción
		desc := buildDescription(p)
		meli.UpdateDescription(accessToken, p.MeliID, desc)

		// Pausar si sin stock, activar si tiene
		if !p.IsActive || p.Stock == 0 {
			meli.PauseItem(accessToken, p.MeliID)
			fmt.Printf("   ⏸️  %s - sin stock\n", p.Name)
			continue
		}

		updates := map[string]interface{}{
			"price":              p.MeliPrice,
			"available_quantity": p.Stock,
		}
		if err := meli.UpdateItem(accessToken, p.MeliID, updates); err != nil {
			fmt.Printf("   ❌ %s: %v\n", p.Name, err)
		} else {
			fmt.Printf("   ✅ %s | $%.0f | Stock: %d\n", p.Name, p.MeliPrice, p.Stock)
		}
	}
}

func updatePictures(accessToken string, p storage.Product) {
	pics := buildPictures(p)
	if len(pics) == 0 {
		return
	}
	var picMaps []map[string]string
	for _, pic := range pics {
		picMaps = append(picMaps, map[string]string{"source": pic.Source})
	}
	meli.UpdateItem(accessToken, p.MeliID, map[string]interface{}{"pictures": picMaps})
}

func buildFamilyName(p storage.Product) string {
	name := fmt.Sprintf("%s %s", p.Name, p.Brand)
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func buildPictures(p storage.Product) []meli.MeliPicture {
	var pics []meli.MeliPicture
	if p.ImageURL != "" {
		pics = append(pics, meli.MeliPicture{Source: p.ImageURL})
	}
	if p.ExtraImages != "" {
		for _, img := range strings.Split(p.ExtraImages, ",") {
			img = strings.TrimSpace(img)
			if img != "" {
				pics = append(pics, meli.MeliPicture{Source: img})
			}
		}
	}
	return pics
}

// eanBySKU mapeo manual de EAN/UPC para productos que lo requieren
var eanBySKU = map[string]string{
	"664": "631656705737", // Platinum Creatine 400g - Muscletech
	"670": "631656708172", // Creactor Celltech 120 svs - Muscletech
	"669": "748927063691", // Creatine Micronized 300g - Optimum Nutrition
}

func buildAttributes(p storage.Product) []meli.MeliAttribute {
	// Detectar tipo de suplemento por categoría de Supletech
	isProtein := strings.Contains(strings.ToLower(p.Category), "prote") ||
		strings.Contains(strings.ToLower(p.Name), "whey") ||
		strings.Contains(strings.ToLower(p.Name), "iso ") ||
		strings.Contains(strings.ToLower(p.Name), "iso-") ||
		strings.Contains(strings.ToLower(p.Name), "isofit") ||
		strings.Contains(strings.ToLower(p.Name), "prostar")

	var mainSupplement, supplementClass, tradeName meli.MeliAttribute
	if isProtein {
		mainSupplement = meli.MeliAttribute{ID: "MAIN_SUPPLEMENT", ValueID: "6565365", ValueName: "Whey protein"}
		supplementClass = meli.MeliAttribute{ID: "SUPPLEMENT_CLASS", ValueID: "52261838", ValueName: "Proteínas"}
		tradeName = meli.MeliAttribute{ID: "TRADE_NAME", ValueName: "Proteína"}
	} else {
		mainSupplement = meli.MeliAttribute{ID: "MAIN_SUPPLEMENT", ValueID: "6565367", ValueName: "Creatina"}
		supplementClass = meli.MeliAttribute{ID: "SUPPLEMENT_CLASS", ValueID: "52261842", ValueName: "Creatina"}
		tradeName = meli.MeliAttribute{ID: "TRADE_NAME", ValueID: "325946", ValueName: "Creatina"}
	}

	attrs := []meli.MeliAttribute{
		{ID: "BRAND", ValueName: p.Brand},
		{ID: "SELLER_SKU", ValueName: p.SKU},
		{ID: "SUPPLEMENT_FORMAT", ValueID: "4567842", ValueName: "Polvo"},
		mainSupplement,
		{ID: "SUPPLEMENT_TYPE", ValueID: "5406719", ValueName: "Deportivo"},
		supplementClass,
		{ID: "FLAVOR", ValueID: "2517530", ValueName: "Sin sabor"},
		{ID: "SALE_FORMAT", ValueID: "1359391", ValueName: "Unidad"},
		{ID: "UNITS_PER_PACK", ValueName: "1"},
		tradeName,
	}

	ean := p.EAN
	if ean == "" {
		ean = eanBySKU[p.SKU]
	}

	if ean != "" {
		attrs = append(attrs, meli.MeliAttribute{ID: "GTIN", ValueName: ean})
	} else {
		attrs = append(attrs, meli.MeliAttribute{ID: "GTIN", ValueName: "does_not_apply"})
		attrs = append(attrs, meli.MeliAttribute{ID: "EMPTY_GTIN_REASON", ValueID: "17055160"})
	}

	return attrs
}

func buildDescription(p storage.Product) string {
	desc := p.Description
	if desc == "" {
		desc = fmt.Sprintf("%s - %s", p.Name, p.Brand)
	}

	footer := `

---
Envío desde Región Metropolitana.
Despacho en 1 día hábil.
Producto 100%% original, sellado y con garantía.`

	return desc + footer
}

func extractName(url string) string {
	parts := strings.Split(strings.Split(url, "?")[0], "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}
