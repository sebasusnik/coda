package auth

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/sebasusnik/coda/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/spotify"
)

//go:embed callback.html
var callbackHTML string

const (
	redirectURI = "http://127.0.0.1:8080/callback"
)

// Set at build time via -ldflags
var (
	DefaultClientID     string
	DefaultClientSecret string
)

var (
	oauthConfig *oauth2.Config
	state       string
)

func init() {
	clientID := DefaultClientID
	clientSecret := DefaultClientSecret
	if ev := os.Getenv("CODA_CLIENT_ID"); ev != "" {
		clientID = ev
	}
	if ev := os.Getenv("CODA_CLIENT_SECRET"); ev != "" {
		clientSecret = ev
	}
	if clientID == "" || clientSecret == "" {
		if cfg, err := config.Load(); err == nil {
			if clientID == "" {
				clientID = cfg.ClientID
			}
			if clientSecret == "" {
				clientSecret = cfg.ClientSecret
			}
		}
	}

	oauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes: []string{
			"user-read-private",
			"user-read-email",
			"user-read-playback-state",
			"user-modify-playback-state",
			"user-read-currently-playing",
			"streaming",
			"playlist-read-private",
			"playlist-read-collaborative",
			"user-library-modify",
			"user-library-read",
		},
		Endpoint: spotify.Endpoint,
	}
}

func Authenticate(headless, force bool) error {
	// Check if already authenticated
	if !force {
		cfg, err := config.Load()
		if err == nil && cfg.AccessToken != "" && cfg.RefreshToken != "" {
			// Try to refresh token
			if err := refreshToken(); err == nil {
				fmt.Println("✓ Already authenticated")
				return nil
			}
		}
	}

	if oauthConfig.ClientID == "" {
		fmt.Print("Enter Spotify Client ID: ")
		var clientID string
		_, _ = fmt.Scanln(&clientID)
		oauthConfig.ClientID = clientID
	}

	if oauthConfig.ClientSecret == "" {
		fmt.Print("Enter Spotify Client Secret: ")
		var clientSecret string
		_, _ = fmt.Scanln(&clientSecret)
		oauthConfig.ClientSecret = clientSecret
	}

	// Generate random state
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("failed to generate state: %v", err)
	}
	state = base64.URLEncoding.EncodeToString(b)

	authURLOpts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if force {
		// show_dialog=true forces Spotify to re-present the consent screen,
		// ensuring new scopes are actually granted rather than silently skipped.
		authURLOpts = append(authURLOpts, oauth2.SetAuthURLParam("show_dialog", "true"))
	}
	authURL := oauthConfig.AuthCodeURL(state, authURLOpts...)

	if headless {
		fmt.Printf("Visit this URL to authorize the application:\n%s\n", authURL)
		fmt.Print("Enter the authorization code: ")
		var code string
		_, _ = fmt.Scanln(&code)
		return exchangeToken(code)
	}

	// Browser-based auth
	fmt.Println("Opening browser for Spotify authorization...")
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Failed to open browser. Please visit: %s\n", authURL)
	}

	// Start local server to handle callback
	return handleCallback()
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

func handleCallback() error {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{Addr: ":8080", Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		receivedState := r.URL.Query().Get("state")
		if receivedState != state {
			errChan <- fmt.Errorf("state mismatch")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(callbackHTML))

		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case code := <-codeChan:
		_ = server.Shutdown(context.Background())
		return exchangeToken(code)
	case err := <-errChan:
		_ = server.Shutdown(context.Background())
		return err
	case <-time.After(5 * time.Minute):
		_ = server.Shutdown(context.Background())
		return fmt.Errorf("authentication timeout")
	}
}

func exchangeToken(code string) error {
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("failed to exchange token: %v", err)
	}

	cfg := &config.Config{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
		ClientID:     oauthConfig.ClientID,
		ClientSecret: oauthConfig.ClientSecret,
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	fmt.Println("✓ Authentication successful!")
	return nil
}

func refreshToken() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	oauthConfig.ClientID = cfg.ClientID
	oauthConfig.ClientSecret = cfg.ClientSecret

	token := &oauth2.Token{
		RefreshToken: cfg.RefreshToken,
		Expiry:       cfg.ExpiresAt,
	}

	tokenSource := oauthConfig.TokenSource(context.Background(), token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return err
	}

	cfg.AccessToken = newToken.AccessToken
	cfg.ExpiresAt = newToken.Expiry
	if newToken.RefreshToken != "" {
		cfg.RefreshToken = newToken.RefreshToken
	}

	return cfg.Save()
}

func GetValidToken() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("not authenticated. Run 'coda auth' first")
	}

	if time.Now().After(cfg.ExpiresAt) {
		if err := refreshToken(); err != nil {
			return "", fmt.Errorf("failed to refresh token: %v", err)
		}
		cfg, _ = config.Load()
	}

	return cfg.AccessToken, nil
}
