package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Browser User-Agent so Reddit does not block requests
const browserUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// RedditItem represents a post or comment.
type RedditItem struct {
	Name      string // Fullname e.g. "t1_xxx" or "t3_xxx"
	Kind      string // "comment" or "post"
	Subreddit string
	Content   string
	CreatedAt time.Time
}

// RedditClient manages Reddit API requests using the token_v2 session token.
type RedditClient struct {
	token    string
	username string
}

type listingResponse struct {
	Data struct {
		After    *string `json:"after"`
		Children []struct {
			Kind string `json:"kind"`
			Data struct {
				Name       string  `json:"name"`
				Subreddit  string  `json:"subreddit"`
				Title      string  `json:"title"`
				Body       string  `json:"body"`
				CreatedUTC float64 `json:"created_utc"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// NewRedditClient initializes a client with the token_v2 cookie token and auto-detects the username.
func NewRedditClient(rawToken string) (*RedditClient, error) {
	token := strings.TrimSpace(rawToken)
	token = strings.TrimPrefix(token, "token_v2=")
	token = strings.Trim(token, `"';`)

	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	client := &RedditClient{token: token}

	// 1. Attempt to extract username directly from the JWT claims
	if u := extractUsernameFromJWT(token); u != "" {
		client.username = u
	}

	// 2. If not found in JWT, query Reddit's /api/v1/me endpoint
	if client.username == "" {
		name, err := client.fetchCurrentUsername()
		if err != nil {
			return nil, fmt.Errorf("could not auto-detect username from token: %w", err)
		}
		client.username = name
	}

	return client, nil
}

// extractUsernameFromJWT decodes the claims section of token_v2 if available.
func extractUsernameFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.RawStdEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		UserName string `json:"user_name"`
	}
	if err := json.Unmarshal(payload, &claims); err == nil && claims.UserName != "" {
		return claims.UserName
	}
	return ""
}

// fetchCurrentUsername detects the username from Reddit's /api/v1/me endpoint.
func (c *RedditClient) fetchCurrentUsername() (string, error) {
	req, err := http.NewRequest("GET", "https://oauth.reddit.com/api/v1/me", nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var me struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return "", err
	}
	return me.Name, nil
}

// FetchAll retrieves all items for a given target ("comments" or "submitted").
func (c *RedditClient) FetchAll(target string) ([]RedditItem, error) {
	var items []RedditItem
	after := ""

	for {
		reqURL := fmt.Sprintf("https://oauth.reddit.com/user/%s/%s?limit=100", c.username, target)
		if after != "" {
			reqURL += "&after=" + url.QueryEscape(after)
		}

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		c.setHeaders(req)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		var listing listingResponse
		err = json.NewDecoder(resp.Body).Decode(&listing)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		for _, child := range listing.Data.Children {
			kind := "post"
			content := child.Data.Title
			if child.Kind == "t1" {
				kind = "comment"
				content = child.Data.Body
			}
			items = append(items, RedditItem{
				Name:      child.Data.Name,
				Kind:      kind,
				Subreddit: child.Data.Subreddit,
				Content:   content,
				CreatedAt: time.Unix(int64(child.Data.CreatedUTC), 0),
			})
		}

		if listing.Data.After == nil || *listing.Data.After == "" {
			break
		}
		after = *listing.Data.After
	}

	return items, nil
}

// Delete removes a post or comment by its fullname.
func (c *RedditClient) Delete(name string) error {
	form := url.Values{"id": {name}}
	req, err := http.NewRequest("POST", "https://oauth.reddit.com/api/del", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// setHeaders applies the browser User-Agent, Bearer Authorization, and cookie headers.
func (c *RedditClient) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Cookie", "token_v2="+c.token)
}
