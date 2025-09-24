package statica

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock file system setup
var testFiles = fstest.MapFS{
	"test.css":                &fstest.MapFile{Data: []byte("body { color: blue; }")},
	"test.js":                 &fstest.MapFile{Data: []byte("console.log('test');")},
	"test.txt":                &fstest.MapFile{Data: []byte("plain text")},
	"test.json":               &fstest.MapFile{Data: []byte(`{"key": "value"}`)},
	"test.png":                &fstest.MapFile{Data: []byte("mock-png-data")},
	"test.woff":               &fstest.MapFile{Data: []byte("mock-woff-data")},
	"test.woff2":              &fstest.MapFile{Data: []byte("mock-woff2-data")},
	"test.jpg":                &fstest.MapFile{Data: []byte("mock-jpg-data")},
	"test.jpeg":               &fstest.MapFile{Data: []byte("mock-jpeg-data")},
	"test.unknown":            &fstest.MapFile{Data: []byte("unknown type data")},
	"test.css.br":             &fstest.MapFile{Data: []byte("compressed-css-data")},
	"forbidden.txt":           &fstest.MapFile{Data: []byte("forbidden"), Mode: 0000},
	"prefix/nested/style.css": &fstest.MapFile{Data: []byte("prefixed css")},
	"prefix/script.js":        &fstest.MapFile{Data: []byte("prefixed js")},
	"only-brotli.js.br":       &fstest.MapFile{Data: []byte("only-brotli-content")},
}

func TestNewAssetServer(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, "/assets/", server.route)
		assert.NotNil(t, server.files)
		assert.NotNil(t, server.typers)
		assert.NotNil(t, server.ErrFunc)
	})

	t.Run("Empty route", func(t *testing.T) {
		server, err := NewAssetServer("", testFiles)
		assert.Nil(t, server)
		assert.Equal(t, ErrEmptyRoute, err)
	})

	t.Run("Nil filesystem", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", nil)
		assert.Nil(t, server)
		assert.Equal(t, ErrNilFS, err)
	})
}

func TestMimeTypeInference(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)
	tests := []struct {
		name     string
		file     string
		expected string
	}{
		{"CSS", "style.css", mimeTypeCSS},
		{"JavaScript", "script.js", mimeTypeJS},
		{"PNG", "image.png", mimeTypePNG},
		{"WOFF2", "font.woff2", mimeTypeWOFF2},
		{"WOFF", "font.woff", mimeTypeWOFF},
		{"JPEG", "image.jpeg", mimeTypeJPG},
		{"JPG", "image.jpg", mimeTypeJPG},
		{"JSON", "data.json", mimeTypeJSON},
		{"Text", "file.txt", mimeTypeText},
		{"Unknown", "file.xyz", mimeTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.inferMimeType(tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegisterMimeType(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)

	t.Run("Register new mime type", func(t *testing.T) {
		success := server.RegisterMimeType(regexp.MustCompile(`\.svg$`), "image/svg+xml", false)
		assert.True(t, success)
		assert.True(t, server.IsMimeTypeRegistered("image/svg+xml"))
	})

	t.Run("Register duplicate mime type", func(t *testing.T) {
		success := server.RegisterMimeType(regexp.MustCompile(`\.svg2$`), "image/svg+xml", false)
		assert.False(t, success)
	})

	t.Run("Register with priority", func(t *testing.T) {
		success := server.RegisterMimeType(regexp.MustCompile(`\.priority$`), "priority/type", true)
		assert.True(t, success)
		assert.Equal(t, "priority/type", server.typers[0].mimeType)
	})
}

func TestRemoveMimeType(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)

	t.Run("Remove existing mime type", func(t *testing.T) {
		success := server.RemoveMimeType(mimeTypeCSS)
		assert.True(t, success)
		assert.False(t, server.IsMimeTypeRegistered(mimeTypeCSS))
	})

	t.Run("Remove non-existent mime type", func(t *testing.T) {
		success := server.RemoveMimeType("non/existent")
		assert.False(t, success)
	})
}

func TestServeHTTP(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedType   string
		expectedBody   string
	}{
		{
			name:           "Serve CSS file",
			path:           "/assets/test.css",
			expectedStatus: http.StatusOK,
			expectedType:   mimeTypeCSS,
			expectedBody:   "body { color: blue; }",
		},
		{
			name:           "Serve JS file",
			path:           "/assets/test.js",
			expectedStatus: http.StatusOK,
			expectedType:   mimeTypeJS,
			expectedBody:   "console.log('test');",
		},
		{
			name:           "File not found",
			path:           "/assets/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
			expectedType:   "text/plain",
			expectedBody:   "open nonexistent.txt: file does not exist",
		},
		{
			name:           "Unknown file type",
			path:           "/assets/test.unknown",
			expectedStatus: http.StatusOK,
			expectedType:   mimeTypeUnknown,
			expectedBody:   "unknown type data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedType, w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestBrotliSupport(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)
	server.BrotliSuffix = ".br"

	t.Run("Normal file with brotli variant", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/test.css", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, mimeTypeCSS, w.Header().Get("Content-Type"))
		assert.Equal(t, "br", w.Header().Get("Content-Encoding"))
		assert.Equal(t, "compressed-css-data", w.Body.String())
	})

	t.Run("Direct request to .br file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/test.css.br", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		// The server should serve the compressed content with correct headers
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, mimeTypeCSS, w.Header().Get("Content-Type"))        // Should use original file's mime type
		assert.Equal(t, brotliEncoding, w.Header().Get("Content-Encoding")) // Mark as brotli compressed
		assert.Equal(t, "compressed-css-data", w.Body.String())             // Send compressed content for client to decompress
	})
}

func TestCustomErrorHandler(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)
	customErr := func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("custom error"))
	}
	server.ErrFunc = customErr

	req := httptest.NewRequest("GET", "/assets/nonexistent.txt", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "custom error", w.Body.String())
}

func TestCustomHeaderHandler(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)
	customHeader := func(w http.ResponseWriter, data []byte) {
		w.Header().Add("X-Custom", "test")
	}
	server.HeaderFunc = customHeader

	req := httptest.NewRequest("GET", "/assets/test.txt", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, "test", w.Header().Get("X-Custom"))
}

func TestDefaultHeaderFunc(t *testing.T) {
	w := httptest.NewRecorder()
	DefaultHeaderFunc(w, nil)
	assert.Equal(t, "private, max-age=604800", w.Header().Get("Cache-Control"))
}

func TestDefaultErrFunc(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "Not Found Error",
			err:            fs.ErrNotExist,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Permission Error",
			err:            fs.ErrPermission,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Other Error",
			err:            errors.New("unknown error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", nil)

			DefaultErrFunc(w, r, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.err.Error(), w.Body.String())
		})
	}
}

func TestCheck(t *testing.T) {
	t.Run("Valid server", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		err = server.Check()
		assert.Nil(t, err)
	})

	t.Run("Empty route", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.route = ""
		err = server.Check()
		assert.Equal(t, ErrEmptyRoute, err)
	})

	t.Run("Nil filesystem", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.files = nil
		err = server.Check()
		assert.Equal(t, ErrNilFS, err)
	})

	t.Run("Bad Brotli suffix - no dot prefix", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.BrotliSuffix = "br"
		err = server.Check()
		assert.Equal(t, ErrBadBrotliSuffix, err)
	})

	t.Run("Good Brotli suffix", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.BrotliSuffix = ".br"
		err = server.Check()
		assert.Nil(t, err)
	})

	t.Run("Absolute FSPrefix", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.FSPrefix = "/absolute/path/"
		err = server.Check()
		assert.Equal(t, ErrAbsoluteFSPrefix, err)
	})

	t.Run("FSPrefix without trailing slash", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.FSPrefix = "relative/path"
		err = server.Check()
		assert.Equal(t, ErrBadFSPrefix, err)
	})

	t.Run("Valid FSPrefix", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.FSPrefix = "relative/path/"
		err = server.Check()
		assert.Nil(t, err)
	})
}

func TestFSPrefix(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)
	server.FSPrefix = "prefix/"

	t.Run("Serve file with FSPrefix", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/script.js", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, mimeTypeJS, w.Header().Get("Content-Type"))
		assert.Equal(t, "prefixed js", w.Body.String())
	})

	t.Run("Serve nested file with FSPrefix", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/nested/style.css", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, mimeTypeCSS, w.Header().Get("Content-Type"))
		assert.Equal(t, "prefixed css", w.Body.String())
	})

	t.Run("File not found with FSPrefix", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/nonexistent.js", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("MIME type inference with FSPrefix", func(t *testing.T) {
		mimeType := server.inferMimeType("script.js")
		assert.Equal(t, mimeTypeJS, mimeType)
	})

	t.Run("MIME type inference with FSPrefix and Brotli", func(t *testing.T) {
		server.BrotliSuffix = ".br"
		mimeType := server.inferMimeType("script.js.br")
		assert.Equal(t, mimeTypeJS, mimeType)
	})
}

func TestBrotliEdgeCases(t *testing.T) {
	t.Run("File with only brotli variant gets served", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.BrotliSuffix = ".br"

		req := httptest.NewRequest("GET", "/assets/only-brotli.js", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		// The readFile method tries brotli first, so this succeeds
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, mimeTypeJS, w.Header().Get("Content-Type"))
		assert.Equal(t, "br", w.Header().Get("Content-Encoding"))
		assert.Equal(t, "only-brotli-content", w.Body.String())
	})

	t.Run("Direct request to brotli file when original doesn't exist", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.BrotliSuffix = ".br"

		req := httptest.NewRequest("GET", "/assets/only-brotli.js.br", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, mimeTypeJS, w.Header().Get("Content-Type"))
		assert.Equal(t, "br", w.Header().Get("Content-Encoding"))
		assert.Equal(t, "only-brotli-content", w.Body.String())
	})

	t.Run("File without brotli variant falls back to original", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		server.BrotliSuffix = ".br"

		req := httptest.NewRequest("GET", "/assets/test.txt", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, mimeTypeText, w.Header().Get("Content-Type"))
		assert.Equal(t, "", w.Header().Get("Content-Encoding"))
		assert.Equal(t, "plain text", w.Body.String())
	})

	t.Run("Empty BrotliSuffix disables brotli", func(t *testing.T) {
		server, err := NewAssetServer("/assets/", testFiles)
		require.Nil(t, err)
		// BrotliSuffix is empty by default, no need to set it

		req := httptest.NewRequest("GET", "/assets/test.css", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "", w.Header().Get("Content-Encoding"))
		assert.Equal(t, "body { color: blue; }", w.Body.String())
	})
}

func TestServeHTTPNilHandlers(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)

	t.Run("Nil ErrFunc", func(t *testing.T) {
		server.ErrFunc = nil
		req := httptest.NewRequest("GET", "/assets/nonexistent.txt", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		// Should return without error, status would be 200 by default
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "", w.Body.String())
	})

	t.Run("Nil HeaderFunc", func(t *testing.T) {
		server.HeaderFunc = nil
		req := httptest.NewRequest("GET", "/assets/test.txt", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "plain text", w.Body.String())
		// Should not have custom headers but still have Content-Type
		assert.Equal(t, mimeTypeText, w.Header().Get("Content-Type"))
	})
}

func TestRemoveMimeTypeEdgeCases(t *testing.T) {
	server, err := NewAssetServer("/assets/", testFiles)
	require.Nil(t, err)

	t.Run("Remove first element", func(t *testing.T) {
		originalFirst := server.typers[0].mimeType
		success := server.RemoveMimeType(originalFirst)
		assert.True(t, success)
		assert.False(t, server.IsMimeTypeRegistered(originalFirst))
		assert.True(t, len(server.typers) > 0) // Should still have other typers
	})

	t.Run("Remove last element", func(t *testing.T) {
		server, _ := NewAssetServer("/assets/", testFiles) // Fresh server
		lastIndex := len(server.typers) - 1
		originalLast := server.typers[lastIndex].mimeType
		success := server.RemoveMimeType(originalLast)
		assert.True(t, success)
		assert.False(t, server.IsMimeTypeRegistered(originalLast))
		assert.True(t, len(server.typers) > 0) // Should still have other typers
	})

	t.Run("Remove middle element", func(t *testing.T) {
		server, _ := NewAssetServer("/assets/", testFiles) // Fresh server
		if len(server.typers) >= 3 {
			middleIndex := len(server.typers) / 2
			originalMiddle := server.typers[middleIndex].mimeType
			originalLength := len(server.typers)
			success := server.RemoveMimeType(originalMiddle)
			assert.True(t, success)
			assert.False(t, server.IsMimeTypeRegistered(originalMiddle))
			assert.Equal(t, originalLength-1, len(server.typers))
		}
	})

	t.Run("Remove from single element list", func(t *testing.T) {
		server, _ := NewAssetServer("/assets/", testFiles) // Fresh server
		// Remove all but one
		for len(server.typers) > 1 {
			server.RemoveMimeType(server.typers[0].mimeType)
		}
		lastType := server.typers[0].mimeType
		success := server.RemoveMimeType(lastType)
		assert.True(t, success)
		assert.False(t, server.IsMimeTypeRegistered(lastType))
		assert.Equal(t, 0, len(server.typers))
	})
}

func TestPermissionErrors(t *testing.T) {
	t.Run("Permission error handling", func(t *testing.T) {
		// Test that DefaultErrFunc properly handles permission errors
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/test", nil)

		DefaultErrFunc(w, r, fs.ErrPermission)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
		assert.Equal(t, fs.ErrPermission.Error(), w.Body.String())
	})
}
