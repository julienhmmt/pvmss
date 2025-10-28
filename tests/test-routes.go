package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Configuration
type Config struct {
	BaseURL       string
	AdminUsername string
	AdminPassword string
	UserUsername  string
	UserPassword  string
	Verbose       bool
}

// Couleurs ANSI
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[0;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[1;33m"
	ColorBlue   = "\033[0;34m"
)

// Compteurs de tests
type TestStats struct {
	Total  int
	Passed int
	Failed int
}

// Test représente un test de route
type Test struct {
	Name           string
	Method         string
	Path           string
	ExpectedStatus int
	NeedsAuth      bool
	NeedsAdmin     bool
}

func main() {
	// Configuration depuis les flags et variables d'environnement
	config := Config{
		BaseURL:       getEnvOrDefault("BASE_URL", "http://localhost:50000"),
		AdminUsername: getEnvOrDefault("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnvOrDefault("ADMIN_PASSWORD", "admin"),
		UserUsername:  getEnvOrDefault("USER_USERNAME", "jhmt@pve"),
		UserPassword:  getEnvOrDefault("USER_PASSWORD", "pouetpouet"),
	}

	verbose := flag.Bool("verbose", getEnvOrDefault("VERBOSE", "0") == "1", "Mode verbose")
	flag.Parse()
	config.Verbose = *verbose

	// Affichage de l'en-tête
	printHeader(config.BaseURL)

	// Attendre que le serveur soit prêt
	if !waitForServer(config.BaseURL, 30*time.Second) {
		logError("Le serveur n'est pas accessible")
		os.Exit(1)
	}
	logSuccess("Server is ready!")

	stats := &TestStats{}

	// Tests des routes publiques
	fmt.Println()
	logSection("Testing PUBLIC routes (no authentication)")
	testPublicRoutes(config, stats)

	// Tests des routes protégées sans auth
	fmt.Println()
	logSection("Testing PROTECTED routes without authentication")
	testProtectedRoutesNoAuth(config, stats)

	// Tests des routes authentifiées (user)
	fmt.Println()
	logSection("Testing AUTHENTICATED routes (user privileges)")
	testUserRoutes(config, stats)

	// Tests des routes admin
	fmt.Println()
	logSection("Testing ADMIN routes (admin privileges)")
	testAdminRoutes(config, stats)

	// Tests des routes 404
	fmt.Println()
	logSection("Testing 404 routes (should not exist)")
	test404Routes(config, stats)

	// Affichage du résumé
	printSummary(stats)

	// Code de sortie
	if stats.Failed > 0 {
		os.Exit(1)
	}
}

func waitForServer(baseURL string, timeout time.Duration) bool {
	logInfo("Waiting for server to be ready...")
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func testPublicRoutes(config Config, stats *TestStats) {
	tests := []Test{
		{"Health check", "GET", "/health", 200, false, false},
		{"API health check", "GET", "/api/health", 200, false, false},
		{"Proxmox health check", "GET", "/api/health/proxmox", 200, false, false},
		{"User login page", "GET", "/login", 200, false, false},
		{"Admin login page", "GET", "/admin/login", 200, false, false},
		{"Documentation page (default)", "GET", "/docs", 200, false, false},
		{"User documentation", "GET", "/docs/user", 200, false, false},
		{"Admin documentation", "GET", "/docs/admin", 200, false, false},
		{"Favicon", "GET", "/favicon.ico", 200, false, false},
		{"Base CSS", "GET", "/css/base.css", 200, false, false},
		{"Accessibility JS", "GET", "/js/accessibility.js", 200, false, false},
	}

	for _, test := range tests {
		runTest(config, test, nil, stats)
	}
}

func testProtectedRoutesNoAuth(config Config, stats *TestStats) {
	tests := []Test{
		{"Profile without auth (should redirect)", "GET", "/profile", 303, false, false},
		{"VM create without auth (should redirect)", "GET", "/vm/create", 303, false, false},
		{"Search without auth (should redirect)", "GET", "/search", 303, false, false},
		{"Admin dashboard without auth (should redirect)", "GET", "/admin", 303, false, false},
		{"Admin nodes without auth (should redirect)", "GET", "/admin/nodes", 303, false, false},
		{"Admin tags without auth (should redirect)", "GET", "/admin/tags", 303, false, false},
	}

	for _, test := range tests {
		runTest(config, test, nil, stats)
	}
}

func testUserRoutes(config Config, stats *TestStats) {
	// Authentification utilisateur
	client := createHTTPClient()
	if !authenticate(config, client, config.UserUsername, config.UserPassword, "/login") {
		logError("User authentication failed")
		return
	}
	logSuccess("User authenticated successfully")

	tests := []Test{
		{"Home page", "GET", "/", 200, true, false},
		{"Search page", "GET", "/search", 200, true, false},
		{"User profile", "GET", "/profile", 200, true, false},
		{"VM creation page", "GET", "/vm/create", 200, true, false},
		{"Get settings API", "GET", "/api/settings", 200, true, false},
		{"Get all settings API", "GET", "/api/settings/all", 200, true, false},
		{"Get all VMBR API", "GET", "/api/vmbr/all", 200, true, false},
		{"Logout (GET redirect)", "GET", "/logout", 303, true, false},
	}

	for _, test := range tests {
		runTest(config, test, client, stats)
	}
}

func testAdminRoutes(config Config, stats *TestStats) {
	// Authentification admin
	client := createHTTPClient()
	if !authenticate(config, client, config.AdminUsername, config.AdminPassword, "/admin/login") {
		logError("Admin authentication failed")
		return
	}
	logSuccess("Admin authenticated successfully")

	tests := []Test{
		{"Admin dashboard", "GET", "/admin", 200, true, true},
		{"Admin nodes page", "GET", "/admin/nodes", 200, true, true},
		{"Admin tags page", "GET", "/admin/tags", 200, true, true},
		{"Admin storage page", "GET", "/admin/storage", 200, true, true},
		{"Admin ISO page", "GET", "/admin/iso", 200, true, true},
		{"Admin VMBR page", "GET", "/admin/vmbr", 200, true, true},
		{"Admin limits page", "GET", "/admin/limits", 200, true, true},
		{"Admin users page", "GET", "/admin/userpool", 200, true, true},
		{"Admin appinfo page", "GET", "/admin/appinfo", 200, true, true},
	}

	for _, test := range tests {
		runTest(config, test, client, stats)
	}
}

func test404Routes(config Config, stats *TestStats) {
	tests := []Test{
		{"Nonexistent route", "GET", "/nonexistent", 404, false, false},
		{"Nonexistent API route", "GET", "/api/nonexistent", 404, false, false},
		{"Nonexistent admin route", "GET", "/admin/nonexistent", 404, false, false},
	}

	for _, test := range tests {
		runTest(config, test, nil, stats)
	}
}

func runTest(config Config, test Test, client *http.Client, stats *TestStats) {
	stats.Total++
	logInfo(fmt.Sprintf("Testing: %s %s", test.Method, test.Path))

	if client == nil {
		client = createHTTPClient()
	}

	req, err := http.NewRequest(test.Method, config.BaseURL+test.Path, nil)
	if err != nil {
		logError(fmt.Sprintf("%s: %s %s → Error: %v", test.Name, test.Method, test.Path, err))
		stats.Failed++
		return
	}

	// Ne pas suivre les redirections automatiquement
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		logError(fmt.Sprintf("%s: %s %s → Error: %v", test.Name, test.Method, test.Path, err))
		stats.Failed++
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == test.ExpectedStatus {
		logSuccess(fmt.Sprintf("%s: %s %s → %d", test.Name, test.Method, test.Path, resp.StatusCode))
		stats.Passed++
	} else {
		logError(fmt.Sprintf("%s: %s %s → Expected: %d, Got: %d", test.Name, test.Method, test.Path, test.ExpectedStatus, resp.StatusCode))
		stats.Failed++
	}
}

func authenticate(config Config, client *http.Client, username, password, loginPath string) bool {
	logInfo(fmt.Sprintf("Authenticating %s: %s", loginPath, username))

	// Récupérer le formulaire de login pour obtenir le CSRF token
	resp, err := client.Get(config.BaseURL + loginPath)
	if err != nil {
		if config.Verbose {
			logError(fmt.Sprintf("Failed to get login page: %v", err))
		}
		return false
	}
	
	// Lire le contenu de la page pour extraire le CSRF token
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		if config.Verbose {
			logError(fmt.Sprintf("Failed to read login page: %v", err))
		}
		return false
	}

	// Extraire le CSRF token de la page HTML
	csrfToken := extractCSRFToken(string(body))
	if csrfToken == "" && config.Verbose {
		logError("No CSRF token found in login page")
	}

	// Préparer les données du formulaire
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)
	if csrfToken != "" {
		data.Set("csrf_token", csrfToken)
	}

	// Envoyer la requête de login
	req, err := http.NewRequest("POST", config.BaseURL+loginPath, strings.NewReader(data.Encode()))
	if err != nil {
		if config.Verbose {
			logError(fmt.Sprintf("Failed to create login request: %v", err))
		}
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Ne pas suivre les redirections pour vérifier le code 303
	oldCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { client.CheckRedirect = oldCheckRedirect }()

	resp, err = client.Do(req)
	if err != nil {
		if config.Verbose {
			logError(fmt.Sprintf("Failed to send login request: %v", err))
		}
		return false
	}
	defer resp.Body.Close()

	// Vérifier la redirection (303 = succès)
	success := resp.StatusCode == 303 || resp.StatusCode == 302
	
	if !success && config.Verbose {
		logError(fmt.Sprintf("Login failed with status: %d", resp.StatusCode))
		bodyBytes, _ := io.ReadAll(resp.Body)
		if len(bodyBytes) > 0 && len(bodyBytes) < 500 {
			logError(fmt.Sprintf("Response: %s", string(bodyBytes)))
		}
	}
	
	return success
}

// extractCSRFToken extrait le token CSRF d'une page HTML
func extractCSRFToken(html string) string {
	// Chercher le token CSRF dans les meta tags ou inputs cachés
	patterns := []string{
		`<meta name="csrf-token" content="([^"]+)"`,
		`<input[^>]*name="csrf_token"[^>]*value="([^"]+)"`,
		`<input[^>]*value="([^"]+)"[^>]*name="csrf_token"`,
		`name="csrf_token" value="([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 && matches[1] != "" {
			return matches[1]
		}
	}

	return ""
}

func createHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func printHeader(baseURL string) {
	fmt.Println(strings.Repeat("=", 42))
	fmt.Printf("%s[INFO]%s PVMSS Route Testing\n", ColorBlue, ColorReset)
	fmt.Printf("%s[INFO]%s Target: %s\n", ColorBlue, ColorReset, baseURL)
	fmt.Println(strings.Repeat("=", 42))
}

func printSummary(stats *TestStats) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 42))
	fmt.Printf("%s[INFO]%s TEST SUMMARY\n", ColorBlue, ColorReset)
	fmt.Println(strings.Repeat("=", 42))
	fmt.Printf("Total tests:  %d\n", stats.Total)
	fmt.Printf("Passed:       %s%d%s\n", ColorGreen, stats.Passed, ColorReset)
	if stats.Failed > 0 {
		fmt.Printf("Failed:       %s%d%s\n", ColorRed, stats.Failed, ColorReset)
	} else {
		fmt.Printf("Failed:       %d\n", stats.Failed)
	}
	fmt.Println(strings.Repeat("=", 42))

	if stats.Failed > 0 {
		fmt.Printf("%s[✗]%s Some tests failed. Please review the output above.\n", ColorRed, ColorReset)
	} else {
		fmt.Printf("%s[✓]%s All tests passed!\n", ColorGreen, ColorReset)
	}
}

func logSection(message string) {
	fmt.Println()
	fmt.Printf("%s[INFO]%s %s\n", ColorBlue, ColorReset, strings.Repeat("=", 56))
	fmt.Printf("%s[INFO]%s %s\n", ColorBlue, ColorReset, message)
	fmt.Printf("%s[INFO]%s %s\n", ColorBlue, ColorReset, strings.Repeat("=", 56))
}

func logInfo(message string) {
	fmt.Printf("%s[INFO]%s %s\n", ColorBlue, ColorReset, message)
}

func logSuccess(message string) {
	fmt.Printf("%s[✓]%s %s\n", ColorGreen, ColorReset, message)
}

func logError(message string) {
	fmt.Printf("%s[✗]%s %s\n", ColorRed, ColorReset, message)
}
