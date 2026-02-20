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
	"strconv"
	"strings"
	"time"

	"github.com/bowmanmike/playlistgen/internal/app"
)

const (
	defaultTimeout    = 30 * time.Second
	albumListEndpoint = "rest/getAlbumList2.view"
	albumEndpoint     = "rest/getAlbum.view"
	apiVersion        = "1.16.1"
	clientName        = "playlistgen"
	albumPageSize     = 200
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

// ListTracks fetches the track list from Navidrome via Subsonic API.
func (c *Client) ListTracks(ctx context.Context) ([]app.Track, error) {
	var (
		tracks []app.Track
		offset int
	)

	for {
		albums, err := c.fetchAlbumPage(ctx, offset)
		if err != nil {
			return nil, err
		}
		if len(albums) == 0 {
			break
		}

		for _, album := range albums {
			songs, err := c.fetchAlbumSongs(ctx, album.ID)
			if err != nil {
				return nil, err
			}
			tracks = append(tracks, songs...)
		}

		if len(albums) < albumPageSize {
			break
		}
		offset += len(albums)
	}

	return tracks, nil
}

func (c *Client) fetchAlbumPage(ctx context.Context, offset int) ([]albumItem, error) {
	params := url.Values{}
	params.Set("type", "alphabeticalByName")
	params.Set("size", strconv.Itoa(albumPageSize))
	params.Set("offset", strconv.Itoa(offset))

	var resp albumListResponse
	if err := c.doRequest(ctx, albumListEndpoint, params, &resp); err != nil {
		return nil, err
	}

	if err := resp.Response.validate(); err != nil {
		return nil, err
	}

	return resp.Response.AlbumList.Albums, nil
}

func (c *Client) fetchAlbumSongs(ctx context.Context, albumID string) ([]app.Track, error) {
	params := url.Values{}
	params.Set("id", albumID)

	var resp albumResponse
	if err := c.doRequest(ctx, albumEndpoint, params, &resp); err != nil {
		return nil, err
	}

	if err := resp.Response.validate(); err != nil {
		return nil, err
	}

	songs := make([]app.Track, 0, len(resp.Response.Album.Songs))
	for _, song := range resp.Response.Album.Songs {
		songs = append(songs, app.Track{
			ID:       song.ID,
			Title:    song.Title,
			Artist:   song.Artist,
			Album:    song.Album,
			Duration: time.Duration(song.Duration) * time.Second,
			Path:     song.Path,
		})
	}

	return songs, nil
}

func (c *Client) doRequest(ctx context.Context, endpoint string, params url.Values, target interface{}) error {
	u := *c.baseURL
	u.Path = ensureLeadingSlash(path.Join(c.baseURL.Path, endpoint))

	query := authParams(c.username, c.password)
	for key, values := range params {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
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

func ensureLeadingSlash(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

type albumListResponse struct {
	Response albumListPayload `json:"subsonic-response"`
}

type albumListPayload struct {
	subsonicEnvelope
	AlbumList struct {
		Albums []albumItem `json:"album"`
	} `json:"albumList2"`
}

type albumResponse struct {
	Response albumPayload `json:"subsonic-response"`
}

type albumPayload struct {
	subsonicEnvelope
	Album struct {
		Songs []songItem `json:"song"`
	} `json:"album"`
}

type subsonicEnvelope struct {
	Status string         `json:"status"`
	Error  *subsonicError `json:"error"`
}

func (e subsonicEnvelope) validate() error {
	if e.Error != nil {
		return fmt.Errorf("subsonic error %d: %s", e.Error.Code, e.Error.Message)
	}
	if strings.ToLower(e.Status) != "ok" {
		return fmt.Errorf("subsonic status %s", e.Status)
	}
	return nil
}

type subsonicError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type albumItem struct {
	ID string `json:"id"`
}

type songItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration"`
	Path     string `json:"path"`
}
