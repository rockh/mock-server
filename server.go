package main

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	gorillamux "github.com/getkin/kin-openapi/routers/gorillamux"
)

var openapiDoc *openapi3.T
var openapiRouter routers.Router

func startServer(openapiPath, dataFile string, port int) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(openapiPath)
	if err != nil {
		log.Fatalf("failed to load openapi: %v", err)
	}

	if err := doc.Validate(loader.Context); err != nil {
		log.Fatalf("invalid openapi schema: %v", err)
	}

	openapiDoc = doc

	r, err := gorillamux.NewRouter(doc)
	if err != nil {
		log.Fatalf("failed to create openapi router: %v", err)
	}
	openapiRouter = r

	store := NewStore(dataFile)
	app := fiber.New()

	RegisterRoutes(app, doc, store, dataFile)

	log.Printf("🚀 Mock server running at http://localhost:%d", port)
	log.Printf("📄 OpenAPI: %s", openapiPath)

	log.Fatal(app.Listen(":" + strconv.Itoa(port)))
}
