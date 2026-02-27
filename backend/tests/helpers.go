// Package tests provides testing utilities and helpers for PVMSS.
// These helpers reduce boilerplate in test files and ensure consistent
// testing patterns across the codebase.
package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequest represents a test HTTP request configuration.
type TestRequest struct {
	Method  string
	Path    string
	Body    interface{}
	Headers map[string]string
	Cookies []*http.Cookie
}

// TestResponse represents the result of a test HTTP request.
type TestResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	Cookies    []*http.Cookie
}

// RequestBuilder provides a fluent interface for building test requests.
type RequestBuilder struct {
	t       *testing.T
	method  string
	path    string
	body    io.Reader
	headers map[string]string
	cookies []*http.Cookie
}

// NewRequest creates a new RequestBuilder.
func NewRequest(t *testing.T, method, path string) *RequestBuilder {
	t.Helper()
	return &RequestBuilder{
		t:       t,
		method:  method,
		path:    path,
		headers: make(map[string]string),
	}
}

// WithBody sets the request body as JSON.
func (rb *RequestBuilder) WithBody(body interface{}) *RequestBuilder {
	rb.t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		rb.t.Fatalf("failed to marshal body: %v", err)
	}
	rb.body = bytes.NewReader(data)
	rb.headers["Content-Type"] = "application/json"
	return rb
}

// WithRawBody sets the request body as raw bytes.
func (rb *RequestBuilder) WithRawBody(body []byte) *RequestBuilder {
	rb.body = bytes.NewReader(body)
	return rb
}

// WithHeader adds a header to the request.
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

// WithCookie adds a cookie to the request.
func (rb *RequestBuilder) WithCookie(cookie *http.Cookie) *RequestBuilder {
	rb.cookies = append(rb.cookies, cookie)
	return rb
}

// WithAuth adds an authorization header.
func (rb *RequestBuilder) WithAuth(token string) *RequestBuilder {
	rb.headers["Authorization"] = "Bearer " + token
	return rb
}

// Build creates the http.Request.
func (rb *RequestBuilder) Build() *http.Request {
	rb.t.Helper()
	req := httptest.NewRequest(rb.method, rb.path, rb.body)
	for k, v := range rb.headers {
		req.Header.Set(k, v)
	}
	for _, c := range rb.cookies {
		req.AddCookie(c)
	}
	return req
}

// Execute runs the request against a handler and returns the response.
func (rb *RequestBuilder) Execute(handler http.Handler) *TestResponse {
	rb.t.Helper()
	req := rb.Build()
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return &TestResponse{
		StatusCode: rr.Code,
		Body:       rr.Body.Bytes(),
		Headers:    rr.Header(),
		Cookies:    rr.Result().Cookies(),
	}
}

// AssertStatus asserts the response status code.
func (tr *TestResponse) AssertStatus(t *testing.T, expected int) *TestResponse {
	t.Helper()
	if tr.StatusCode != expected {
		t.Errorf("status code = %d, want %d", tr.StatusCode, expected)
	}
	return tr
}

// AssertHeader asserts a response header value.
func (tr *TestResponse) AssertHeader(t *testing.T, key, expected string) *TestResponse {
	t.Helper()
	if got := tr.Headers.Get(key); got != expected {
		t.Errorf("header %s = %q, want %q", key, got, expected)
	}
	return tr
}

// AssertBodyContains asserts the response body contains a substring.
func (tr *TestResponse) AssertBodyContains(t *testing.T, substring string) *TestResponse {
	t.Helper()
	if !bytes.Contains(tr.Body, []byte(substring)) {
		t.Errorf("body does not contain %q", substring)
	}
	return tr
}

// AssertJSON unmarshals the response body into the provided struct.
func (tr *TestResponse) AssertJSON(t *testing.T, v interface{}) *TestResponse {
	t.Helper()
	if err := json.Unmarshal(tr.Body, v); err != nil {
		t.Errorf("failed to unmarshal JSON: %v", err)
	}
	return tr
}

// BodyString returns the response body as a string.
func (tr *TestResponse) BodyString() string {
	return string(tr.Body)
}

// MockHandler creates a simple mock HTTP handler.
func MockHandler(statusCode int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}
}

// MockJSONHandler creates a mock handler that returns JSON.
func MockJSONHandler(statusCode int, data interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(data)
	}
}

// AssertEqual is a generic equality assertion.
func AssertEqual[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

// AssertNotEqual is a generic inequality assertion.
func AssertNotEqual[T comparable](t *testing.T, got, notWant T, msg string) {
	t.Helper()
	if got == notWant {
		t.Errorf("%s: got %v, should not equal %v", msg, got, notWant)
	}
}

// AssertNil asserts that a value is nil.
func AssertNil(t *testing.T, got interface{}, msg string) {
	t.Helper()
	if got != nil {
		t.Errorf("%s: got %v, want nil", msg, got)
	}
}

// AssertNotNil asserts that a value is not nil.
func AssertNotNil(t *testing.T, got interface{}, msg string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: got nil, want non-nil", msg)
	}
}

// AssertError asserts that an error is not nil.
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error, got nil", msg)
	}
}

// AssertNoError asserts that an error is nil.
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: unexpected error: %v", msg, err)
	}
}

// AssertTrue asserts that a condition is true.
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Errorf("%s: expected true, got false", msg)
	}
}

// AssertFalse asserts that a condition is false.
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Errorf("%s: expected false, got true", msg)
	}
}

// AssertLen asserts the length of a slice or map.
func AssertLen[T any](t *testing.T, slice []T, expected int, msg string) {
	t.Helper()
	if len(slice) != expected {
		t.Errorf("%s: len = %d, want %d", msg, len(slice), expected)
	}
}

// AssertContains asserts that a slice contains an element.
func AssertContains[T comparable](t *testing.T, slice []T, element T, msg string) {
	t.Helper()
	for _, item := range slice {
		if item == element {
			return
		}
	}
	t.Errorf("%s: slice does not contain %v", msg, element)
}

// TableTest represents a single test case in a table-driven test.
type TableTest[I, O any] struct {
	Name     string
	Input    I
	Expected O
	WantErr  bool
}

// RunTableTests runs a table-driven test with the provided test function.
func RunTableTests[I, O comparable](t *testing.T, tests []TableTest[I, O], fn func(I) (O, error)) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := fn(tt.Input)
			if (err != nil) != tt.WantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.WantErr)
				return
			}
			if !tt.WantErr && got != tt.Expected {
				t.Errorf("got %v, want %v", got, tt.Expected)
			}
		})
	}
}
