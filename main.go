package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"appointment-booking-gin-mysql/internal/api"
	"appointment-booking-gin-mysql/internal/db"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

// serveOpenAPI exposes the OpenAPI YAML and a small Swagger UI at /docs
func serveOpenAPI(r *gin.Engine) {
	// serve the YAML file
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.File("openapi.yaml")
	})

	// simple swagger UI page from CDN pointing to /openapi.yaml
	r.GET("/docs", func(c *gin.Context) {
		html := `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>API Docs — Swagger UI</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/4.15.5/swagger-ui.css" rel="stylesheet">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/4.15.5/swagger-ui-bundle.min.js"></script>
  <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: '/openapi.yaml',
        dom_id: '#swagger-ui',
        presets: [SwaggerUIBundle.presets.apis],
        layout: 'BaseLayout',
        deepLinking: true
      });
      window.ui = ui;
    };
  </script>
</body>
</html>`
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})
}

// loadEnv attempts to load .env from common locations
func loadEnv() {
	// try project root .env
	if err := godotenv.Load(".env"); err == nil {
		log.Println(".env loaded from project root")
		return
	}

	// try parent directory (useful if running from cmd/server)
	if err := godotenv.Load(filepath.Join("..", ".env")); err == nil {
		log.Println(".env loaded from parent directory")
		return
	}

	// fallback: try current working dir without explicit path
	if err := godotenv.Load(); err == nil {
		log.Println(".env loaded from working directory")
		return
	}

	log.Println("⚠ .env not found — relying on system environment variables")
}

func main() {

	loadEnv()
	// Read MYSQL_DSN from env or .env
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN env required. Example: root:pass@tcp(127.0.0.1:3306)/bookingdb?parseTime=true")
	}

	driver := flag.String("driver", "mysql", "database driver")
	flag.Parse()

	// Connect to DB
	sqlDB, err := db.Open(*driver, dsn)
	if err != nil {
		log.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	// Run migrations if file present
	migrationPath := filepath.Join("internal", "models", "migrations.sql")
	if b, err := os.ReadFile(migrationPath); err == nil && len(b) > 0 {
		if _, err := sqlDB.Exec(string(b)); err != nil {
			log.Printf("migration warning: %v (OK if already applied)", err)
		} else {
			log.Println("migrations executed successfully")
		}
	} else if err != nil {
		log.Printf("could not read migration file: %v", err)
	}

	// Initialize Gin server
	r := gin.Default()

	// Serve OpenAPI and Swagger UI
	serveOpenAPI(r)

	// Register API routes
	api.RegisterRoutes(r, sqlDB)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("gin.Run: %v", err)
	}
}
