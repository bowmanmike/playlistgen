package navidrome

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/bowmanmike/playlistgen/internal/app"
)

const (
	defaultTimeout = 30 * time.Second
	tracksEndpoint = "/api/library/tracks"
	apiVersion     = "1.16.1"
	clientName     = "playlistgen"
)

// Config drives Client construction.
type Config struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// Client proxies requests to the Navidrome API.
type Client struct {
	baseURL    *url.URL
	username   string
	password   string
	httpClient *http.Client
}

// NewClient builds a Navidrome API client.
func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("base URL is required")
	}

	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}

	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{
		baseURL:    parsed,
		username:   cfg.Username,
		password:   cfg.Password,
		httpClient: cfg.HTTPClient,
	}, nil
}

// ListTracks fetches the track list from Navidrome.
func (c *Client) ListTracks(ctx context.Context) ([]app.Track, error) {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, tracksEndpoint)

	if c.username != "" {
		params := authParams(c.username, c.password)
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request tracks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Tracks []trackPayload `json:"tracks"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode tracks: %w", err)
	}

	tracks := make([]app.Track, 0, len(payload.Tracks))
	for _, t := range payload.Tracks {
		tracks = append(tracks, app.Track{
			ID:       t.ID,
			Title:    t.Title,
			Artist:   t.Artist,
			Album:    t.Album,
			Duration: time.Duration(t.DurationSeconds * float64(time.Second)),
			Path:     t.Path,
		})
	}

	return tracks, nil
}

type trackPayload struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Artist          string  `json:"artist"`
	Album           string  `json:"album"`
	DurationSeconds float64 `json:"duration"`
	Path            string  `json:"path"`
}

func authParams(user, password string) url.Values {
	v := url.Values{}
	if strings.TrimSpace(user) == "" {
		return v
	}

	salt := randomSalt(16)
	hash := sha1.Sum([]byte(password + salt))

	v.Set("u", user)
	v.Set("t", hex.EncodeToString(hash[:]))
	v.Set("s", salt)
	v.Set("v", apiVersion)
	v.Set("c", clientName)
	v.Set("f", "json")

	return v
}

func randomSalt(n int) string {
	if n <= 0 {
		n = 16
	}

	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("rand.Read: %v", err))
	}

	return hex.EncodeToString(buf)
}
