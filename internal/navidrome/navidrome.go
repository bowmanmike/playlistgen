package navidrome

import (
	"context"
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
	authHeader     = "Authorization"
)

// Config drives Client construction.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Client proxies requests to the Navidrome API.
type Client struct {
	baseURL    *url.URL
	apiKey     string
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
		apiKey:     cfg.APIKey,
		httpClient: cfg.HTTPClient,
	}, nil
}

// ListTracks fetches the track list from Navidrome.
func (c *Client) ListTracks(ctx context.Context) ([]app.Track, error) {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, tracksEndpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set(authHeader, "Bearer "+c.apiKey)
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
