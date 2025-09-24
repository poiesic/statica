// Copyright 2025 Poiesic Systems
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statica

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type wrappedDirFS struct {
	fs fs.FS
}

func (w *wrappedDirFS) Open(name string) (fs.File, error) {
	return w.fs.Open(name)
}

func (w *wrappedDirFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(w.fs, name)
}

//go:embed benchmark_assets/*.*
var benchmarkAssets embed.FS

func BenchmarkOnDiskFS(b *testing.B) {
	tempDir := setupBenchmarkAssets(b)
	defer os.RemoveAll(tempDir)

	server, err := NewAssetServer("/assets/", &wrappedDirFS{fs: os.DirFS(tempDir)})
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/assets/style.css", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkEmbedFS(b *testing.B) {
	server, err := NewAssetServer("/assets/", benchmarkAssets)
	if err != nil {
		b.Fatal(err)
	}
	server.FSPrefix = "benchmark_assets/"

	req := httptest.NewRequest("GET", "/assets/style.css", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkOnDiskFSWithCaching(b *testing.B) {
	tempDir := setupBenchmarkAssets(b)
	defer os.RemoveAll(tempDir)

	cachingFS, err := NewDefaultCachingFS(&wrappedDirFS{fs: os.DirFS(tempDir)})
	if err != nil {
		b.Fatal(err)
	}

	server, err := NewAssetServer("/assets/", cachingFS)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/assets/style.css", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkEmbedFSWithCaching(b *testing.B) {
	cachingFS, err := NewDefaultCachingFS(benchmarkAssets)
	if err != nil {
		b.Fatal(err)
	}

	server, err := NewAssetServer("/assets/", cachingFS)
	if err != nil {
		b.Fatal(err)
	}
	server.FSPrefix = "benchmark_assets/"

	req := httptest.NewRequest("GET", "/assets/style.css", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkMultipleFileAccess_OnDisk(b *testing.B) {
	tempDir := setupBenchmarkAssets(b)
	defer os.RemoveAll(tempDir)

	server, err := NewAssetServer("/assets/", &wrappedDirFS{fs: os.DirFS(tempDir)})
	if err != nil {
		b.Fatal(err)
	}

	files := []string{
		"/assets/style.css",
		"/assets/app.js",
		"/assets/index.html",
		"/assets/data.json",
		"/assets/logo.png",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, file := range files {
			req := httptest.NewRequest("GET", file, nil)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
		}
	}
}

func BenchmarkMultipleFileAccess_EmbedFS(b *testing.B) {
	server, err := NewAssetServer("/assets/", benchmarkAssets)
	if err != nil {
		b.Fatal(err)
	}
	server.FSPrefix = "benchmark_assets/"

	files := []string{
		"/assets/style.css",
		"/assets/app.js",
		"/assets/index.html",
		"/assets/data.json",
		"/assets/logo.png",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, file := range files {
			req := httptest.NewRequest("GET", file, nil)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
		}
	}
}

func BenchmarkMultipleFileAccess_OnDiskCached(b *testing.B) {
	tempDir := setupBenchmarkAssets(b)
	defer os.RemoveAll(tempDir)

	cachingFS, err := NewDefaultCachingFS(&wrappedDirFS{fs: os.DirFS(tempDir)})
	if err != nil {
		b.Fatal(err)
	}

	server, err := NewAssetServer("/assets/", cachingFS)
	if err != nil {
		b.Fatal(err)
	}

	files := []string{
		"/assets/style.css",
		"/assets/app.js",
		"/assets/index.html",
		"/assets/data.json",
		"/assets/logo.png",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, file := range files {
			req := httptest.NewRequest("GET", file, nil)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
		}
	}
}

func BenchmarkMultipleFileAccess_EmbedFSCached(b *testing.B) {
	cachingFS, err := NewDefaultCachingFS(benchmarkAssets)
	if err != nil {
		b.Fatal(err)
	}

	server, err := NewAssetServer("/assets/", cachingFS)
	if err != nil {
		b.Fatal(err)
	}
	server.FSPrefix = "benchmark_assets/"

	files := []string{
		"/assets/style.css",
		"/assets/app.js",
		"/assets/index.html",
		"/assets/data.json",
		"/assets/logo.png",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, file := range files {
			req := httptest.NewRequest("GET", file, nil)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
		}
	}
}

func BenchmarkRepeatedFileAccess_OnDisk(b *testing.B) {
	tempDir := setupBenchmarkAssets(b)
	defer os.RemoveAll(tempDir)

	server, err := NewAssetServer("/assets/", &wrappedDirFS{fs: os.DirFS(tempDir)})
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/assets/style.css", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
	}
}

func BenchmarkRepeatedFileAccess_Cached(b *testing.B) {
	tempDir := setupBenchmarkAssets(b)
	defer os.RemoveAll(tempDir)

	cachingFS, err := NewDefaultCachingFS(&wrappedDirFS{fs: os.DirFS(tempDir)})
	if err != nil {
		b.Fatal(err)
	}

	server, err := NewAssetServer("/assets/", cachingFS)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/assets/style.css", nil)

	// Warm up the cache
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
	}
}

func BenchmarkLargeFileAccess_OnDisk(b *testing.B) {
	tempDir := setupBenchmarkAssets(b)
	defer os.RemoveAll(tempDir)

	server, err := NewAssetServer("/assets/", &wrappedDirFS{fs: os.DirFS(tempDir)})
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/assets/large.txt", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkLargeFileAccess_Cached(b *testing.B) {
	tempDir := setupBenchmarkAssets(b)
	defer os.RemoveAll(tempDir)

	cachingFS, err := NewDefaultCachingFS(&wrappedDirFS{fs: os.DirFS(tempDir)})
	if err != nil {
		b.Fatal(err)
	}

	server, err := NewAssetServer("/assets/", cachingFS)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/assets/large.txt", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func setupBenchmarkAssets(b *testing.B) string {
	tempDir, err := os.MkdirTemp("", "statica_bench")
	if err != nil {
		b.Fatal(err)
	}

	assets := map[string]string{
		"style.css":  "body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }",
		"app.js":     "document.addEventListener('DOMContentLoaded', function() { console.log('App loaded'); });",
		"index.html": "<!DOCTYPE html><html><head><title>Test</title><link rel='stylesheet' href='style.css'></head><body><h1>Hello World</h1><script src='app.js'></script></body></html>",
		"data.json":  `{"name": "test", "version": "1.0.0", "description": "benchmark test data", "items": [1, 2, 3, 4, 5]}`,
		"logo.png":   "fake-png-data-for-benchmark-testing-purposes-only",
		"large.txt":  generateLargeContent(),
	}

	for filename, content := range assets {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
	}

	return tempDir
}

func generateLargeContent() string {
	content := "This is a large file for benchmarking purposes.\n"
	result := ""
	for i := 0; i < 1000; i++ {
		result += content
	}
	return result
}

func init() {
	benchmarkAssetsDir := "benchmark_assets"
	if err := os.MkdirAll(benchmarkAssetsDir, 0755); err != nil {
		return
	}

	assets := map[string]string{
		"style.css":  "body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }",
		"app.js":     "document.addEventListener('DOMContentLoaded', function() { console.log('App loaded'); });",
		"index.html": "<!DOCTYPE html><html><head><title>Test</title><link rel='stylesheet' href='style.css'></head><body><h1>Hello World</h1><script src='app.js'></script></body></html>",
		"data.json":  `{"name": "test", "version": "1.0.0", "description": "benchmark test data", "items": [1, 2, 3, 4, 5]}`,
		"logo.png":   "fake-png-data-for-benchmark-testing-purposes-only",
	}

	for filename, content := range assets {
		path := filepath.Join(benchmarkAssetsDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.WriteFile(path, []byte(content), 0644)
		}
	}
}
