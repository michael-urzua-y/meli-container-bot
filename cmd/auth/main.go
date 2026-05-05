package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/michael-urzua-y/meli-container-bot/internal/meli"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error cargando .env:", err)
	}

	authURL := meli.GetAuthURL()

	fmt.Println("Autorización MercadoLibre")
	fmt.Println("============================")
	fmt.Println()
	fmt.Println("1. Abre esta URL en tu navegador:")
	fmt.Println()
	fmt.Println("   ", authURL)
	fmt.Println()
	fmt.Println("2. Autoriza la aplicación")
	fmt.Println("3. Te va a redirigir a Google. Copia la URL completa de la barra del navegador")
	fmt.Println("   (va a tener algo como ?code=TG-xxxxx)")
	fmt.Println()
	fmt.Print("4. Pega la URL aquí: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	// Extraer el code de la URL o del input directo
	code := extractCode(input)
	if code == "" {
		log.Fatal("No se encontró el código de autorización en la URL")
	}

	fmt.Printf("\nIntercambiando código: %s...\n", code[:10])

	token, err := meli.ExchangeCode(code)
	if err != nil {
		log.Fatal("Error:", err)
	}

	if err := meli.SaveToken(token); err != nil {
		log.Fatal("Error guardando token:", err)
	}

	fmt.Println("Token obtenido y guardado")
	fmt.Printf("   User ID: %d\n", token.UserID)
	fmt.Printf("   Expira en: %d segundos\n", token.ExpiresIn)
}

func extractCode(input string) string {
	// Si pegó la URL completa
	if strings.Contains(input, "code=") {
		parts := strings.Split(input, "code=")
		if len(parts) > 1 {
			code := parts[1]
			// Quitar parámetros adicionales después del code
			if idx := strings.Index(code, "&"); idx != -1 {
				code = code[:idx]
			}
			return code
		}
	}
	// Si pegó solo el código
	if strings.HasPrefix(input, "TG-") {
		return input
	}
	return ""
}
