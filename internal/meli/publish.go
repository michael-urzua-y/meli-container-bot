package meli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseAPI     = "https://api.mercadolibre.com"
	contentType = "application/json"
	itemsPath   = "/items/"
)

// MeliItem representa el payload para crear un item en MeLi (modelo User Products)
type MeliItem struct {
	FamilyName        string          `json:"family_name"`
	CategoryID        string          `json:"category_id"`
	Price             float64         `json:"price"`
	CurrencyID        string          `json:"currency_id"`
	AvailableQuantity int             `json:"available_quantity"`
	BuyingMode        string          `json:"buying_mode"`
	Condition         string          `json:"condition"`
	ListingTypeID     string          `json:"listing_type_id"`
	SaleTerms         []MeliAttribute `json:"sale_terms"`
	Pictures          []MeliPicture   `json:"pictures"`
	Attributes        []MeliAttribute `json:"attributes"`
}

// MeliAttribute representa un atributo genérico
type MeliAttribute struct {
	ID        string `json:"id"`
	ValueID   string `json:"value_id,omitempty"`
	ValueName string `json:"value_name,omitempty"`
}

// MeliPicture representa una imagen
type MeliPicture struct {
	Source string `json:"source"`
}

// MeliItemResponse respuesta al crear un item
type MeliItemResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Permalink string `json:"permalink"`
	Status    string `json:"status"`
}

// newRequest crea un request autenticado
func newRequest(method, path string, body []byte, accessToken string) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, baseAPI+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", contentType)
	return req, nil
}

// doRequest ejecuta un request y retorna el body
func doRequest(req *http.Request, timeout time.Duration) ([]byte, int, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// PublishItem crea una nueva publicación en MercadoLibre
func PublishItem(accessToken string, item MeliItem) (*MeliItemResponse, error) {
	body, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	req, err := newRequest("POST", "/items", body, accessToken)
	if err != nil {
		return nil, err
	}
	respBody, status, err := doRequest(req, 30*time.Second)
	if err != nil {
		return nil, err
	}
	if status != 201 {
		return nil, fmt.Errorf("error publicando (%d): %s", status, string(respBody))
	}
	var result MeliItemResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateItem actualiza precio, stock o estado de un item existente
func UpdateItem(accessToken, itemID string, updates map[string]interface{}) error {
	body, err := json.Marshal(updates)
	if err != nil {
		return err
	}
	req, err := newRequest("PUT", itemsPath+itemID, body, accessToken)
	if err != nil {
		return err
	}
	respBody, status, err := doRequest(req, 15*time.Second)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("error actualizando %s (%d): %s", itemID, status, string(respBody))
	}
	return nil
}

// PauseItem pausa una publicación
func PauseItem(accessToken, itemID string) error {
	return UpdateItem(accessToken, itemID, map[string]interface{}{"status": "paused"})
}

// ActivateItem activa una publicación pausada
func ActivateItem(accessToken, itemID string) error {
	return UpdateItem(accessToken, itemID, map[string]interface{}{"status": "active"})
}

// AddDescription agrega descripción a un item
func AddDescription(accessToken, itemID, plainText string) error {
	body, _ := json.Marshal(map[string]string{"plain_text": plainText})
	req, err := newRequest("POST", itemsPath+itemID+"/description", body, accessToken)
	if err != nil {
		return err
	}
	respBody, status, err := doRequest(req, 15*time.Second)
	if err != nil {
		return err
	}
	if status != 201 && status != 200 {
		return fmt.Errorf("error descripción (%d): %s", status, string(respBody))
	}
	return nil
}

// UpdateDescription actualiza la descripción de un item existente
func UpdateDescription(accessToken, itemID, plainText string) error {
	body, _ := json.Marshal(map[string]string{"plain_text": plainText})
	req, err := newRequest("PUT", itemsPath+itemID+"/description", body, accessToken)
	if err != nil {
		return err
	}
	respBody, status, err := doRequest(req, 15*time.Second)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("error descripción (%d): %s", status, string(respBody))
	}
	return nil
}
