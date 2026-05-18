package traefikwildcardhostrewrite

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRewriteRegexHost(t *testing.T) {
	var gotHost string
	var gotForwardedHost string
	var gotOriginalHost string

	next := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		gotHost = req.Host
		gotForwardedHost = req.Header.Get("X-Forwarded-Host")
		gotOriginalHost = req.Header.Get("X-Original-Host")
	})

	handler, err := New(context.Background(), next, &Config{
		Rules: []Rule{
			{
				Pattern:     `^preview-([a-z0-9-]+)\.example\.com$`,
				Replacement: `$1.internal.example.net`,
			},
		},
		AllowedSuffixes:       []string{".internal.example.net"},
		PreserveForwardedHost: true,
		OriginalHostHeader:    "X-Original-Host",
	}, "test")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://preview-api.example.com:8080/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotHost != "api.internal.example.net:8080" {
		t.Fatalf("expected rewritten host, got %q", gotHost)
	}
	if gotForwardedHost != "preview-api.example.com:8080" {
		t.Fatalf("expected original X-Forwarded-Host, got %q", gotForwardedHost)
	}
	if gotOriginalHost != "preview-api.example.com:8080" {
		t.Fatalf("expected original host header, got %q", gotOriginalHost)
	}
}

func TestSkipNonMatchingHost(t *testing.T) {
	var gotHost string

	next := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		gotHost = req.Host
	})

	handler, err := New(context.Background(), next, &Config{
		Rules: []Rule{
			{
				Pattern:     `^preview-([a-z0-9-]+)\.example\.com$`,
				Replacement: `$1.internal.example.net`,
			},
		},
	}, "test")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://api.other.com/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotHost != "api.other.com" {
		t.Fatalf("expected host to remain unchanged, got %q", gotHost)
	}
}

func TestMultipleRules(t *testing.T) {
	var gotHost string

	next := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		gotHost = req.Host
	})

	handler, err := New(context.Background(), next, &Config{
		Rules: []Rule{
			{
				Pattern:     `^preview-([a-z0-9-]+)\.example\.com$`,
				Replacement: `$1.internal.example.net`,
			},
			{
				Pattern:     `^([a-z0-9-]+)\.apps\.example\.com$`,
				Replacement: `$1.internal.example.net`,
			},
		},
	}, "test")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://api.apps.example.com/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotHost != "api.internal.example.net" {
		t.Fatalf("expected rewritten host from second rule, got %q", gotHost)
	}
}

func TestOriginalHostHeaderOverwritesClientValue(t *testing.T) {
	var gotOriginalHost string

	next := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		gotOriginalHost = req.Header.Get("X-Original-Host")
	})

	handler, err := New(context.Background(), next, &Config{
		Rules: []Rule{
			{
				Pattern:     `^preview-([a-z0-9-]+)\.example\.com$`,
				Replacement: `$1.internal.example.net`,
			},
		},
		OriginalHostHeader: "X-Original-Host",
	}, "test")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://preview-api.example.com/", nil)
	req.Header.Set("X-Original-Host", "spoofed.example")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotOriginalHost != "preview-api.example.com" {
		t.Fatalf("expected authoritative original host header, got %q", gotOriginalHost)
	}
}

func TestAllowedSuffixBlocksUnexpectedRewrite(t *testing.T) {
	var gotHost string

	next := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		gotHost = req.Host
	})

	handler, err := New(context.Background(), next, &Config{
		Rules: []Rule{
			{
				Pattern:     `^preview-([a-z0-9-]+)\.example\.com$`,
				Replacement: `$1.example.net`,
			},
		},
		AllowedSuffixes: []string{".internal.example.net"},
	}, "test")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://preview-api.example.com/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotHost != "preview-api.example.com" {
		t.Fatalf("expected blocked rewrite to leave host unchanged, got %q", gotHost)
	}
}

func TestInvalidConfig(t *testing.T) {
	_, err := New(context.Background(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), &Config{
		Rules: []Rule{
			{
				Pattern:     "(",
				Replacement: "$1.internal.example.net",
			},
		},
	}, "test")
	if err == nil {
		t.Fatal("expected invalid pattern error")
	}
}
