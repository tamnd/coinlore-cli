// Package coinlore is the library behind the coinlore command line:
// the HTTP client, request shaping, and the typed data models for CoinLore.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package coinlore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultUserAgent identifies the client to CoinLore.
const DefaultUserAgent = "coinlore-cli/dev (+https://github.com/tamnd/coinlore-cli)"

// Host is the API host this client talks to.
const Host = "api.coinlore.net"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Client talks to CoinLore over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 30s timeout, a 300ms
// minimum gap between requests, and five retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Retries:   5,
	}
}

// Get fetches url and returns the response body. It paces and retries according
// to the client's settings. The caller owns nothing extra; the body is read
// fully and closed here.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- data models ---

// Coin holds the per-coin data returned by the tickers and ticker endpoints.
type Coin struct {
	ID               string  `kit:"id" json:"id"`
	Symbol           string  `json:"symbol"`
	Name             string  `json:"name"`
	Rank             int     `json:"rank"`
	PriceUSD         string  `json:"price_usd"`
	PercentChange1h  string  `json:"percent_change_1h"`
	PercentChange24h string  `json:"percent_change_24h"`
	PercentChange7d  string  `json:"percent_change_7d"`
	MarketCapUSD     string  `json:"market_cap_usd"`
	Volume24         float64 `json:"volume24"`
	CirculatingSupply string `json:"csupply"`
	TotalSupply      string  `json:"tsupply"`
}

// Market holds a single exchange market for a coin.
type Market struct {
	Name      string  `kit:"id" json:"name"`
	Base      string  `json:"base"`
	Quote     string  `json:"quote"`
	Price     float64 `json:"price"`
	PriceUSD  float64 `json:"price_usd"`
	Volume    float64 `json:"volume"`
	VolumeUSD float64 `json:"volume_usd"`
}

// GlobalStats holds the global crypto market overview.
type GlobalStats struct {
	CoinsCount    int     `kit:"id" json:"coins_count"`
	ActiveMarkets int     `json:"active_markets"`
	TotalMcap     float64 `json:"total_mcap"`
	TotalVolume   float64 `json:"total_volume"`
	BTCDominance  string  `json:"btc_d"`
	ETHDominance  string  `json:"eth_d"`
	McapChange    string  `json:"mcap_change"`
	VolumeChange  string  `json:"volume_change"`
}

// --- client methods ---

// ListCoins fetches a paginated list of coins by market rank.
// limit controls how many coins to return (max 100); start is the offset.
func (c *Client) ListCoins(ctx context.Context, limit, start int) ([]Coin, error) {
	url := fmt.Sprintf("%s/api/tickers/?limit=%d&start=%d", BaseURL, limit, start)
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Coin `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode tickers: %w", err)
	}
	return resp.Data, nil
}

// GetCoin fetches a single coin by its numeric ID (e.g. "90" for Bitcoin).
func (c *Client) GetCoin(ctx context.Context, id string) (*Coin, error) {
	url := fmt.Sprintf("%s/api/ticker/?id=%s", BaseURL, id)
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var coins []Coin
	if err := json.Unmarshal(body, &coins); err != nil {
		return nil, fmt.Errorf("decode ticker: %w", err)
	}
	if len(coins) == 0 {
		return nil, fmt.Errorf("coin %s not found", id)
	}
	return &coins[0], nil
}

// GetMarkets fetches the exchange markets for a coin by its numeric ID.
// limit truncates the result client-side; the API returns up to 50.
func (c *Client) GetMarkets(ctx context.Context, id string, limit int) ([]Market, error) {
	url := fmt.Sprintf("%s/api/coin/markets/?id=%s", BaseURL, id)
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var markets []Market
	if err := json.Unmarshal(body, &markets); err != nil {
		return nil, fmt.Errorf("decode markets: %w", err)
	}
	if limit > 0 && len(markets) > limit {
		markets = markets[:limit]
	}
	return markets, nil
}

// GetGlobal fetches the global crypto market overview.
func (c *Client) GetGlobal(ctx context.Context) (*GlobalStats, error) {
	url := BaseURL + "/api/global/"
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var stats []GlobalStats
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("decode global: %w", err)
	}
	if len(stats) == 0 {
		return nil, fmt.Errorf("empty global response")
	}
	return &stats[0], nil
}
