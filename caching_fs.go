package statica

import (
	"context"
	"errors"
	"io/fs"

	"github.com/maypok86/otter/v2"
)

const DefaultMaxEntries = 1000
const DefaultInitialCapacity = 100

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

type CachingFSOption struct {
	MaxEntryCount   int
	InitialCapacity int
}

// CachingFS uses a pull-through otter.Cache to minimize IO calls
type CachingFS struct {
	fs    *FSLoader
	cache *otter.Cache[string, []byte]
}

var _ fs.ReadFileFS = (*CachingFS)(nil)

// NewDefaultCachingFS creates a new CachingFS instance with max cache size
// and initial capacity set to `DefaultMaxEntries` and `DefaultInitialCapacity`
// Use NewCachingFS if different values are desired.
func NewDefaultCachingFS(baseFS fs.ReadFileFS) (*CachingFS, error) {
	return NewCachingFS(baseFS, &CachingFSOption{
		MaxEntryCount: DefaultMaxEntries,
		InitialCapacity: DefaultInitialCapacity,
	})
}

// NewCachingFS creates a new CachingFS instance
func NewCachingFS(baseFS fs.ReadFileFS, option *CachingFSOption) (*CachingFS, error) {
	if baseFS == nil {
		return nil, ErrNilFS
	}
	loader := &FSLoader{
		files: baseFS,
	}
	var options otter.Options[string, []byte]
	options.MaximumSize = DefaultMaxEntries
	options.InitialCapacity = DefaultInitialCapacity
	if option != nil {
		if option.MaxEntryCount > 0 {
			options.MaximumSize = option.MaxEntryCount
		}
		if option.InitialCapacity > 0 {
			options.InitialCapacity = option.InitialCapacity
		}
	}
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
