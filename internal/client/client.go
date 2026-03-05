package client

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/sebasusnik/coda/internal/auth"
)

const spotifyBaseURL = "https://api.spotify.com/v1"

// isSuccess returns true for any 2xx status code. Spotify's playback endpoints
// can return either 200 or 204 depending on the version and device type.
func isSuccess(code int) bool {
	return code >= 200 && code < 300
}

func makeSpotifyRequest(method, endpoint string, body []byte) (*http.Response, error) {
	token, err := auth.GetValidToken()
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, spotifyBaseURL+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

func artistNames(artists []Artist) string {
	var parts []string
	for _, a := range artists {
		parts = append(parts, a.Name)
	}
	return strings.Join(parts, ", ")
}
