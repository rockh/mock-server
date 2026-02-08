# Go Fiber Mock Server

A lightweight mock server for front-end development that:
* Automatically registers routes from an OpenAPI YAML file
* Validates requests against the OpenAPI schema (like Prism CLI)
* Persists JSON data to a file (data.json by default)
* Supports simple CRUD operations

## Requirements

* Go 1.21+
* OpenAPI schema file (YAML)

## Installation
```
git clone https://github.com/rockh/mock-server.git
cd mock-server
go mod tidy
```

## Usage
```
go run . mock <openapi.yaml> [--port 3000] [--data data.json]
```
* <openapi.yaml>: path to your OpenAPI file
* --port: optional, default 3000
* --data: optional, default data.json

## License
MIT