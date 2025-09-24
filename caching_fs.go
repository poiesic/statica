package statica

import (
	"context"
	"errors"
	"io/fs"

	"github.com/maypok86/otter/v2"
)

const defaultCacheSize = 1000
const defaultInitialCapacity = 100

// FSLoader implements otter.Loader
type FSLoader struct {
	files fs.ReadFileFS
}

func (loader *FSLoader) load(filePath string) ([]byte, error) {
	data, err := loader.files.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, otter.ErrNotFound
		}
		return nil, err
	}
	return data, nil
}

func (loader *FSLoader) Load(ctx context.Context, filePath string) ([]byte, error) {
	return loader.load(filePath)
}

func (loader *FSLoader) Reload(ctx context.Context, filePath string, data []byte) ([]byte, error) {
	return loader.load(filePath)
}

var _ otter.Loader[string, []byte] = (*FSLoader)(nil)

// CachingFS uses a pull-through otter.Cache to minimize IO calls
type CachingFS struct {
	fs    *FSLoader
	cache *otter.Cache[string, []byte]
}

var _ fs.ReadFileFS = (*CachingFS)(nil)

// NewCachingFS creates a new CachingFS instance
func NewCachingFS(baseFS fs.ReadFileFS) (*CachingFS, error) {
	if baseFS == nil {
		return nil, ErrNilFS
	}
	loader := &FSLoader{
		files: baseFS,
	}
	var options otter.Options[string, []byte]
	options.MaximumSize = defaultCacheSize
	options.InitialCapacity = defaultInitialCapacity
	cache, err := otter.New(&options)
	if err != nil {
		return nil, err
	}
	return &CachingFS{
		fs:    loader,
		cache: cache,
	}, nil
}

// Open bypasses the cache since the lifetime of the returned fs.File is unknown.
func (cfs *CachingFS) Open(filePath string) (fs.File, error) {
	return cfs.fs.files.Open(filePath)
}

// ReadFile pulls entries into the cache
func (cfs *CachingFS) ReadFile(filePath string) ([]byte, error) {
	data, err := cfs.cache.Get(context.Background(), filePath, cfs.fs)
	if err != nil {
		if errors.Is(err, otter.ErrNotFound) {
			err = fs.ErrNotExist
		}
		return nil, err
	}
	return data, nil
}
