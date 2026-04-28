// Package client provides a Go client for the Nayatel Cloud API.
package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the default Nayatel Cloud API base URL.
	DefaultBaseURL = "https://cloud.nayatel.com/api"

	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second

	// tokenCacheDir is the directory name for caching tokens.
	tokenCacheDir = "nayatel"

	// tokenExpiryBuffer is the time before actual expiry when we consider token expired.
	// This ensures we refresh tokens before they actually expire.
	tokenExpiryBuffer = 5 * time.Minute
)

// cachedToken represents a cached JWT token with metadata.
type cachedToken struct {
	Token     string `json:"token"`
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expires_at"`
}

// getTokenCachePath returns the path to the token cache file for a given username.
func getTokenCachePath(username string) (string, error) {
	// Use XDG_CONFIG_HOME if set, otherwise ~/.config
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config")
	}

	cacheDir := filepath.Join(configDir, tokenCacheDir)
	cacheName := url.PathEscape(username)
	return filepath.Join(cacheDir, fmt.Sprintf("token_%s.json", cacheName)), nil
}

// parseJWTExpiry extracts the expiration time from a JWT token without verifying the signature.
func parseJWTExpiry(token string) (time.Time, error) {
	// JWT format: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part)
	payload := parts[1]
	// Add padding if needed for base64 decoding
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// Try standard encoding
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to decode JWT payload: %w", err)
		}
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("no expiration claim in JWT")
	}

	return time.Unix(claims.Exp, 0), nil
}

// isTokenValid checks if a token is still valid (not expired).
func isTokenValid(token string) bool {
	expiry, err := parseJWTExpiry(token)
	if err != nil {
		return false
	}
	// Consider token expired if it's within the buffer time of expiry
	return time.Now().Add(tokenExpiryBuffer).Before(expiry)
}

// loadCachedToken attempts to load a valid cached token for the given username.
func loadCachedToken(username string) (string, error) {
	cachePath, err := getTokenCachePath(username)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No cache file, not an error
		}
		return "", fmt.Errorf("failed to read token cache: %w", err)
	}

	var cached cachedToken
	if unmarshalErr := json.Unmarshal(data, &cached); unmarshalErr != nil {
		// Invalid cache file, delete it.
		os.Remove(cachePath)
		return "", fmt.Errorf("failed to parse token cache: %w", unmarshalErr)
	}

	// Verify the cached token is for the right user and still valid
	if cached.Username != username {
		return "", nil
	}

	if !isTokenValid(cached.Token) {
		// Token expired, delete cache
		os.Remove(cachePath)
		return "", nil
	}

	return cached.Token, nil
}

// saveCachedToken saves a token to the cache file.
func saveCachedToken(username, token string) error {
	cachePath, err := getTokenCachePath(username)
	if err != nil {
		return err
	}

	// Create cache directory if it doesn't exist
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Get expiry time from token
	expiresAt, err := parseJWTExpiry(token)
	if err != nil {
		// If we can't parse expiry, still cache but with 0 expiry (will check token validity on load)
		expiresAt = time.Time{}
	}

	cached := cachedToken{
		Token:     token,
		Username:  username,
		ExpiresAt: expiresAt.Unix(),
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token cache: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token cache: %w", err)
	}

	return nil
}

// Client is the Nayatel Cloud API client.
type Client struct {
	// BaseURL is the API base URL.
	BaseURL string

	// Username is the Nayatel Cloud username.
	Username string

	// Token is the JWT authentication token.
	Token string

	// ProjectID is the default project ID.
	ProjectID string

	// HTTPClient is the underlying HTTP client.
	HTTPClient *http.Client

	// csrfMu protects csrfToken across Terraform's parallel operations.
	csrfMu sync.Mutex

	// csrfToken is the cached CSRF token for the current HTTP session.
	csrfToken string

	// Instances provides instance operations.
	Instances *InstanceService

	// Networks provides network operations.
	Networks *NetworkService

	// Routers provides router operations.
	Routers *RouterService

	// FloatingIPs provides floating IP operations.
	FloatingIPs *FloatingIPService

	// SecurityGroups provides security group operations.
	SecurityGroups *SecurityGroupService

	// Images provides image operations.
	Images *ImageService

	// Flavors provides flavor operations.
	Flavors *FlavorService

	// SSHKeys provides SSH key operations.
	SSHKeys *SSHKeyService

	// Projects provides project operations.
	Projects *ProjectService

	// Volumes provides volume operations.
	Volumes *VolumeService
}

// ClientOption is a function that configures the client.
type ClientOption func(*Client)

// WithBaseURL sets the API base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.HTTPClient.Timeout = timeout
	}
}

// WithProjectID sets the default project ID.
func WithProjectID(projectID string) ClientOption {
	return func(c *Client) {
		c.ProjectID = projectID
	}
}

// ensureCookieJar installs a cookie jar on the HTTP client if one is not already set.
func ensureCookieJar(httpClient *http.Client) error {
	if httpClient == nil {
		return fmt.Errorf("http client is nil")
	}

	if httpClient.Jar != nil {
		return nil
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("failed to create cookie jar: %w", err)
	}

	httpClient.Jar = jar
	return nil
}

// NewClient creates a new Nayatel Cloud API client.
func NewClient(username, token string, opts ...ClientOption) *Client {
	c := &Client{
		BaseURL:  DefaultBaseURL,
		Username: username,
		Token:    token,
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: DefaultTimeout}
	}
	_ = ensureCookieJar(c.HTTPClient)

	// Initialize services
	c.Instances = &InstanceService{client: c}
	c.Networks = &NetworkService{client: c}
	c.Routers = &RouterService{client: c}
	c.FloatingIPs = &FloatingIPService{client: c}
	c.SecurityGroups = &SecurityGroupService{client: c}
	c.Images = &ImageService{client: c}
	c.Flavors = &FlavorService{client: c}
	c.SSHKeys = &SSHKeyService{client: c}
	c.Projects = &ProjectService{client: c}
	c.Volumes = &VolumeService{client: c}

	return c
}

// NewClientWithLogin creates a new client by logging in with username and password.
// It will use a cached token if available and still valid, otherwise authenticate
// and cache the new token for future use.
func NewClientWithLogin(ctx context.Context, username, password string, opts ...ClientOption) (*Client, error) {
	// Try to load cached token first
	cachedToken, err := loadCachedToken(username)
	if err != nil {
		// Ignore cache read errors and continue with fresh authentication.
		cachedToken = ""
	}

	if cachedToken != "" {
		// Use cached token
		return NewClient(username, cachedToken, opts...), nil
	}

	// No valid cached token, need to authenticate with the same HTTP client/jar
	// that will be used for subsequent API requests.
	c := NewClient(username, "", opts...)
	token, err := authenticate(ctx, c.HTTPClient, c.BaseURL, username, password)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Cache the new token (ignore errors - caching is best effort)
	_ = saveCachedToken(username, token)

	c.Token = token
	return c, nil
}

// Request makes an HTTP request to the API.
func (c *Client) Request(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, endpoint)

	var bodyBytes []byte
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyBytes = jsonBody
	}

	if err := c.ensureCSRFToken(ctx, false); err != nil {
		return nil, fmt.Errorf("failed to fetch CSRF token: %w", err)
	}

	doRequest := func() ([]byte, int, error) {
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-CSRF-Token", c.currentCSRFToken())

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read response body: %w", err)
		}

		return respBody, resp.StatusCode, nil
	}

	respBody, statusCode, err := doRequest()
	if err != nil {
		return nil, err
	}

	if shouldRetryCSRF(statusCode, respBody) {
		if err := c.ensureCSRFToken(ctx, true); err != nil {
			return nil, fmt.Errorf("failed to refresh CSRF token: %w", err)
		}

		respBody, statusCode, err = doRequest()
		if err != nil {
			return nil, err
		}
	}

	if statusCode < 200 || statusCode >= 300 {
		return nil, &APIError{
			StatusCode: statusCode,
			Message:    string(respBody),
		}
	}

	return respBody, nil
}

func (c *Client) currentCSRFToken() string {
	c.csrfMu.Lock()
	defer c.csrfMu.Unlock()

	return c.csrfToken
}

func (c *Client) ensureCSRFToken(ctx context.Context, force bool) error {
	c.csrfMu.Lock()
	defer c.csrfMu.Unlock()

	if !force && c.csrfToken != "" {
		return nil
	}

	token, err := fetchCSRFToken(ctx, c.HTTPClient, c.BaseURL)
	if err != nil {
		return err
	}

	c.csrfToken = token
	return nil
}

func fetchCSRFToken(ctx context.Context, httpClient *http.Client, baseURL string) (string, error) {
	if err := ensureCookieJar(httpClient); err != nil {
		return "", err
	}

	csrfURL := fmt.Sprintf("%s/csrf-token", strings.TrimRight(baseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, csrfURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create CSRF request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("CSRF request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if message := apiErrorMessage(body); message != "" {
			return "", fmt.Errorf("CSRF request failed (status %d): %s", resp.StatusCode, message)
		}
		return "", fmt.Errorf("CSRF request failed (status %d)", resp.StatusCode)
	}

	token := resp.Header.Get("X-CSRF-Token")
	if token == "" {
		for _, cookie := range resp.Cookies() {
			if cookie.Name == "csrf-token" {
				token = cookie.Value
				break
			}
		}
	}

	if token == "" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			return "", fmt.Errorf("failed to read CSRF response body: %w", err)
		}
		token = csrfTokenFromJSON(body)
	}

	if token == "" {
		return "", fmt.Errorf("no CSRF token in response")
	}

	return token, nil
}

func csrfTokenFromJSON(respBody []byte) string {
	var payload struct {
		Token     string `json:"token"`
		CSRFToken string `json:"csrf_token"`
		CSRFCamel string `json:"csrfToken"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return ""
	}

	switch {
	case payload.Token != "":
		return payload.Token
	case payload.CSRFToken != "":
		return payload.CSRFToken
	case payload.CSRFCamel != "":
		return payload.CSRFCamel
	default:
		return ""
	}
}

func shouldRetryCSRF(statusCode int, respBody []byte) bool {
	if statusCode == http.StatusForbidden && isCSRFError(respBody) {
		return true
	}

	return statusCode == 419
}

func isCSRFError(respBody []byte) bool {
	body := strings.ToLower(string(respBody))
	return strings.Contains(body, "csrf") ||
		strings.Contains(body, "cross-site request forgery") ||
		strings.Contains(body, "token mismatch") ||
		strings.Contains(body, "page expired")
}

// Get makes a GET request.
func (c *Client) Get(ctx context.Context, endpoint string) ([]byte, error) {
	return c.Request(ctx, http.MethodGet, endpoint, nil)
}

// Post makes a POST request.
func (c *Client) Post(ctx context.Context, endpoint string, body interface{}) ([]byte, error) {
	return c.Request(ctx, http.MethodPost, endpoint, body)
}

// Delete makes a DELETE request.
func (c *Client) Delete(ctx context.Context, endpoint string, body interface{}) ([]byte, error) {
	return c.Request(ctx, http.MethodDelete, endpoint, body)
}

// GetProjectID returns the project ID, fetching it if not set.
func (c *Client) GetProjectID(ctx context.Context) (string, error) {
	if c.ProjectID != "" {
		return c.ProjectID, nil
	}

	projects, err := c.Projects.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		return "", fmt.Errorf("no projects found")
	}

	c.ProjectID = projects[0].GetID()
	return c.ProjectID, nil
}

// APIError represents an API error response.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error is a 404 not found error.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 404
	}
	return false
}

// IsUnauthorized returns true if the error is a 401 unauthorized error.
func IsUnauthorized(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 401
	}
	return false
}

// IsInsufficientBalance returns true if the error is a 402 payment required / insufficient balance error.
func IsInsufficientBalance(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 402
	}
	return false
}

// BalanceInfo represents the user's account balance.
type BalanceInfo struct {
	Balance         float64 `json:"balance"`
	PendingCharges  float64 `json:"pending_charges"`
	AvailableCredit float64 `json:"available_credit"`
}

// GetBalance returns the user's account balance.
func (c *Client) GetBalance(ctx context.Context) (*BalanceInfo, error) {
	resp, err := c.Get(ctx, fmt.Sprintf("/user/%s/balance", c.Username))
	if err != nil {
		return nil, err
	}

	var result struct {
		Status  bool    `json:"status"`
		Balance float64 `json:"balance"`
		Data    struct {
			Balance         float64 `json:"balance"`
			PendingCharges  float64 `json:"pending_charges"`
			AvailableCredit float64 `json:"available_credit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode balance response: %w", err)
	}

	// Handle different API response formats
	info := &BalanceInfo{}
	if result.Data.Balance > 0 {
		info.Balance = result.Data.Balance
		info.PendingCharges = result.Data.PendingCharges
		info.AvailableCredit = result.Data.AvailableCredit
	} else {
		info.Balance = result.Balance
	}

	return info, nil
}

// VerifyBalance checks if account has sufficient balance for a given cost.
// It retries on 0 balance (handles Nayatel API glitch) and returns an error if insufficient.
// This is a common safety check used by all billable resource allocations.
func (c *Client) VerifyBalance(ctx context.Context, requiredAmount float64, resourceType string) error {
	if requiredAmount <= 0 {
		return nil // No cost verification needed
	}

	maxChecks := 3
	var lastBalance *BalanceInfo

	for check := 1; check <= maxChecks; check++ {
		balance, err := c.GetBalance(ctx)
		if err != nil {
			return fmt.Errorf("unable to verify account balance for %s (aborting to protect against unwanted charges): %w", resourceType, err)
		}
		lastBalance = balance

		effectiveBalance := balance.Balance + balance.AvailableCredit
		if effectiveBalance >= requiredAmount {
			return nil // Balance is sufficient
		}

		// If balance shows 0, it might be a glitch - wait and retry
		if balance.Balance == 0 && check < maxChecks {
			time.Sleep(time.Duration(check*3) * time.Second)
			continue
		}

		// Insufficient balance after retries
		return fmt.Errorf("insufficient balance for %s: required Rs. %.2f, available Rs. %.2f. "+
			"If you believe you have sufficient balance, this may be a temporary Nayatel API issue. "+
			"Please verify at https://cloud.nayatel.com/billing", resourceType, requiredAmount, effectiveBalance)
	}

	_ = lastBalance
	return nil
}

func apiErrorMessage(respBody []byte) string {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return ""
	}

	message := payload.Message
	if message == "" {
		message = payload.Error
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	if len(message) > 200 {
		message = message[:200] + "..."
	}
	return message
}

// authenticate performs the Nayatel portal authentication flow.
func authenticate(ctx context.Context, httpClient *http.Client, baseURL, username, password string) (string, error) {
	csrfToken, err := fetchCSRFToken(ctx, httpClient, baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to get CSRF token: %w", err)
	}

	authURL := fmt.Sprintf("%s/authenticate", strings.TrimRight(baseURL, "/"))

	data := url.Values{}
	data.Set("userid", username)
	data.Set("password", password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if message := apiErrorMessage(body); message != "" {
			return "", fmt.Errorf("authentication failed (status %d): %s", resp.StatusCode, message)
		}
		return "", fmt.Errorf("authentication failed (status %d)", resp.StatusCode)
	}

	var authResp struct {
		Message string `json:"message"`
		Token   string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.Token == "" {
		if authResp.Message != "" {
			return "", fmt.Errorf("authentication failed: %s", authResp.Message)
		}
		return "", fmt.Errorf("no token in auth response")
	}

	return authResp.Token, nil
}
