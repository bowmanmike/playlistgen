package navidrome

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestListTracks(t *testing.T) {
	t.Run("success fetches albums and songs", func(t *testing.T) {
		call := 0
		httpClient := mockHTTPClient(func(req *http.Request) (*http.Response, error) {
			call++
			values, _ := url.ParseQuery(req.URL.RawQuery)
			if values.Get("u") != "user" || values.Get("f") != "json" {
				t.Fatalf("missing auth params: %v", values)
			}

			switch req.URL.Path {
			case "/rest/getAlbumList2.view":
				body := `{"subsonic-response":{"status":"ok","albumList2":{"album":[{"id":"alb1"}]}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			case "/rest/getAlbum.view":
				if req.URL.Query().Get("id") != "alb1" {
					t.Fatalf("unexpected album id %s", req.URL.Query().Get("id"))
				}
				body := `{"subsonic-response":{"status":"ok","album":{"song":[{"id":"1","title":"Song","artist":"Artist","album":"Album","duration":180,"path":"/music/song.mp3"}]}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			default:
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			return nil, nil
		})

		client, err := NewClient(Config{
			BaseURL:    "https://navidrome.local",
			Username:   "user",
			Password:   "pass",
			HTTPClient: httpClient,
		})
		if err != nil {
			t.Fatalf("create client: %v", err)
		}

		tracks, err := client.ListTracks(context.Background())
		if err != nil {
			t.Fatalf("ListTracks error: %v", err)
		}

		if len(tracks) != 1 || tracks[0].ID != "1" {
			t.Fatalf("unexpected tracks %+v", tracks)
		}
		if tracks[0].Duration != 180*time.Second {
			t.Fatalf("unexpected duration %v", tracks[0].Duration)
		}
		if call != 2 {
			t.Fatalf("expected two requests, got %d", call)
		}
	})

	t.Run("non-200 from album list", func(t *testing.T) {
		httpClient := mockHTTPClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("bad gateway")),
				Header:     make(http.Header),
			}, nil
		})

		client, err := NewClient(Config{
			BaseURL:    "https://navidrome.local",
			Username:   "user",
			Password:   "pass",
			HTTPClient: httpClient,
		})
		if err != nil {
			t.Fatalf("create client: %v", err)
		}

		if _, err := client.ListTracks(context.Background()); err == nil {
			t.Fatalf("expected error for non-200 response")
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func mockHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{
		Timeout:   time.Second,
		Transport: fn,
	}
}
