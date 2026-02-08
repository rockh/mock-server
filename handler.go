package main

import (
	"bytes"
	"context"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

func handle(c *fiber.Ctx, method, resource string, store *Store, dataFile string) error {
	// --- OpenAPI Request Validation ---
	routePath := c.Route().Path
	operation := operationForPathMethod(routePath, method)
	if operation != nil {
		reqBody := bytes.NewReader(c.Body())
		reqURL := "http://localhost" + c.OriginalURL()
		req, err := http.NewRequest(c.Method(), reqURL, reqBody)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Request validation failed: "+err.Error())
		}
		c.Request().Header.VisitAll(func(k, v []byte) {
			req.Header.Add(string(k), string(v))
		})

		// Use the OpenAPI router to find the route and validate the request
		if openapiRouter != nil {
			route, pathParams, err := openapiRouter.FindRoute(req)
			if err == nil && route != nil {
				reqValidationInput := &openapi3filter.RequestValidationInput{
					Request:    req,
					Route:      route,
					PathParams: pathParams,
				}
				if err := openapi3filter.ValidateRequest(context.Background(), reqValidationInput); err != nil {
					return fiber.NewError(fiber.StatusBadRequest, "Request validation failed: "+err.Error())
				}
			}
			// If FindRoute fails, skip validation rather than crashing
		}
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	list := store.Data[resource]

	id, _ := strconv.Atoi(c.Params("id"))

	switch method {
	case fiber.MethodGet:
		if id > 0 {
			for _, item := range list {
				if int(item["id"].(float64)) == id {
					return c.JSON(item)
				}
			}
			return fiber.ErrNotFound
		}
		return c.JSON(list)

	case fiber.MethodPost:
		var body map[string]any
		_ = c.BodyParser(&body)
		body["id"] = len(list) + 1
		store.Data[resource] = append(list, body)
		if dataFile == "" {
			store.Save("data.json")
		} else {
			store.Save(dataFile)
		}
		return c.Status(201).JSON(body)

	case fiber.MethodDelete:
		for i, item := range list {
			if int(item["id"].(float64)) == id {
				store.Data[resource] = append(list[:i], list[i+1:]...)
				if dataFile == "" {
					store.Save("data.json")
				} else {
					store.Save(dataFile)
				}
				return c.SendStatus(204)
			}
		}
		return fiber.ErrNotFound
	}

	return fiber.ErrNotImplemented
}

func operationForPathMethod(path, method string) *openapi3.Operation {
	if openapiDoc == nil {
		return nil
	}
	if item := openapiDoc.Paths.Find(path); item != nil {
		switch method {
		case fiber.MethodGet:
			return item.Get
		case fiber.MethodPost:
			return item.Post
		case fiber.MethodPut:
			return item.Put
		case fiber.MethodPatch:
			return item.Patch
		case fiber.MethodDelete:
			return item.Delete
		}
	}
	return nil
}
