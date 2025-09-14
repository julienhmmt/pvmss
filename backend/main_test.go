package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Run the main application in a goroutine
	go main()

	// Give the server a moment to start
	// A more robust solution would be to poll the endpoint
	time.Sleep(2 * time.Second)

	// Run the tests
	code := m.Run()

	// The test process will exit, and the `main` goroutine will be terminated.
	os.Exit(code)
}

func TestSearchEn(t *testing.T) {
	resp, err := http.Get("http://localhost:50000/search?lang=en")
	assert.NoError(t, err, "Should be able to get search?lang=en")
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 for search?lang=en")
}

func TestSearchFr(t *testing.T) {
	resp, err := http.Get("http://localhost:50000/search?lang=fr")
	assert.NoError(t, err, "Should be able to get search?lang=fr")
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 for search?lang=fr")
}
