// cmd/publish/main.go — Solo publica/sincroniza en MeLi (sin scrape).
// Para el flujo completo usa: go run cmd/bot/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/michael-urzua-y/meli-container-bot/internal/meli"
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

	token, err := meli.GetValidToken()
	if err != nil {
		log.Fatal("Error token:", err)
	}
	fmt.Printf("🔑 Token válido (User ID: %d)\n", token.UserID)

	publishNew(db, token.AccessToken)
	syncExisting(db, token.AccessToken)
	fmt.Println("\n✅ Publicación completa.")
}

func publishNew(db *gorm.DB, accessToken string) {
	var productos []storage.Product
	db.Where("is_active = ? AND meli_id = ?", true, "").Find(&productos)
	if len(productos) == 0 {
		fmt.Println("No hay productos nuevos para publicar")
		return
	}

	category := envStr("MELI_CATEGORY", "MLC435304")
	listingType := envStr("MELI_LISTING_TYPE", "gold_special")
	currency := envStr("MELI_CURRENCY", "CLP")

	fmt.Printf("🚀 %d productos para publicar\n\n", len(productos))

	for _, p := range productos {
		fmt.Printf("Publicando: %s (%s)\n", p.Name, p.Brand)

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
				{ID: "WARRANTY_TYPE", ValueName: envStr("MELI_WARRANTY_TYPE", "Garantía del vendedor")},
				{ID: "WARRANTY_TIME", ValueName: envStr("MELI_WARRANTY_TIME", "30 días")},
			},
			Pictures:   buildPictures(p),
			Attributes: buildAttributes(p),
		}

		result, err := meli.PublishItem(accessToken, item)
		if err != nil {
			fmt.Printf("   ❌ %v\n", err)
			continue
		}

		db.Model(&p).Update("meli_id", result.ID)

		desc := buildDescription(p)
		if desc != "" {
			meli.AddDescription(accessToken, result.ID, desc)
		}

		fmt.Printf("   ✅ %s\n   %s\n", result.ID, result.Permalink)
	}
}

func syncExisting(db *gorm.DB, accessToken string) {
	var publicados []storage.Product
	db.Where("meli_id != ?", "").Find(&publicados)
	if len(publicados) == 0 {
		return
	}

	fmt.Printf("\n🔄 Sincronizando %d productos existentes...\n", len(publicados))

	for _, p := range publicados {
		updatePictures(accessToken, p)
		meli.UpdateDescription(accessToken, p.MeliID, buildDescription(p))

		if !p.IsActive || p.Stock == 0 {
			meli.PauseItem(accessToken, p.MeliID)
			fmt.Printf("   ⏸️  Pausado: %s (%s) - sin stock\n", p.MeliID, p.Name)
			continue
		}

		updates := map[string]interface{}{
			"price":              p.MeliPrice,
			"available_quantity": p.Stock,
		}
		if err := meli.UpdateItem(accessToken, p.MeliID, updates); err != nil {
			fmt.Printf("   ❌ Sync %s: %v\n", p.MeliID, err)
		} else {
			fmt.Printf("   ✅ Sync: %s | $%.0f | Stock: %d\n", p.Name, p.MeliPrice, p.Stock)
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

var eanBySKU = map[string]string{
	"664": "631656705737",
	"670": "631656708172",
	"669": "748927063691",
}

func buildAttributes(p storage.Product) []meli.MeliAttribute {
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

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
