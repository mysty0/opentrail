package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPServer_StaticFileServing(t *testing.T) {
	server, cleanup := setupTestHTTPServer(t)
	defer cleanup()
	
	// Create test server
	mux := http.NewServeMux()
	server.setupRoutes(mux)
	testServer := httptest.NewServer(mux)
	defer testServer.Close()
	
	// Test serving index.html
	resp, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatalf("Failed to get index page: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for index page, got %d", resp.StatusCode)
	}
	
	// Check content type for HTML
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}
}

func TestHTTPServer_StaticCSS(t *testing.T) {
	server, cleanup := setupTestHTTPServer(t)
	defer cleanup()
	
	// Create test server
	mux := http.NewServeMux()
	server.setupRoutes(mux)
	testServer := httptest.NewServer(mux)
	defer testServer.Close()
	
	// Test serving CSS file
	resp, err := http.Get(testServer.URL + "/static/style.css")
	if err != nil {
		t.Fatalf("Failed to get CSS file: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for CSS file, got %d", resp.StatusCode)
	}
	
	// Check content type for CSS
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/css" {
		t.Errorf("Expected CSS content type, got %s", contentType)
	}
}

func TestHTTPServer_StaticJS(t *testing.T) {
	server, cleanup := setupTestHTTPServer(t)
	defer cleanup()
	
	// Create test server
	mux := http.NewServeMux()
	server.setupRoutes(mux)
	testServer := httptest.NewServer(mux)
	defer testServer.Close()
	
	// Test serving JavaScript file
	resp, err := http.Get(testServer.URL + "/static/app.js")
	if err != nil {
		t.Fatalf("Failed to get JS file: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for JS file, got %d", resp.StatusCode)
	}
	
	// Check content type for JavaScript
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/javascript" {
		t.Errorf("Expected JavaScript content type, got %s", contentType)
	}
}

func TestHTTPServer_StaticFileNotFound(t *testing.T) {
	server, cleanup := setupTestHTTPServer(t)
	defer cleanup()
	
	// Create test server
	mux := http.NewServeMux()
	server.setupRoutes(mux)
	testServer := httptest.NewServer(mux)
	defer testServer.Close()
	
	// Test requesting non-existent static file
	resp, err := http.Get(testServer.URL + "/static/nonexistent.css")
	if err != nil {
		t.Fatalf("Failed to request non-existent file: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent file, got %d", resp.StatusCode)
	}
}

func TestHTTPServer_IndexWithAuth(t *testing.T) {
	server, cleanup := setupTestHTTPServerWithAuth(t)
	defer cleanup()
	
	// Create test server
	mux := http.NewServeMux()
	server.setupRoutes(mux)
	testServer := httptest.NewServer(mux)
	defer testServer.Close()
	
	// Test index page without auth - should fail
	resp, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatalf("Failed to get index page: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for index page without auth, got %d", resp.StatusCode)
	}
	
	// Test index page with auth - should succeed
	req, err := http.NewRequest("GET", testServer.URL+"/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.SetBasicAuth("admin", "password")
	
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to get index page with auth: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for index page with auth, got %d", resp.StatusCode)
	}
}

func TestHTTPServer_StaticFilesNoAuth(t *testing.T) {
	server, cleanup := setupTestHTTPServerWithAuth(t)
	defer cleanup()
	
	// Create test server
	mux := http.NewServeMux()
	server.setupRoutes(mux)
	testServer := httptest.NewServer(mux)
	defer testServer.Close()
	
	// Test that static files don't require authentication
	// (This is a design decision - static assets should be publicly accessible)
	resp, err := http.Get(testServer.URL + "/static/style.css")
	if err != nil {
		t.Fatalf("Failed to get CSS file: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for CSS file without auth, got %d", resp.StatusCode)
	}
}