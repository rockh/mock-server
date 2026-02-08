package main

import (
	"log"
	"sort"
	"strings"
	"github.com/gofiber/fiber/v2"
	"github.com/getkin/kin-openapi/openapi3"
)

func RegisterRoutes(app *fiber.App, doc *openapi3.T, store *Store, dataFile string) {
	endpointsMap := map[string]struct{}{}

	for path, item := range doc.Paths {
		p := path
		resource := strings.Split(strings.Trim(p, "/"), "/")[0]

		if store.Data[resource] == nil {
			store.Data[resource] = []map[string]any{}
		}

		register := func(method string) {
			app.Add(method, p, func(c *fiber.Ctx) error {
				return handle(c, method, resource, store, dataFile)
			})
			endpointsMap[strings.ToUpper(method)+" "+p] = struct{}{}
		}

		if item.Get != nil {
			register(fiber.MethodGet)
		}
		if item.Post != nil {
			register(fiber.MethodPost)
		}
		if item.Put != nil {
			register(fiber.MethodPut)
		}
		if item.Patch != nil {
			register(fiber.MethodPatch)
		}
		if item.Delete != nil {
			register(fiber.MethodDelete)
		}

	}

	if len(endpointsMap) > 0 {
		var endpoints []string
		for e := range endpointsMap {
			endpoints = append(endpoints, e)
		}
		sort.Strings(endpoints)
		log.Println("Available endpoints:")
		for _, e := range endpoints {
			log.Printf("  %s", e)
		}
	}
}
