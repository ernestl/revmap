package cmd

import (
	"errors"

	"github.com/ernestl/revmap/store"
)

// isCacheFallbackErr returns true if the error indicates a permission
// or access issue (401, 403, 404) where falling back to cache is appropriate.
func isCacheFallbackErr(err error) bool {
	var storeErr *store.StoreError
	if errors.As(err, &storeErr) {
		switch storeErr.StatusCode {
		case 401, 403, 404:
			return true
		}
	}
	return false
}
