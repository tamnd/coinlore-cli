package coinlore

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "coinlore" {
		t.Errorf("Scheme = %q, want coinlore", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "coinlore" {
		t.Errorf("Identity.Binary = %q, want coinlore", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in, typ, id string
	}{
		{"90", "coin", "90"},
		{"12345", "coin", "12345"},
		{"BTC", "symbol", "BTC"},
		{"ETH", "symbol", "ETH"},
		{"bitcoin", "symbol", "bitcoin"},
		{"BTC2", "symbol", "BTC2"},
		{"bitcoin cash", "query", "bitcoin cash"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") expected error, got nil")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("coin", "90")
	want := "https://coinlore.com/crypto/90"
	if err != nil || got != want {
		t.Errorf("Locate(coin, 90) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateSymbol(t *testing.T) {
	got, err := Domain{}.Locate("symbol", "BTC")
	want := "https://coinlore.com/crypto/"
	if err != nil || got != want {
		t.Errorf("Locate(symbol, BTC) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate(unknown, foo) expected error, got nil")
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	coin := &Coin{ID: "90", Symbol: "BTC", Name: "Bitcoin", Rank: 1, PriceUSD: "64140.22"}
	u, err := h.Mint(coin)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "coinlore://coin/90"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("coinlore", "90")
	if err != nil || got.String() != "coinlore://coin/90" {
		t.Errorf("ResolveOn = (%q, %v), want coinlore://coin/90", got.String(), err)
	}
}

func TestIsNumeric(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"90", true},
		{"0", true},
		{"12345", true},
		{"BTC", false},
		{"", false},
		{"12a", false},
	}
	for _, tc := range cases {
		got := isNumeric(tc.s)
		if got != tc.want {
			t.Errorf("isNumeric(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestIsTickerSymbol(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"BTC", true},
		{"ETH", true},
		{"BTC2", true},
		{"bitcoin", true},
		{"90", false},
		{"", false},
		{"BTC-USD", false},
		{"BTC USD", false},
	}
	for _, tc := range cases {
		got := isTickerSymbol(tc.s)
		if got != tc.want {
			t.Errorf("isTickerSymbol(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}
