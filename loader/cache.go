package loader

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lestrrat/go-xslate/compiler"
	"github.com/lestrrat/go-xslate/parser"
	"github.com/lestrrat/go-xslate/vm"
)

// ErrCacheMiss is returned when the bytecode could not be found in the cache
var ErrCacheMiss = errors.New("cache miss")

// NewCachedByteCodeLoader creates a new CachedByteCodeLoader
func NewCachedByteCodeLoader(
	cache Cache,
	cacheLevel CacheStrategy,
	fetcher TemplateFetcher,
	parser parser.Parser,
	compiler compiler.Compiler,
) *CachedByteCodeLoader {
	return &CachedByteCodeLoader{
		NewStringByteCodeLoader(parser, compiler),
		NewReaderByteCodeLoader(parser, compiler),
		fetcher,
		[]Cache{MemoryCache{}, cache},
		cacheLevel,
	}
}

func (l *CachedByteCodeLoader) DumpAST(v bool) {
	l.StringByteCodeLoader.DumpAST(v)
	l.ReaderByteCodeLoader.DumpAST(v)
}

func (l *CachedByteCodeLoader) DumpByteCode(v bool) {
	l.StringByteCodeLoader.DumpByteCode(v)
	l.ReaderByteCodeLoader.DumpByteCode(v)
}

func (l *CachedByteCodeLoader) ShouldDumpAST() bool {
	return l.StringByteCodeLoader.ShouldDumpAST() || l.ReaderByteCodeLoader.ShouldDumpAST()
}

func (l *CachedByteCodeLoader) ShouldDumpByteCode() bool {
	return l.StringByteCodeLoader.ShouldDumpByteCode() || l.ReaderByteCodeLoader.ShouldDumpByteCode()
}

// Load loads the ByteCode for template specified by `key`, which, for this
// ByteCodeLoader, is the path to the template we want.
// If cached vm.ByteCode struct is found, it is loaded and its last modified
// time is compared against that of the template file. If the template is
// newer, it's compiled. Otherwise the cached version is used, saving us the
// time to parse and compile the template.
func (l *CachedByteCodeLoader) Load(key string) (bc *vm.ByteCode, err error) {
	defer func() {
		if bc != nil && err == nil && l.ShouldDumpByteCode() {
			fmt.Fprintf(os.Stderr, "%s\n", bc.String())
		}
	}()

	var source TemplateSource
	if l.CacheLevel > CacheNone {
		var entity *CacheEntity
		for _, cache := range l.Caches {
			entity, err = cache.Get(key)
			if err == nil {
				break
			}
		}

		if err == nil {
			if l.CacheLevel == CacheNoVerify {
				return entity.ByteCode, nil
			}

			t, err := entity.Source.LastModified()
			if err != nil {
				return nil, err
			}

			if t.Before(entity.ByteCode.GeneratedOn) {
				return entity.ByteCode, nil
			}

			// ByteCode validation failed, but we can still re-use source
			source = entity.Source
		}
	}

	if source == nil {
		source, err = l.Fetcher.FetchTemplate(key)
		if err != nil {
			return nil, err
		}
	}

	rdr, err := source.Reader()
	if err != nil {
		return nil, err
	}

	bc, err = l.LoadReader(key, rdr)
	if err != nil {
		return nil, err
	}

	entity := &CacheEntity{bc, source}
	for _, cache := range l.Caches {
		cache.Set(key, entity)
	}

	return bc, nil
}

// NewFileCache creates a new FileCache which stores caches underneath
// the directory specified by `dir`
func NewFileCache(dir string) (*FileCache, error) {
	f := &FileCache{dir}
	return f, nil
}

// GetCachePath creates a string describing where a given template key
// would be cached in the file system
func (c *FileCache) GetCachePath(key string) string {
	// What's the best, portable way to remove make an absolute path into
	// a relative path?
	key = filepath.Clean(key)
	key = strings.TrimPrefix(key, "/")
	return filepath.Join(c.Dir, key)
}

// Get returns the cached vm.ByteCode, if available
func (c *FileCache) Get(key string) (*CacheEntity, error) {
	path := c.GetCachePath(key)

	// Need to avoid race condition
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entity CacheEntity
	dec := gob.NewDecoder(file)
	if err = dec.Decode(&entity); err != nil {
		return nil, err
	}

	return &entity, nil
}

// Set creates a new cache file to store the ByteCode.
func (c *FileCache) Set(key string, entity *CacheEntity) error {
	path := c.GetCachePath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return err
	}

	// Need to avoid race condition
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	f := bufio.NewWriter(file)
	defer f.Flush()
	enc := gob.NewEncoder(f)
	if err = enc.Encode(entity); err != nil {
		return err
	}

	return nil
}

// Delete deletes the cache
func (c *FileCache) Delete(key string) error {
	return os.Remove(c.GetCachePath(key))
}

// Get returns the cached ByteCode
func (c MemoryCache) Get(key string) (*CacheEntity, error) {
	bc, ok := c[key]
	if !ok {
		return nil, ErrCacheMiss
	}
	return bc, nil
}

// Set stores the ByteCode
func (c MemoryCache) Set(key string, bc *CacheEntity) error {
	c[key] = bc
	return nil
}

// Delete deletes the ByteCode
func (c MemoryCache) Delete(key string) error {
	delete(c, key)
	return nil
}
