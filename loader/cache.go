package loader

import(
  "encoding/gob"
  "errors"
  "os"
  "path/filepath"
  "strings"

  "github.com/lestrrat/go-xslate/compiler"
  "github.com/lestrrat/go-xslate/parser"
  "github.com/lestrrat/go-xslate/vm"
)

// ErrCacheMiss is returned when the bytecode could not be found in the cache
var ErrCacheMiss = errors.New("cache miss")

// CacheStrategy specifies how the cache should be checked
type CacheStrategy int
const (
  // CacheNone flag specifies that cache checking and setting hould be skipped
  CacheNone CacheStrategy = iota
  // CacheVerify flag specifies that cached ByteCode generation time should be 
  // verified against the source's last modified time. If new, the source is
  // re-parsed and re-compiled even on a cache hit.
  CacheVerify
  // CacheNoVerify flag specifies that if we have a cache hit, the ByteCode
  // is not verified against the source. If there's a cache hit, it is
  // used regardless of updates to the original template on file system
  CacheNoVerify
)


// Cache defines the interface for things that can cache generated ByteCode
type Cache interface {
  Get(string) (*vm.ByteCode, error)
  Set(string, *vm.ByteCode) error
  Delete(string) error
}

// CachedByteCodeLoader is the default ByteCodeLoader that loads templates
// from the file system and caches in the file system, too
type CachedByteCodeLoader struct {
  *StringByteCodeLoader // gives us LoadString
  Fetcher TemplateFetcher
  Caches []Cache
  CacheLevel CacheStrategy
}

// NewCachedByteCodeLoader creates a new CachedByteCodeLoader
func NewCachedByteCodeLoader(
  cache Cache,
  cacheLevel CacheStrategy,
  fetcher TemplateFetcher,
  parser parser.Parser,
  compiler compiler.Compiler,
) *CachedByteCodeLoader {
  return &CachedByteCodeLoader {
    NewStringByteCodeLoader(parser, compiler),
    fetcher,
    []Cache { MemoryCache {}, cache },
    cacheLevel,
  }
}

// Load loads the ByteCode for template specified by `key`, which, for this
// ByteCodeLoader, is the path to the template we want.
// If cached vm.ByteCode struct is found, it is loaded and its last modified
// time is compared against that of the template file. If the template is
// newer, it's compiled. Otherwise the cached version is used, saving us the
// time to parse and compile the template.
func (l *CachedByteCodeLoader) Load(key string) (*vm.ByteCode, error) {
  var bc *vm.ByteCode
  var err error
  var source TemplateSource

  if l.CacheLevel > CacheNone {
    for _, cache := range l.Caches {
      bc, err = cache.Get(key)
      if err == nil {
        break
      }
    }

    if err == nil {
      if l.CacheLevel == CacheNoVerify {
        return bc, nil
      }

      source, err = l.Fetcher.FetchTemplate(key)
      if err != nil {
        return nil, err
      }

      t, err := source.LastModified()
      if err != nil {
        return nil, err
      }

      if t.Before(bc.GeneratedOn) {
        return bc, nil
      }
    }
  }

  if source == nil {
    source, err = l.Fetcher.FetchTemplate(key)
  }

  if err != nil {
    return nil, err
  }

  content, err := source.Bytes()
  if err != nil {
    return nil, err
  }

  bc, err = l.LoadString(string(content))
  if err != nil {
    return nil, err
  }

  for _, cache := range l.Caches {
    cache.Set(key, bc)
  }

  return bc, nil
}

// FileCache is Cache implementation that stores caches in the file system
type FileCache struct {
  Dir string
}

// NewFileCache creates a new FileCache which stores caches underneath
// the directory specified by `dir`
func NewFileCache(dir string) (*FileCache, error) {
  f := &FileCache { dir }

STAT:
  fi, err := os.Stat(dir)
  if err != nil { // non-existing dir
    if err = os.MkdirAll(dir, 0777); err != nil {
      return nil, err
    }
    goto STAT
  }

  if ! fi.IsDir() {
    return nil, errors.New("error: Specified directory is not a directory")
  }

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
func (c *FileCache) Get(key string) (*vm.ByteCode, error) {
  path := c.GetCachePath(key)

  // Need to avoid race condition
  file, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer file.Close()

  var bc vm.ByteCode
  dec := gob.NewDecoder(file)
  if err = dec.Decode(&bc); err != nil {
    return nil, err
  }

  return &bc, nil
}

// Set creates a new cache file to store the ByteCode.
func (c *FileCache) Set(key string, bc *vm.ByteCode) error {
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

  enc := gob.NewEncoder(file)
  if err = enc.Encode(bc); err != nil {
    return err
  }

  return nil
}

// Delete deletes the cache
func (c *FileCache) Delete(key string) error {
  return os.Remove(c.GetCachePath(key))
}

// MemoryCache is what's used store cached ByteCode in memory for maximum
// speed. As of this writing this cache never freed. We may need to
// introduce LRU in the future
type MemoryCache map[string]*vm.ByteCode

// Get returns the cached ByteCode
func (c MemoryCache) Get(key string) (*vm.ByteCode, error) {
  bc, ok := c[key]
  if !ok {
    return nil, ErrCacheMiss
  }
  return bc, nil
}

// Set stores the ByteCode
func (c MemoryCache) Set(key string, bc *vm.ByteCode) error {
  c[key] = bc
  return nil
}

// Delete deletes the ByteCode
func (c MemoryCache) Delete(key string) error {
  delete(c, key)
  return nil
}
