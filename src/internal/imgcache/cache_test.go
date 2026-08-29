package imgcache

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestImageCachingReturnsPublicCacheURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()

	cacheDir := t.TempDir() + string(os.PathSeparator)
	cacheURL := "http://threadfin.example/images/"

	cache, err := New(cacheDir, cacheURL, true)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	sourceURL := server.URL + "/logo.png?token=secret"
	if got := cache.Image.GetURL(sourceURL, "", "", false, 0, ""); got != sourceURL {
		t.Fatalf("first GetURL = %q, want original source URL", got)
	}

	cache.Image.Caching()
	if len(cache.Queue) != 0 {
		t.Fatalf("cache queue length = %d, want 0", len(cache.Queue))
	}

	got := cache.Image.GetURL(sourceURL, "", "", false, 0, "")
	if !strings.HasPrefix(got, cacheURL) {
		t.Fatalf("cached URL = %q, want prefix %q", got, cacheURL)
	}
	if strings.Contains(got, cacheDir) {
		t.Fatalf("cached URL leaked filesystem path %q in %q", cacheDir, got)
	}
	if !strings.HasSuffix(got, ".png") {
		t.Fatalf("cached URL = %q, want .png suffix", got)
	}
}
