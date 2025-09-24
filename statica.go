package statica

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
)

// mimeTyper infers mime types from file names
type mimeTyper struct {
	expr     *regexp.Regexp
	mimeType string
}

// StaticaHeaderFunc is used to set headers on a response
type StaticaHeaderFunc func(w http.ResponseWriter, data []byte)

// StaticaErrFunc translates Go errors into HTTP responses
type StaticaErrFunc func(w http.ResponseWriter, r *http.Request, err error)

// AssetServer serves static assets from a fs.ReadFileFS
type AssetServer struct {
	files        fs.ReadFileFS
	typers       []mimeTyper
	route        string
	FSPrefix     string
	ErrFunc      StaticaErrFunc
	HeaderFunc   StaticaHeaderFunc
	BrotliSuffix string
}

// Default mime types
const (
	mimeTypeCSS     = "text/css"
	mimeTypeJS      = "text/javascript"
	mimeTypeJSON    = "application/json"
	mimeTypeHTML    = "text/html"
	mimeTypePNG     = "image/png"
	mimeTypeWOFF2   = "font/woff2"
	mimeTypeWOFF    = "font/woff"
	mimeTypeJPG     = "image/jpeg"
	mimeTypeText    = "text/plain"
	mimeTypeUnknown = "application/octet-stream"
)

var (
	cssRegex   = regexp.MustCompile(`\.css$`)
	jsRegex    = regexp.MustCompile(`\.js$`)
	htmlRegex  = regexp.MustCompile(`\.html$`)
	jsonRegex  = regexp.MustCompile(`\.json$`)
	pngRegex   = regexp.MustCompile(`\.png$`)
	woff2Regex = regexp.MustCompile(`\.woff2$`)
	woffRegex  = regexp.MustCompile(`\.woff$`)
	jpegRegex  = regexp.MustCompile(`\.jpeg$`)
	jpgRegex   = regexp.MustCompile(`\.jpg$`)
	txtRegex   = regexp.MustCompile(`\.txt$`)
)

var ErrEmptyRoute = errors.New("assets route is empty")
var ErrNilFS = errors.New("asset filesystem is nil")
var ErrAbsoluteFSPrefix = errors.New("filesystem prefix is an absolute path")
var ErrBadFSPrefix = errors.New("filesystem prefix does not end with '/'")
var ErrBadBrotliSuffix = errors.New("brotli suffix does not start with '.'")

const brotliEncoding = "br"

// DefaultErrFunc translates errors into 404, 403, or 500 status codes depending on the error
func DefaultErrFunc(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, fs.ErrNotExist) {
		w.WriteHeader(http.StatusNotFound)
	} else if errors.Is(err, fs.ErrPermission) {
		w.WriteHeader(http.StatusForbidden)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Add("Content-Type", "text/plain")
	w.Write([]byte(err.Error()))
}

// DefaultHeaderFunc sets Cache-Control header such clients will cache assets for 7 days
func DefaultHeaderFunc(w http.ResponseWriter, data []byte) {
	const cacheHeader = "private, max-age=604800"
	w.Header().Add("Cache-Control", cacheHeader)
}

func buildDefaultTypers() []mimeTyper {
	// Order is significant as first match wins
	var typers = []mimeTyper{
		{cssRegex, mimeTypeCSS},
		{jsRegex, mimeTypeJS},
		{htmlRegex, mimeTypeHTML},
		{jsonRegex, mimeTypeJSON},
		{pngRegex, mimeTypePNG},
		{woff2Regex, mimeTypeWOFF2},
		{woffRegex, mimeTypeWOFF},
		{jpegRegex, mimeTypeJPG},
		{jpgRegex, mimeTypeJPG},
		{txtRegex, mimeTypeText},
	}
	return typers
}

// NewAssetServer creates a new AssetServer instance
func NewAssetServer(route string, files fs.ReadFileFS) (*AssetServer, error) {
	if route == "" {
		return nil, ErrEmptyRoute
	}
	if files == nil {
		return nil, ErrNilFS
	}
	return &AssetServer{
		route:   route,
		files:   files,
		typers:  buildDefaultTypers(),
		ErrFunc: DefaultErrFunc,
	}, nil
}

// Check verifies the AssetServer instance is properly configured
func (server *AssetServer) Check() error {
	if server.route == "" {
		return ErrEmptyRoute
	}
	if server.files == nil {
		return ErrNilFS
	}
	if server.BrotliSuffix != "" {
		if !strings.HasPrefix(server.BrotliSuffix, ".") {
			return ErrBadBrotliSuffix
		}
	}
	if server.FSPrefix != "" {
		if strings.HasPrefix(server.FSPrefix, "/") {
			return ErrAbsoluteFSPrefix
		}
		if !strings.HasSuffix(server.FSPrefix, "/") {
			return ErrBadFSPrefix
		}
	}
	return nil
}

func (server *AssetServer) inferMimeType(filePath string) string {
	if server.BrotliSuffix != "" && strings.HasSuffix(filePath, server.BrotliSuffix) {
		filePath = strings.TrimSuffix(filePath, server.BrotliSuffix)
	}
	mimeType := mimeTypeUnknown
	for _, typer := range server.typers {
		if typer.expr.MatchString(filePath) {
			mimeType = typer.mimeType
			break
		}
	}
	return mimeType
}

func (server *AssetServer) readFile(filePath string) ([]byte, bool, error) {
	var isBrotli = false
	var data []byte
	var err error

	// Apply FSPrefix if configured
	if server.FSPrefix != "" {
		filePath = fmt.Sprintf("%s%s", server.FSPrefix, filePath)
	}

	brotliRequested := strings.HasSuffix(filePath, server.BrotliSuffix)
	if server.BrotliSuffix != "" && !brotliRequested {
		brotliPath := fmt.Sprintf("%s%s", filePath, server.BrotliSuffix)
		data, err = server.files.ReadFile(brotliPath)
		if err == nil {
			isBrotli = true
		}
	}
	if !isBrotli {
		data, err = server.files.ReadFile(filePath)
		if err == nil && brotliRequested && server.BrotliSuffix != "" {
			isBrotli = true
		}
	}
	return data, isBrotli, err
}

// RegisterMimeType adds a new mime type to a asset server instance. Returns true on success
// and false if a duplicate mime type is detected. Set priority to true to make the mime type
// check happen before the default built-in detectors.
// This method is not safe for concurrent use with other configuration
// methods or with ServeHTTP. Configure the server before serving requests
func (server *AssetServer) RegisterMimeType(expr *regexp.Regexp, mimeType string, priority bool) bool {
	found := false
	for _, typer := range server.typers {
		if typer.mimeType == mimeType {
			found = true
			break
		}
	}
	if found {
		return false
	}
	if priority {
		server.typers = append([]mimeTyper{
			{
				expr:     expr,
				mimeType: mimeType},
		}, server.typers...)
	} else {
		server.typers = append(server.typers, mimeTyper{
			expr:     expr,
			mimeType: mimeType,
		})
	}
	return true
}

// RemoveMimeType removes a typer from the asset server instance. Returns true on success
// and false if the mime type wasn't registered.
func (server *AssetServer) RemoveMimeType(mimeType string) bool {
	found := false
	var target int
	for i, typer := range server.typers {
		if typer.mimeType == mimeType {
			found = true
			target = i
			break
		}
	}
	if found {
		if target == len(server.typers)-1 {
			server.typers = server.typers[:target]
		} else if target == 0 {
			server.typers = server.typers[1:]
		} else {
			server.typers = append(server.typers[:target], server.typers[target+1:]...)
		}
		return true
	}
	return false
}

// IsMimeTypeRegistered checks to see if a specific mime type has been set up for detection
// by the asset server instances
func (server *AssetServer) IsMimeTypeRegistered(mimeType string) bool {
	for _, typer := range server.typers {
		if typer.mimeType == mimeType {
			return true
		}
	}
	return false
}

// ServeHTTP serves requests for configured assets
func (server *AssetServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestedPath := strings.TrimPrefix(r.URL.Path, server.route)
	data, isBrotli, err := server.readFile(requestedPath)
	if err != nil {
		if server.ErrFunc != nil {
			server.ErrFunc(w, r, err)
		}
		return
	}
	if server.HeaderFunc != nil {
		server.HeaderFunc(w, data)
	}
	w.Header().Add("Content-Type", server.inferMimeType(requestedPath))
	if isBrotli {
		w.Header().Add("Content-Encoding", brotliEncoding)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
