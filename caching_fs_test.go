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
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/maypok86/otter/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test data for caching filesystem tests
var cachingTestFiles = fstest.MapFS{
	"cached.txt":     &fstest.MapFile{Data: []byte("cached content")},
	"test.css":       &fstest.MapFile{Data: []byte("body { color: red; }")},
	"large.js":       &fstest.MapFile{Data: []byte("console.log('large file content');")},
	"nested/file.js": &fstest.MapFile{Data: []byte("nested content")},
}

func TestFSLoader(t *testing.T) {
	t.Run("Load existing file", func(t *testing.T) {
		loader := &FSLoader{files: cachingTestFiles}
		ctx := context.Background()

		data, err := loader.Load(ctx, "cached.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("cached content"), data)
	})

	t.Run("Load non-existent file", func(t *testing.T) {
		loader := &FSLoader{files: cachingTestFiles}
		ctx := context.Background()

		data, err := loader.Load(ctx, "nonexistent.txt")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, otter.ErrNotFound))
		assert.Nil(t, data)
	})

	t.Run("Load nested file", func(t *testing.T) {
		loader := &FSLoader{files: cachingTestFiles}
		ctx := context.Background()

		data, err := loader.Load(ctx, "nested/file.js")
		require.NoError(t, err)
		assert.Equal(t, []byte("nested content"), data)
	})

	t.Run("Reload returns fresh data", func(t *testing.T) {
		loader := &FSLoader{files: cachingTestFiles}
		ctx := context.Background()

		// Initial load
		data1, err := loader.Load(ctx, "cached.txt")
		require.NoError(t, err)

		// Reload should return same data (since underlying fs doesn't change)
		data2, err := loader.Reload(ctx, "cached.txt", data1)
		require.NoError(t, err)
		assert.Equal(t, data1, data2)
	})

	t.Run("Reload non-existent file", func(t *testing.T) {
		loader := &FSLoader{files: cachingTestFiles}
		ctx := context.Background()

		data, err := loader.Reload(ctx, "nonexistent.txt", nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, otter.ErrNotFound))
		assert.Nil(t, data)
	})

	t.Run("load method behaves same as Load", func(t *testing.T) {
		loader := &FSLoader{files: cachingTestFiles}

		data1, err1 := loader.load("cached.txt")
		data2, err2 := loader.Load(context.Background(), "cached.txt")

		assert.Equal(t, err1, err2)
		assert.Equal(t, data1, data2)
	})
}

func TestNewCachingFS(t *testing.T) {
	t.Run("Valid filesystem", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)
		assert.NotNil(t, cfs)
		assert.NotNil(t, cfs.fs)
		assert.NotNil(t, cfs.cache)
	})

	t.Run("Nil filesystem", func(t *testing.T) {
		cfs, err := NewCachingFS(nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrNilFS))
		assert.Nil(t, cfs)
	})
}

func TestCachingFS_ReadFile(t *testing.T) {
	t.Run("Read existing file", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		data, err := cfs.ReadFile("cached.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("cached content"), data)
	})

	t.Run("Read non-existent file", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		data, err := cfs.ReadFile("nonexistent.txt")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, fs.ErrNotExist))
		assert.Nil(t, data)
	})

	t.Run("Read nested file", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		data, err := cfs.ReadFile("nested/file.js")
		require.NoError(t, err)
		assert.Equal(t, []byte("nested content"), data)
	})

	t.Run("Caching behavior - multiple reads", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		// First read
		data1, err := cfs.ReadFile("cached.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("cached content"), data1)

		// Second read should return same data (from cache)
		data2, err := cfs.ReadFile("cached.txt")
		require.NoError(t, err)
		assert.Equal(t, data1, data2)
	})

	t.Run("Multiple different files", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		data1, err := cfs.ReadFile("cached.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("cached content"), data1)

		data2, err := cfs.ReadFile("test.css")
		require.NoError(t, err)
		assert.Equal(t, []byte("body { color: red; }"), data2)

		data3, err := cfs.ReadFile("large.js")
		require.NoError(t, err)
		assert.Equal(t, []byte("console.log('large file content');"), data3)
	})
}

func TestCachingFS_Open(t *testing.T) {
	t.Run("Open bypasses cache", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		file, err := cfs.Open("cached.txt")
		assert.NoError(t, err)
		assert.NotNil(t, file)

		// Verify we can read from the file
		defer file.Close()
		data := make([]byte, 100)
		n, readErr := file.Read(data)
		assert.NoError(t, readErr)
		assert.Greater(t, n, 0)
		assert.Equal(t, "cached content", string(data[:n]))
	})

	t.Run("Open non-existent file", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		file, err := cfs.Open("nonexistent.txt")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, fs.ErrNotExist))
		assert.Nil(t, file)
	})
}

func TestCachingFS_InterfaceCompliance(t *testing.T) {
	t.Run("FSLoader implements otter.Loader", func(t *testing.T) {
		var _ otter.Loader[string, []byte] = (*FSLoader)(nil)
	})

	t.Run("CachingFS implements fs.ReadFileFS", func(t *testing.T) {
		var _ fs.ReadFileFS = (*CachingFS)(nil)
	})
}

func TestCachingFS_Constants(t *testing.T) {
	t.Run("Default constants are reasonable", func(t *testing.T) {
		assert.Equal(t, 1000, defaultCacheSize)
		assert.Equal(t, 100, defaultInitialCapacity)
		assert.True(t, defaultInitialCapacity <= defaultCacheSize)
	})
}

// Test with a filesystem that returns errors other than ErrNotExist
type errorFS struct{}

func (e errorFS) Open(name string) (fs.File, error) {
	return nil, errors.ErrUnsupported
}

func (e errorFS) ReadFile(name string) ([]byte, error) {
	if name == "permission_error" {
		return nil, fs.ErrPermission
	}
	if name == "invalid_error" {
		return nil, fs.ErrInvalid
	}
	return nil, fs.ErrNotExist
}

func TestFSLoader_ErrorHandling(t *testing.T) {
	t.Run("Permission error is passed through", func(t *testing.T) {
		loader := &FSLoader{files: errorFS{}}
		ctx := context.Background()

		data, err := loader.Load(ctx, "permission_error")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, fs.ErrPermission))
		assert.Nil(t, data)
	})

	t.Run("Invalid error is passed through", func(t *testing.T) {
		loader := &FSLoader{files: errorFS{}}
		ctx := context.Background()

		data, err := loader.Load(ctx, "invalid_error")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, fs.ErrInvalid))
		assert.Nil(t, data)
	})

	t.Run("NotExist is converted to otter.ErrNotFound", func(t *testing.T) {
		loader := &FSLoader{files: errorFS{}}
		ctx := context.Background()

		data, err := loader.Load(ctx, "other_file")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, otter.ErrNotFound))
		assert.Nil(t, data)
	})
}

func TestCachingFS_ErrorHandling(t *testing.T) {
	t.Run("otter.ErrNotFound is converted to fs.ErrNotExist", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		data, err := cfs.ReadFile("nonexistent.txt")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, fs.ErrNotExist))
		assert.Nil(t, data)
	})

	t.Run("Other errors from underlying filesystem are passed through", func(t *testing.T) {
		cfs, err := NewCachingFS(errorFS{})
		require.NoError(t, err)

		data, err := cfs.ReadFile("permission_error")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, fs.ErrPermission))
		assert.Nil(t, data)
	})
}

// Additional edge case tests
func TestCachingFS_EdgeCases(t *testing.T) {
	t.Run("Empty file path", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		data, err := cfs.ReadFile("")
		assert.Error(t, err)
		assert.Nil(t, data)
	})

	t.Run("File path with spaces", func(t *testing.T) {
		testFS := fstest.MapFS{
			"file with spaces.txt": &fstest.MapFile{Data: []byte("spaced content")},
		}
		cfs, err := NewCachingFS(testFS)
		require.NoError(t, err)

		data, err := cfs.ReadFile("file with spaces.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("spaced content"), data)
	})

	t.Run("File path with special characters", func(t *testing.T) {
		testFS := fstest.MapFS{
			"file@#$%^&*().txt": &fstest.MapFile{Data: []byte("special chars")},
		}
		cfs, err := NewCachingFS(testFS)
		require.NoError(t, err)

		data, err := cfs.ReadFile("file@#$%^&*().txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("special chars"), data)
	})

	t.Run("Empty file content", func(t *testing.T) {
		testFS := fstest.MapFS{
			"empty.txt": &fstest.MapFile{Data: []byte("")},
		}
		cfs, err := NewCachingFS(testFS)
		require.NoError(t, err)

		data, err := cfs.ReadFile("empty.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte(""), data)
		assert.Len(t, data, 0)
	})

	t.Run("Very large file", func(t *testing.T) {
		largeContent := make([]byte, 1024*1024) // 1MB
		for i := range largeContent {
			largeContent[i] = byte(i % 256)
		}
		testFS := fstest.MapFS{
			"large.bin": &fstest.MapFile{Data: largeContent},
		}
		cfs, err := NewCachingFS(testFS)
		require.NoError(t, err)

		data, err := cfs.ReadFile("large.bin")
		require.NoError(t, err)
		assert.Equal(t, largeContent, data)
		assert.Len(t, data, 1024*1024)
	})

	t.Run("Binary file content", func(t *testing.T) {
		binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD}
		testFS := fstest.MapFS{
			"binary.bin": &fstest.MapFile{Data: binaryData},
		}
		cfs, err := NewCachingFS(testFS)
		require.NoError(t, err)

		data, err := cfs.ReadFile("binary.bin")
		require.NoError(t, err)
		assert.Equal(t, binaryData, data)
	})

	t.Run("Path traversal attempts", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		// These should all fail - the underlying fstest.MapFS should reject them
		testCases := []string{
			"../../../etc/passwd",
			"..\\..\\windows\\system32",
			"/etc/passwd",
			"\\windows\\system32",
		}

		for _, testCase := range testCases {
			data, err := cfs.ReadFile(testCase)
			assert.Error(t, err, "Expected error for path: %s", testCase)
			assert.Nil(t, data, "Expected nil data for path: %s", testCase)
		}
	})
}

// Test concurrent access
func TestCachingFS_Concurrency(t *testing.T) {
	t.Run("Concurrent reads of same file", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		const numGoroutines = 100
		results := make(chan []byte, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				data, err := cfs.ReadFile("cached.txt")
				if err != nil {
					errors <- err
					return
				}
				results <- data
			}()
		}

		// Collect all results
		var allData [][]byte
		var allErrors []error
		for i := 0; i < numGoroutines; i++ {
			select {
			case data := <-results:
				allData = append(allData, data)
			case err := <-errors:
				allErrors = append(allErrors, err)
			}
		}

		// Should have no errors
		assert.Empty(t, allErrors)
		assert.Len(t, allData, numGoroutines)

		// All results should be identical
		expected := []byte("cached content")
		for i, data := range allData {
			assert.Equal(t, expected, data, "Result %d differs", i)
		}
	})

	t.Run("Concurrent reads of different files", func(t *testing.T) {
		cfs, err := NewCachingFS(cachingTestFiles)
		require.NoError(t, err)

		files := []string{"cached.txt", "test.css", "large.js", "nested/file.js"}
		const goroutinesPerFile = 25
		totalGoroutines := len(files) * goroutinesPerFile

		results := make(chan struct {
			file string
			data []byte
		}, totalGoroutines)
		errors := make(chan error, totalGoroutines)

		for _, file := range files {
			for i := 0; i < goroutinesPerFile; i++ {
				go func(fileName string) {
					data, err := cfs.ReadFile(fileName)
					if err != nil {
						errors <- err
						return
					}
					results <- struct {
						file string
						data []byte
					}{fileName, data}
				}(file)
			}
		}

		// Collect all results
		fileResults := make(map[string][][]byte)
		var allErrors []error

		for i := 0; i < totalGoroutines; i++ {
			select {
			case result := <-results:
				fileResults[result.file] = append(fileResults[result.file], result.data)
			case err := <-errors:
				allErrors = append(allErrors, err)
			}
		}

		// Should have no errors
		assert.Empty(t, allErrors)

		// Verify each file got the expected number of results and they're all identical
		expectedContents := map[string][]byte{
			"cached.txt":     []byte("cached content"),
			"test.css":       []byte("body { color: red; }"),
			"large.js":       []byte("console.log('large file content');"),
			"nested/file.js": []byte("nested content"),
		}

		for file, results := range fileResults {
			assert.Len(t, results, goroutinesPerFile, "File %s should have %d results", file, goroutinesPerFile)
			expected := expectedContents[file]
			for i, data := range results {
				assert.Equal(t, expected, data, "File %s result %d differs", file, i)
			}
		}
	})
}

// Test cache behavior under stress
func TestCachingFS_CacheStress(t *testing.T) {
	t.Run("Cache fills up beyond capacity", func(t *testing.T) {
		// Create many files to exceed cache capacity
		manyFiles := make(fstest.MapFS)
		for i := 0; i < defaultCacheSize*2; i++ {
			filename := fmt.Sprintf("file%d.txt", i)
			content := fmt.Sprintf("content of file %d", i)
			manyFiles[filename] = &fstest.MapFile{Data: []byte(content)}
		}

		cfs, err := NewCachingFS(manyFiles)
		require.NoError(t, err)

		// Read all files - should work despite exceeding cache capacity
		for i := 0; i < defaultCacheSize*2; i++ {
			filename := fmt.Sprintf("file%d.txt", i)
			expectedContent := fmt.Sprintf("content of file %d", i)

			data, err := cfs.ReadFile(filename)
			require.NoError(t, err)
			assert.Equal(t, []byte(expectedContent), data)
		}
	})
}

// Test FSLoader edge cases
func TestFSLoader_EdgeCases(t *testing.T) {
	t.Run("Load with nil files field", func(t *testing.T) {
		loader := &FSLoader{files: nil}
		ctx := context.Background()

		// This should panic or return an error
		assert.Panics(t, func() {
			_, _ = loader.Load(ctx, "any.txt")
		})
	})

	t.Run("Context cancellation ignored", func(t *testing.T) {
		loader := &FSLoader{files: cachingTestFiles}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Load should still work since context is ignored
		data, err := loader.Load(ctx, "cached.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("cached content"), data)
	})
}