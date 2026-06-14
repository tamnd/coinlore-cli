package coinlore_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/coinlore-cli/coinlore"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := coinlore.NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := coinlore.NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestListCoins(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tickers/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		limit := r.URL.Query().Get("limit")
		if limit != "5" {
			t.Errorf("limit = %q, want 5", limit)
		}
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "90", "symbol": "BTC", "name": "Bitcoin", "rank": 1,
					"price_usd": "64140.22", "percent_change_1h": "0.05",
					"percent_change_24h": "1.25", "percent_change_7d": "-2.31",
					"market_cap_usd": "1267234567890", "volume24": 28000000000.0,
					"csupply": "19700000", "tsupply": "21000000"},
				{"id": "80", "symbol": "ETH", "name": "Ethereum", "rank": 2,
					"price_usd": "3100.00", "percent_change_1h": "0.10",
					"percent_change_24h": "2.00", "percent_change_7d": "5.00",
					"market_cap_usd": "372000000000", "volume24": 15000000000.0,
					"csupply": "120000000", "tsupply": "120000000"},
			},
			"info": map[string]any{"coins_num": 14471, "time": 1781455505},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := coinlore.NewClient()
	c.Rate = 0
	// Override BaseURL by pointing HTTP client to test server
	c.HTTP = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &prefixTransport{prefix: srv.URL, base: http.DefaultTransport},
	}

	coins, err := c.ListCoins(context.Background(), 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(coins) != 2 {
		t.Fatalf("got %d coins, want 2", len(coins))
	}
	if coins[0].Symbol != "BTC" {
		t.Errorf("coins[0].Symbol = %q, want BTC", coins[0].Symbol)
	}
	if coins[0].ID != "90" {
		t.Errorf("coins[0].ID = %q, want 90", coins[0].ID)
	}
	if coins[1].Name != "Ethereum" {
		t.Errorf("coins[1].Name = %q, want Ethereum", coins[1].Name)
	}
}

func TestGetCoin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ticker/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		id := r.URL.Query().Get("id")
		if id != "90" {
			t.Errorf("id = %q, want 90", id)
		}
		resp := []map[string]any{
			{"id": "90", "symbol": "BTC", "name": "Bitcoin", "rank": 1,
				"price_usd": "64140.22", "percent_change_1h": "0.05",
				"percent_change_24h": "1.25", "percent_change_7d": "-2.31",
				"market_cap_usd": "1267234567890", "volume24": 28000000000.0,
				"csupply": "19700000", "tsupply": "21000000"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := coinlore.NewClient()
	c.Rate = 0
	c.HTTP = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &prefixTransport{prefix: srv.URL, base: http.DefaultTransport},
	}

	coin, err := c.GetCoin(context.Background(), "90")
	if err != nil {
		t.Fatal(err)
	}
	if coin == nil {
		t.Fatal("got nil coin")
	}
	if coin.Symbol != "BTC" {
		t.Errorf("Symbol = %q, want BTC", coin.Symbol)
	}
	if coin.Rank != 1 {
		t.Errorf("Rank = %d, want 1", coin.Rank)
	}
}

func TestGetMarkets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/coin/markets/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := []map[string]any{
			{"name": "Binance", "base": "BTC", "quote": "USDT",
				"price": 64151.2, "price_usd": 64151.2, "volume": 46764, "volume_usd": 2999966716.8},
			{"name": "Coinbase", "base": "BTC", "quote": "USD",
				"price": 64145.0, "price_usd": 64145.0, "volume": 12000, "volume_usd": 769740000.0},
			{"name": "Kraken", "base": "BTC", "quote": "USD",
				"price": 64150.0, "price_usd": 64150.0, "volume": 8000, "volume_usd": 513200000.0},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := coinlore.NewClient()
	c.Rate = 0
	c.HTTP = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &prefixTransport{prefix: srv.URL, base: http.DefaultTransport},
	}

	markets, err := c.GetMarkets(context.Background(), "90", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(markets) != 2 {
		t.Fatalf("got %d markets after limit=2, want 2", len(markets))
	}
	if markets[0].Name != "Binance" {
		t.Errorf("markets[0].Name = %q, want Binance", markets[0].Name)
	}
}

func TestGetGlobal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/global/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := []map[string]any{
			{"coins_count": 14471, "active_markets": 32000,
				"total_mcap": 2.4e12, "total_volume": 8.5e10,
				"btc_d": "52.50", "eth_d": "15.30",
				"mcap_change": "0.79", "volume_change": "-3.21"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := coinlore.NewClient()
	c.Rate = 0
	c.HTTP = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &prefixTransport{prefix: srv.URL, base: http.DefaultTransport},
	}

	stats, err := c.GetGlobal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats == nil {
		t.Fatal("got nil stats")
	}
	if stats.CoinsCount != 14471 {
		t.Errorf("CoinsCount = %d, want 14471", stats.CoinsCount)
	}
	if stats.BTCDominance != "52.50" {
		t.Errorf("BTCDominance = %q, want 52.50", stats.BTCDominance)
	}
	if stats.ActiveMarkets != 32000 {
		t.Errorf("ActiveMarkets = %d, want 32000", stats.ActiveMarkets)
	}
}

func TestGetCoinNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API returns empty array for unknown IDs
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := coinlore.NewClient()
	c.Rate = 0
	c.HTTP = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &prefixTransport{prefix: srv.URL, base: http.DefaultTransport},
	}

	_, err := c.GetCoin(context.Background(), "99999999")
	if err == nil {
		t.Error("expected error for unknown coin ID, got nil")
	}
}

func TestGetMarkets_NoLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []map[string]any{
			{"name": "Exchange1", "base": "BTC", "quote": "USDT", "price": 64000.0, "price_usd": 64000.0},
			{"name": "Exchange2", "base": "BTC", "quote": "USD", "price": 64001.0, "price_usd": 64001.0},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := coinlore.NewClient()
	c.Rate = 0
	c.HTTP = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &prefixTransport{prefix: srv.URL, base: http.DefaultTransport},
	}

	// limit=0 means no truncation
	markets, err := c.GetMarkets(context.Background(), "90", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(markets) != 2 {
		t.Errorf("got %d markets, want 2", len(markets))
	}
}

// prefixTransport rewrites the host of every request to point to a test server,
// keeping the path and query intact so handler assertions can check them.
type prefixTransport struct {
	prefix string
	base   http.RoundTripper
}

func (p *prefixTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = r.URL.Host
	// Replace the host with the test server
	r2.URL.Host = ""
	full := p.prefix + r.URL.RequestURI()
	parsed, _ := r2.URL.Parse(full)
	r2.URL = parsed
	return p.base.RoundTrip(r2)
}
