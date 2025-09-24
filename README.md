# Statica

Tired of wrestling with `embed.FS`, `http.FileServer`, and `http.StripPrefix` just to serve static assets? Statica simplifies static file serving with a clean, feature-rich API that handles MIME types, compression, and routing out of the box.

A Go package for serving static assets over HTTP with built-in MIME type detection, Brotli compression support, and customizable error handling.

## Features

- Serve static files from any `fs.ReadFileFS` filesystem
- Automatic MIME type detection for common file types
- Optional Brotli compression support
- Customizable error handling and response headers
- Support for filesystem path prefixes

## Installation

```bash
go get github.com/poiesic/statica@latest
```

## Basic Usage

```go
package main

import (
    "embed"
    "log"
    "net/http"

    "github.com/poiesic/statica"
)

//go:embed assets/*
var assets embed.FS

func main() {
    // Create asset server
    server, err := statica.NewAssetServer("/static/", assets)
    if err != nil {
        log.Fatal(err)
    }

    // Register with HTTP mux
    http.Handle("/static/", server)

    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Configuration

### Filesystem Prefix

Use `FSPrefix` to serve files from a subdirectory within your filesystem:

```go
server, _ := statica.NewAssetServer("/static/", assets)
server.FSPrefix = "public/"  // Serve files from the "public/" directory
```

### Brotli Compression

Enable Brotli compression by setting a suffix for compressed files:

```go
server, _ := statica.NewAssetServer("/static/", assets)
server.BrotliSuffix = ".br"  // Look for files with .br extension
```

When `BrotliSuffix` is set:
- For `/static/app.js`, the server first checks for `/static/app.js.br` and serves it with Brotli encoding if found
- If the compressed version doesn't exist, it falls back to the original file
- Files explicitly requested with the suffix (e.g., `/static/app.js.br`) are served with Brotli encoding

When `BrotliSuffix` is empty (default), no Brotli compression is attempted.

### Custom Error Handling

You can customize error responses by providing your own implementation of [`StaticaErrFunc`](statica.go:36):

```go
func customErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
    w.WriteHeader(http.StatusNotFound)
    w.Write([]byte("Asset not found"))
}

server, _ := statica.NewAssetServer("/static/", assets)
server.ErrFunc = customErrorHandler
```

### Custom Headers

By default, Statica sets a 7-day cache header (`Cache-Control: private, max-age=604800`). You can customize header behavior by providing your own implementation of [`StaticaHeaderFunc`](statica.go:33):

```go
func customHeaders(w http.ResponseWriter, data []byte) {
    w.Header().Set("Cache-Control", "public, max-age=31536000")  // 1 year
    w.Header().Set("X-Custom-Header", "value")
}

server, _ := statica.NewAssetServer("/static/", assets)
server.HeaderFunc = customHeaders
```

To disable the default cache header, set `HeaderFunc` to `nil`.

### Custom MIME Types

```go
import "regexp"

server, _ := statica.NewAssetServer("/static/", assets)

// Add support for .webp files
webpRegex := regexp.MustCompile(`\.webp$`)
server.RegisterMimeType(webpRegex, "image/webp", false)

// Priority = true makes it check before built-in types
svgRegex := regexp.MustCompile(`\.svg$`)
server.RegisterMimeType(svgRegex, "image/svg+xml", true)
```

## Examples

### Complete Example with All Features

```go
package main

import (
    "embed"
    "log"
    "net/http"
    "regexp"

    "github.com/your-org/statica"
)

//go:embed dist/*
var distFiles embed.FS

func main() {
    server, err := statica.NewAssetServer("/assets/", distFiles)
    if err != nil {
        log.Fatal(err)
    }

    // Configure server
    server.FSPrefix = "dist/"
    server.BrotliSuffix = ".br"
    server.HeaderFunc = func(w http.ResponseWriter, data []byte) {
        w.Header().Set("Cache-Control", "public, max-age=31536000")
        w.Header().Set("X-Served-By", "Statica")
    }

    // Add custom MIME type for .wasm files
    wasmRegex := regexp.MustCompile(`\.wasm$`)
    server.RegisterMimeType(wasmRegex, "application/wasm", false)

    // Validate configuration
    if err := server.Check(); err != nil {
        log.Fatal("Configuration error:", err)
    }

    http.Handle("/assets/", server)

    log.Println("Serving assets at http://localhost:8080/assets/")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Using with Gorilla Mux

```go
import (
    "github.com/gorilla/mux"
    "github.com/your-org/statica"
)

func setupRoutes() *mux.Router {
    r := mux.NewRouter()

    server, _ := statica.NewAssetServer("/static/", assets)
    r.PathPrefix("/static/").Handler(server)

    return r
}
```

## Built-in MIME Types

Statica includes built-in support for:
- CSS (`.css`) → `text/css`
- JavaScript (`.js`) → `text/javascript`
- HTML (`.html`) → `text/html`
- JSON (`.json`) → `application/json`
- PNG (`.png`) → `image/png`
- JPEG (`.jpg`, `.jpeg`) → `image/jpeg`
- WOFF/WOFF2 fonts → `font/woff`, `font/woff2`
- Text files (`.txt`) → `text/plain`

## License

Licensed under the Apache License, Version 2.0.