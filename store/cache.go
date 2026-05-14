package store

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheData holds the complete cached history for a single snap.
type CacheData struct {
	Snap      string                            `json:"snap"`
	CachedAt  time.Time                         `json:"cached_at"`
	Revisions []RevisionEntry                   `json:"revisions"`
	Details   map[string]map[string]interface{} `json:"details"`
}

// WriteCache serializes cache data to a gzipped JSON file.
func WriteCache(path string, data *CacheData) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create cache directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create cache file: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	enc := json.NewEncoder(gw)
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("cannot encode cache data: %w", err)
	}

	return nil
}

// ReadCache reads and decompresses a gzipped JSON cache file.
func ReadCache(path string) (*CacheData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("cannot decompress cache file: %w", err)
	}
	defer gr.Close()

	var data CacheData
	if err := json.NewDecoder(gr).Decode(&data); err != nil {
		return nil, fmt.Errorf("cannot decode cache data: %w", err)
	}

	return &data, nil
}

// FindCacheFile locates the cache file for a snap by searching known
// locations: $SNAP/cache/, ./cache/, next to the executable, and the
// current working directory itself.
func FindCacheFile(snapName string) string {
	filename := snapName + ".json.gz"

	// Check $SNAP/cache/ (inside the snap).
	if snap := os.Getenv("SNAP"); snap != "" {
		p := filepath.Join(snap, "cache", filename)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Check next to the executable.
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "cache", filename)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Check cache/ subdirectory of current working directory.
	p := filepath.Join("cache", filename)
	if _, err := os.Stat(p); err == nil {
		return p
	}

	// Check current working directory directly (e.g. running from inside cache/).
	if _, err := os.Stat(filename); err == nil {
		return filename
	}

	return ""
}

// LoadCacheSnaps reads the cache configuration file listing snap names.
func LoadCacheSnaps(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var snaps []string
	if err := json.Unmarshal(data, &snaps); err != nil {
		return nil, fmt.Errorf("cannot parse cache config %s: %w", path, err)
	}

	return snaps, nil
}

// FindCacheConfig locates the cache-snaps.json config file.
func FindCacheConfig() string {
	// Check current working directory first.
	if _, err := os.Stat("cache-snaps.json"); err == nil {
		return "cache-snaps.json"
	}

	// Check $SNAP/.
	if snap := os.Getenv("SNAP"); snap != "" {
		p := filepath.Join(snap, "cache-snaps.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Next to executable.
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "cache-snaps.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
