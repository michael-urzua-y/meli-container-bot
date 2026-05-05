# meli-container-bot

Bot automatizado para publicar y sincronizar productos de Supletech en MercadoLibre.

## 🚀 Funcionalidad

- Scrapea precios y stock desde Supletech (API VTEX)
- Calcula precios de venta considerando comisiones, envíos y margen de ganancia
- Publica productos nuevos en MercadoLibre
- Sincroniza precios, stock y fotos de productos existentes
- Pausa automáticamente productos sin stock
- Ejecución automática con ticker configurable

## 📦 Estructura del Proyecto

```
meli-container-bot/
├── cmd/
│   ├── bot/       # Flujo completo (scrape → BD → publish → sync)
│   ├── publish/   # Solo publicar/sincronizar (sin scrape)
│   ├── auth/      # Autorización OAuth MercadoLibre
│   └── debug/     # Prueba scraper Supletech
├── internal/
│   ├── meli/      # Cliente API MercadoLibre
│   ├── scraper/   # Scraper API VTEX Supletech
│   ├── storage/   # PostgreSQL + GORM
│   └── logic/     # Cálculo de precios
├── .env           # Configuración (no versionada)
└── .meli_token.json  # Token OAuth (no versionado)
```

## 🔧 Instalación

```bash
# Clonar repositorio
git clone <repo>
cd meli-container-bot

# Instalar dependencias
go mod download

# Compilar (opcional)
go build -o bot cmd/bot/main.go
go build -o publish cmd/publish/main.go
```

## ⚙️ Configuración

Copiar `.env.example` a `.env` (si existe) o crear manualmente:

```bash
cp .env.example .env  # si existe el ejemplo
```

Variables requeridas:

### Base de Datos
```env
DB_HOST=localhost
DB_USER=postgres
DB_NAME=meli_bot
DB_PORT=5432
DB_SSLMODE=disable
```

### MercadoLibre OAuth
```env
MELI_APP_ID=tu_app_id
MELI_SECRET_KEY=tu_secret_key
MELI_REDIRECT_URI=https://tu-dominio.com/callback
```

### Parámetros de Negocio
```env
SHIPPING_COST=3200              # Costo envío Supletech → tu dirección (CLP)
DESIRED_MARGIN=4000             # Ganancia neta deseada por producto (CLP)
SUPLETECH_FREE_SHIP=49990       # Umbral envío gratis Supletech
MELI_COMMISSION=0.15            # Comisión MercadoLibre (15%)
MELI_FIXED_FEE=950              # Cargo fijo MeLi (< $19.990)
MELI_SHIPPING_COST=4500         # Costo envío gratis MeLi
FREE_SHIPPING_THRESHOLD=19990   # Umbral envío gratis obligatorio MeLi
```

### Publicación MeLi
```env
MELI_CATEGORY=MLC435304
MELI_LISTING_TYPE=gold_special
MELI_CURRENCY=CLP
MELI_WARRANTY_TYPE=Garantía del vendedor
MELI_WARRANTY_TIME=30 días
```

### Scraping
```env
PRODUCT_URLS=https://www.supletech.cl/producto1/p?skuId=111,https://www.supletech.cl/producto2/p?skuId=222
```

## 📖 Uso

### 1. Autorizar Aplicación MercadoLibre

```bash
go run cmd/auth/main.go
```

Sigue las instrucciones para obtener el token OAuth. Se guarda en `.meli_token.json`.

### 2. Ejecutar Flujo Completo

```bash
# Primera vez (puebla BD y publica)
go run cmd/bot/main.go

# Modo automático (cada 30 minutos por defecto)
SYNC_INTERVAL=60 go run cmd/bot/main.go  # cada 60 min
```

### 3. Solo Publicar/Sincronizar

Si ya tienes productos en la BD (poblada por scraper externo):

```bash
go run cmd/publish/main.go
```

### 4. Probar Scraper

```bash
go run cmd/debug/main.go
```

Prueba la extracción de datos de Supletech con URL hardcodeada.

## 🔄 Flujo de Trabajo

1. **Scrapeo**: Obtiene datos de productos desde API VTEX de Supletech
2. **Cálculo**: Calcula precio de venta considerando:
   - Costo del producto
   - Envío (si aplica)
   - Comisión MeLi (15%)
   - Cargo fijo MeLi ($950 si precio < $19.990)
   - Costo envío gratis MeLi ($4500 si precio ≥ $19.990)
   - Margen de ganancia deseado ($4000)
3. **Almacenamiento**: Guarda/actualiza productos en PostgreSQL
4. **Publicación**: Publica productos nuevos en MercadoLibre
5. **Sincronización**: Actualiza precio, stock, fotos y descripción
6. **Pausa**: Desactiva productos sin stock

## 🧮 Lógica de Precios

```go
precio = (costo + envio + margen + cargo_fijo) / (1 - comisión)

// Si precio ≥ $19.990 (envío gratis obligatorio MeLi):
precio = (costo + envio + margen + costo_envio_gratis) / (1 - comisión)

// Redondeo: múltiplo de 100 + 90 (ej: $14.990, $15.990)
```

**Notas:**
- Si `costo ≥ $49.990` → Supletech no cobra envío
- `cargo_fijo` aplica solo a productos < $19.990
- `costo_envio_gratis` se suma cuando precio ≥ $19.990

## 🏷️ Atributos de Publicación

Detección automática de tipo de producto:
- **Proteína**: si nombre/categoría contiene "prote", "whey", "iso", "prostar"
- **Creatina**: caso contrario

Atributos fijos:
- `SUPPLEMENT_FORMAT=Polvo`
- `SUPPLEMENT_TYPE=Deportivo`
- `FLAVOR=Sin sabor`
- `SALE_FORMAT=Unidad`
- `UNITS_PER_PACK=1`

**GTIN/EAN:**
- Usa EAN de BD si existe
- Fallback: mapa hardcodeado para SKUs específicos (664, 670, 669)
- Si no hay EAN: `GTIN=does_not_apply` + `EMPTY_GTIN_REASON=17055160`

## 🗄️ Base de Datos

**Motor:** PostgreSQL (GORM ORM)

**Tabla `products`:**
- `sku` (índice único)
- `meli_id` (ID de publicación MeLi)
- `name`, `brand`, `category`
- `cost_price`, `list_price`, `meli_price`
- `stock`, `is_active`
- `image_url`, `nutrition_info_url`, `extra_images`
- `description`, `delivery_time`
- `ean` (código de barras)
- `shipping_cost`, `tax_rate` (19% default)
- `created_at`, `updated_at`, `deleted_at`

## 🔐 Seguridad

- **Nunca** committear `.env` ni `.meli_token.json`
- Usar variables de entorno para credenciales
- Token OAuth se renueva automáticamente
- Timeouts en requests HTTP (15-30s)

## 🐛 Debugging

```bash
# Ver logs detallados
go run cmd/bot/main.go 2>&1 | tee bot.log

# Probar scraper individual
go run cmd/debug/main.go

# Verificar token
cat .meli_token.json | jq .

# Consultar BD
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT * FROM products;"
```

## 📝 Notas Técnicas

- **Go 1.25.3**
- API MeLi: `https://api.mercadolibre.com`
- API Supletech (VTEX): `https://www.supletech.cl/api/catalog_system/pub/products/search/{slug}/p`
- Stock capado a 100 unidades si VTEX reporta >1000 (stock infinito)
- Imágenes: [0]=principal, [1]=info nutricional, [2+]=extras

## 🚀 Despliegue

Para ejecución continua, usar `screen`, `tmux` o systemd:

```bash
# Ejemplo con screen
screen -S meli-bot
SYNC_INTERVAL=30 go run cmd/bot/main.go
# Ctrl+A, D para detachar
```

O compilar y ejecutar binario:

```bash
go build -o bot cmd/bot/main.go
./bot
```

## 📄 Licencia

Propietario - Todos los derechos reservados.
