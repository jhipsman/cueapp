package api_test

import (
	"os"
	"testing"

	"github.com/rdborg/mediarium/internal/archiveorg"
	"github.com/rdborg/mediarium/internal/books"
	"github.com/rdborg/mediarium/internal/introdb"
)

// TestMain keeps the tests off the internet: lookups that would go to
// Audible, Audnexus and the Internet Archive go to a closed local port and fail straight away.
func TestMain(m *testing.M) {
	books.AudibleSearchURL = "http://127.0.0.1:1/catalog/products"
	books.AudnexusURL = "http://127.0.0.1:1"
	archiveorg.DefaultBase = "http://127.0.0.1:1"
	introdb.DefaultBase = "http://127.0.0.1:1"
	os.Exit(m.Run())
}
