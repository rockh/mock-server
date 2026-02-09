package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/getkin/kin-openapi/openapi3"
)

// validationError is a helper that emits Prism-style logs and returns the error response.
func validationError(c *fiber.Ctx, logger *Logger, statusCode int, errMsg string) error {
	logger.Warning(ComponentValidator, "Request did not pass the validation rules")
	logger.Error(ComponentValidator, errMsg)
	logger.RespondWith(statusCode)
	logger.Violation(errMsg)
	return c.Status(statusCode).JSON(fiber.Map{
		"error":   http.StatusText(statusCode),
		"message": errMsg,
	})
}

// bodyValidationError logs every violation on its own line, then responds.
func bodyValidationError(c *fiber.Ctx, logger *Logger, statusCode int, violations []string) error {
	logger.Warning(ComponentValidator, "Request did not pass the validation rules")
	for _, v := range violations {
		logger.Error(ComponentValidator, v)
	}
	logger.RespondWith(statusCode)
	for _, v := range violations {
		logger.Error(ComponentValidator, "Violation: "+v)
	}
	return c.Status(statusCode).JSON(fiber.Map{
		"error":   http.StatusText(statusCode),
		"message": strings.Join(violations, "; "),
	})
}

func handle(c *fiber.Ctx, method, resource string, store *Store, dataFile string) error {
	logger := NewLogger()

	// ── Log request received ───────────────────────────────────────────
	logger.RequestReceived(method, c.Path())

	if accept := c.Get("Accept"); accept != "" {
		logger.Info(ComponentNegotiator, fmt.Sprintf("Request contains an accept header: %s", accept))
	}

	// ── Resolve OpenAPI operation ──────────────────────────────────────
	routePath := c.Route().Path
	operation := operationForPathMethod(routePath, method)

	// ── STEP 1: Security validation ────────────────────────────────────
	// Check per-operation security, then fall back to global security.
	secReqs := resolveSecurityRequirements(operation)
	if len(secReqs) > 0 {
		if !isAuthenticated(c, secReqs) {
			return validationError(c, logger, 401, "Invalid security scheme used")
		}
		logger.Success(ComponentValidator, "Security check passed")
	}

	// ── STEP 2: Content-Type negotiation ───────────────────────────────
	if operation != nil && needsRequestBody(method) {
		if operation.RequestBody != nil && operation.RequestBody.Value != nil {
			rb := operation.RequestBody.Value

			// 2a. Body required but missing
			if rb.Required && len(c.Body()) == 0 {
				return validationError(c, logger, 400, "Body parameter is required")
			}

			// 2b. Content-Type must be acceptable
			if len(c.Body()) > 0 && rb.Content != nil {
				ct := c.Get("Content-Type")
				if ct == "" {
					return validationError(c, logger, 415, "Content-Type header is required")
				}
				// Normalise: take everything before ';' for comparison
				baseCT := strings.Split(ct, ";")[0]
				baseCT = strings.TrimSpace(baseCT)
				if _, ok := rb.Content[baseCT]; !ok {
					allowed := make([]string, 0, len(rb.Content))
					for k := range rb.Content {
						allowed = append(allowed, k)
					}
					return validationError(c, logger, 415,
						fmt.Sprintf("Unsupported media type: %s. Allowed: %s", baseCT, strings.Join(allowed, ", ")))
				}

				// 2c. Validate body against schema (required fields, types, etc.)
				mediaType := rb.Content[baseCT]
				if mediaType.Schema != nil && mediaType.Schema.Value != nil {
					if violations := validateBody(c.Body(), mediaType.Schema.Value); len(violations) > 0 {
						return bodyValidationError(c, logger, 400, violations)
					}
				}
			}
		}
	}

	// ── STEP 3: Required query / path parameters ───────────────────────
	if operation != nil {
		for _, paramRef := range operation.Parameters {
			if paramRef.Value == nil {
				continue
			}
			p := paramRef.Value
			if !p.Required {
				continue
			}
			var val string
			switch p.In {
			case "query":
				val = c.Query(p.Name)
			case "path":
				val = c.Params(p.Name)
			case "header":
				val = c.Get(p.Name)
			}
			if val == "" {
				return validationError(c, logger, 400,
					fmt.Sprintf("Required %s parameter \"%s\" is missing", p.In, p.Name))
			}
		}
	}

	logger.Success(ComponentValidator, "Request passed all validation rules")

	// ── STEP 4: Mock response ──────────────────────────────────────────
	store.mu.Lock()
	defer store.mu.Unlock()

	if store.Data[resource] == nil {
		store.Data[resource] = []map[string]any{}
	}

	list := store.Data[resource]
	id, _ := strconv.Atoi(c.Params("id"))

	switch method {
	case fiber.MethodGet:
		if id > 0 {
			for _, item := range list {
				if int(item["id"].(float64)) == id {
					logger.RespondWith(200)
					return c.JSON(item)
				}
			}
			logger.RespondWith(404)
			return fiber.ErrNotFound
		}
		logger.Success(ComponentNegotiator, fmt.Sprintf("Found %d items. Responding with collection", len(list)))
		logger.RespondWith(200)
		return c.JSON(list)

	case fiber.MethodPost:
		body := make(map[string]any)
		_ = c.BodyParser(&body)
		body["id"] = len(list) + 1
		store.Data[resource] = append(list, body)
		saveStore(store, dataFile)
		logger.RespondWith(201)
		return c.Status(201).JSON(body)

	case fiber.MethodPut, fiber.MethodPatch:
		for i, item := range list {
			if int(item["id"].(float64)) == id {
				body := make(map[string]any)
				_ = c.BodyParser(&body)
				for k, v := range body {
					item[k] = v
				}
				store.Data[resource][i] = item
				saveStore(store, dataFile)
				logger.RespondWith(200)
				return c.JSON(item)
			}
		}
		logger.RespondWith(404)
		return fiber.ErrNotFound

	case fiber.MethodDelete:
		for i, item := range list {
			if int(item["id"].(float64)) == id {
				store.Data[resource] = append(list[:i], list[i+1:]...)
				saveStore(store, dataFile)
				logger.RespondWith(204)
				return c.SendStatus(204)
			}
		}
		logger.RespondWith(404)
		return fiber.ErrNotFound
	}

	return fiber.ErrNotImplemented
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// resolveSecurityRequirements returns the effective security requirements for
// an operation.  Per-operation security wins; if absent we fall back to the
// top-level (global) security definition.
func resolveSecurityRequirements(op *openapi3.Operation) openapi3.SecurityRequirements {
	if op != nil && op.Security != nil {
		return *op.Security
	}
	if openapiDoc != nil && openapiDoc.Security != nil {
		return openapiDoc.Security
	}
	return nil
}

// isAuthenticated checks that the request satisfies at least one of the
// security requirements.  It supports http/bearer AND apiKey schemes.
func isAuthenticated(c *fiber.Ctx, reqs openapi3.SecurityRequirements) bool {
	if openapiDoc == nil || openapiDoc.Components == nil || openapiDoc.Components.SecuritySchemes == nil {
		return false
	}

	for _, req := range reqs {
		// An empty requirement object {} means "no auth needed".
		if len(req) == 0 {
			return true
		}

		allSatisfied := true
		for schemeName := range req {
			schemeRef, ok := openapiDoc.Components.SecuritySchemes[schemeName]
			if !ok || schemeRef.Value == nil {
				allSatisfied = false
				break
			}
			scheme := schemeRef.Value

			switch scheme.Type {
			case "http":
				auth := c.Get("Authorization")
				if auth == "" {
					allSatisfied = false
					break
				}
				// For "bearer" scheme the header must start with "Bearer ".
				if strings.EqualFold(scheme.Scheme, "bearer") {
					if !strings.HasPrefix(auth, "Bearer ") && !strings.HasPrefix(auth, "bearer ") {
						allSatisfied = false
					}
				}
			case "apiKey":
				switch scheme.In {
				case "header":
					if c.Get(scheme.Name) == "" {
						allSatisfied = false
					}
				case "query":
					if c.Query(scheme.Name) == "" {
						allSatisfied = false
					}
				case "cookie":
					if c.Cookies(scheme.Name) == "" {
						allSatisfied = false
					}
				default:
					allSatisfied = false
				}
			default:
				// oauth2, openIdConnect — accept if Authorization header present
				if c.Get("Authorization") == "" {
					allSatisfied = false
				}
			}

			if !allSatisfied {
				break
			}
		}

		if allSatisfied {
			return true
		}
	}
	return false
}

// validateBody checks the JSON body against the schema's required fields and
// basic type constraints.  It handles allOf / oneOf / anyOf compositions by
// flattening required fields and properties from all sub-schemas.
// Returns a slice of all validation error messages (empty = valid).
func validateBody(raw []byte, schema *openapi3.Schema) []string {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return []string{fmt.Sprintf("Invalid JSON body: %s", err.Error())}
	}

	// Collect all required fields and property schemas by walking the schema
	// tree (allOf, oneOf, anyOf and the schema itself).
	required, props := collectSchemaConstraints(schema)

	var violations []string

	// Check required fields — collect ALL missing, don't stop at first
	for _, field := range required {
		if _, ok := body[field]; !ok {
			violations = append(violations,
				fmt.Sprintf("request.body Request body must have required property '%s'", field))
		}
	}

	// Check property types for supplied values
	for name, prop := range props {
		val, exists := body[name]
		if !exists {
			continue
		}
		if prop == nil {
			continue
		}
		if err := checkType(name, val, prop); err != nil {
			violations = append(violations, "request.body "+err.Error())
		}
	}

	return violations
}

// collectSchemaConstraints walks a schema (including allOf, oneOf, anyOf) and
// returns the union of all required field names and a merged property map.
func collectSchemaConstraints(schema *openapi3.Schema) ([]string, map[string]*openapi3.Schema) {
	required := make([]string, 0)
	props := make(map[string]*openapi3.Schema)

	if schema == nil {
		return required, props
	}

	// Collect from the schema itself
	required = append(required, schema.Required...)
	for name, ref := range schema.Properties {
		if ref != nil && ref.Value != nil {
			props[name] = ref.Value
		}
	}

	// Walk allOf — merge everything (intersection semantics, all must match)
	for _, sub := range schema.AllOf {
		if sub.Value == nil {
			continue
		}
		r, p := collectSchemaConstraints(sub.Value)
		required = append(required, r...)
		for k, v := range p {
			props[k] = v
		}
	}

	// Walk oneOf / anyOf — merge properties so we can at least validate
	// fields that the caller supplied.  Required fields from branches are NOT
	// promoted because only one branch needs to match.
	for _, sub := range schema.OneOf {
		if sub.Value == nil {
			continue
		}
		_, p := collectSchemaConstraints(sub.Value)
		for k, v := range p {
			if _, exists := props[k]; !exists {
				props[k] = v
			}
		}
	}
	for _, sub := range schema.AnyOf {
		if sub.Value == nil {
			continue
		}
		_, p := collectSchemaConstraints(sub.Value)
		for k, v := range p {
			if _, exists := props[k]; !exists {
				props[k] = v
			}
		}
	}

	return required, props
}

// checkType validates a single value against an OpenAPI property schema.
func checkType(name string, val any, prop *openapi3.Schema) error {
	if val == nil {
		if !prop.Nullable {
			return fmt.Errorf("Property \"%s\" must not be null", name)
		}
		return nil
	}

	switch prop.Type {
	case "string":
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("Property \"%s\" must be a string", name)
		}
		if prop.MinLength > 0 && uint64(len(s)) < prop.MinLength {
			return fmt.Errorf("Property \"%s\" must be at least %d characters", name, prop.MinLength)
		}
		if prop.MaxLength != nil && uint64(len(s)) > *prop.MaxLength {
			return fmt.Errorf("Property \"%s\" must be at most %d characters", name, *prop.MaxLength)
		}
		if len(prop.Enum) > 0 {
			found := false
			for _, e := range prop.Enum {
				if fmt.Sprintf("%v", e) == s {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Property \"%s\" must be one of: %v", name, prop.Enum)
			}
		}
	case "integer", "number":
		if _, ok := val.(float64); !ok {
			return fmt.Errorf("Property \"%s\" must be a number", name)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("Property \"%s\" must be a boolean", name)
		}
	case "array":
		arr, ok := val.([]any)
		if !ok {
			return fmt.Errorf("Property \"%s\" must be an array", name)
		}
		if prop.MinItems > 0 && uint64(len(arr)) < prop.MinItems {
			return fmt.Errorf("Property \"%s\" must have at least %d items", name, prop.MinItems)
		}
	}
	return nil
}

// needsRequestBody returns true for methods that can carry a body.
func needsRequestBody(method string) bool {
	switch method {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch:
		return true
	}
	return false
}

// operationForPathMethod returns the OpenAPI Operation for a given path+method.
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

// saveStore persists the store to disk.
func saveStore(store *Store, dataFile string) {
	if dataFile == "" {
		store.Save("data.json")
	} else {
		store.Save(dataFile)
	}
}
