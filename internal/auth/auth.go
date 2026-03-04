package auth

import (
	"context"
	"crypto/rand"
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

const (
	redirectURI = "http://127.0.0.1:8080/callback"
)

var (
	oauthConfig *oauth2.Config
	state       string
)

func init() {
	clientID := os.Getenv("CODA_CLIENT_ID")
	clientSecret := os.Getenv("CODA_CLIENT_SECRET")
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
		},
		Endpoint: spotify.Endpoint,
	}
}

func Authenticate(headless bool) error {
	// Check if already authenticated
	cfg, err := config.Load()
	if err == nil && cfg.AccessToken != "" && cfg.RefreshToken != "" {
		// Try to refresh token
		if err := refreshToken(); err == nil {
			fmt.Println("✓ Already authenticated")
			return nil
		}
	}

	if oauthConfig.ClientID == "" {
		fmt.Print("Enter Spotify Client ID: ")
		var clientID string
		fmt.Scanln(&clientID)
		oauthConfig.ClientID = clientID
	}

	if oauthConfig.ClientSecret == "" {
		fmt.Print("Enter Spotify Client Secret: ")
		var clientSecret string
		fmt.Scanln(&clientSecret)
		oauthConfig.ClientSecret = clientSecret
	}

	// Generate random state
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("failed to generate state: %v", err)
	}
	state = base64.URLEncoding.EncodeToString(b)

	authURL := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	if headless {
		fmt.Printf("Visit this URL to authorize the application:\n%s\n", authURL)
		fmt.Print("Enter the authorization code: ")
		var code string
		fmt.Scanln(&code)
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
		w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>coda — connected</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }

    body {
      background: #121212;
      color: #fff;
      font-family: 'Circular', 'Helvetica Neue', Helvetica, Arial, sans-serif;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      overflow: hidden;
    }

    .glow {
      position: fixed;
      width: 600px;
      height: 600px;
      border-radius: 50%;
      background: radial-gradient(circle, rgba(29,185,84,0.12) 0%, transparent 70%);
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      pointer-events: none;
    }

    .card {
      position: relative;
      text-align: center;
      padding: 0 2rem;
      animation: fadeUp 0.6s ease both;
    }

    @keyframes fadeUp {
      from { opacity: 0; transform: translateY(20px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    .spotify-logo {
      margin-bottom: 2.5rem;
      opacity: 0.9;
    }

    .check-wrap {
      margin-bottom: 2rem;
    }

    .check-wrap svg {
      width: 56px;
      height: 56px;
      stroke: #1db954;
      stroke-width: 2;
      stroke-linecap: round;
      stroke-linejoin: round;
      fill: none;
    }

    .check-path {
      stroke-dasharray: 60;
      stroke-dashoffset: 60;
      animation: draw 0.5s ease 0.3s forwards;
    }

    @keyframes draw {
      to { stroke-dashoffset: 0; }
    }

    h1 {
      font-size: 2rem;
      font-weight: 700;
      letter-spacing: -0.03em;
      margin-bottom: 0.6rem;
    }

    .sub {
      font-size: 0.95rem;
      color: #a7a7a7;
      margin-bottom: 2.5rem;
    }

    .pill {
      display: inline-block;
      font-size: 0.75rem;
      color: #6a6a6a;
      border: 1px solid #2a2a2a;
      border-radius: 999px;
      padding: 0.4rem 1rem;
      letter-spacing: 0.02em;
    }
  </style>
</head>
<body>
  <div class="glow"></div>
  <div class="card">
    <div class="spotify-logo">
      <svg width="40" height="40" viewBox="0 0 24 24" fill="#1db954" xmlns="http://www.w3.org/2000/svg">
        <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10S17.523 2 12 2zm4.586 14.424a.623.623 0 01-.857.207c-2.348-1.435-5.304-1.76-8.785-.964a.623.623 0 01-.277-1.215c3.809-.87 7.077-.496 9.712 1.115a.623.623 0 01.207.857zm1.223-2.722a.78.78 0 01-1.072.257c-2.687-1.652-6.785-2.131-9.965-1.166a.78.78 0 01-.973-.519.781.781 0 01.52-.972c3.632-1.102 8.147-.568 11.233 1.328a.78.78 0 01.257 1.072zm.105-2.835C14.692 8.95 9.375 8.775 6.297 9.71a.937.937 0 11-.543-1.793c3.532-1.072 9.404-.865 13.115 1.338a.937.937 0 01-.955 1.612z"/>
      </svg>
    </div>

    <div class="check-wrap">
      <svg viewBox="0 0 24 24">
        <polyline class="check-path" points="20 6 9 17 4 12"/>
      </svg>
    </div>

    <h1>You're all set!</h1>
    <p class="sub">Your Spotify account is connected to coda.</p>
    <span class="pill">You can close this window</span>
  </div>
</body>
</html>
`))

		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case code := <-codeChan:
		server.Shutdown(context.Background())
		return exchangeToken(code)
	case err := <-errChan:
		server.Shutdown(context.Background())
		return err
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
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
