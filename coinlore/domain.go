package coinlore

import (
	"context"
	"unicode"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes coinlore as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/coinlore-cli/coinlore"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// coinlore:// URIs by routing to the operations Register installs.
func init() { kit.Register(Domain{}) }

// Domain is the coinlore driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "coinlore",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "coinlore",
			Short:  "A command line for CoinLore crypto data.",
			Long: `A command line for CoinLore crypto data.

coinlore reads public CoinLore API data over plain HTTPS, shapes it into
clean records, and prints output that pipes into the rest of your tools. No API
key, nothing to run alongside it.`,
			Site: "coinlore.com",
			Repo: "https://github.com/tamnd/coinlore-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "coins",
		Group:   "read",
		List:    true,
		Summary: "List coins by market rank",
	}, listCoins)

	kit.Handle(app, kit.OpMeta{
		Name:     "coin",
		Group:    "read",
		Single:   true,
		Summary:  "Fetch a single coin by numeric ID",
		URIType:  "coin",
		Resolver: true,
		Args:     []kit.Arg{{Name: "id", Help: "coin ID (numeric, e.g. 90 for Bitcoin)"}},
	}, getCoin)

	kit.Handle(app, kit.OpMeta{
		Name:    "markets",
		Group:   "read",
		List:    true,
		Summary: "List exchange markets for a coin",
		Args:    []kit.Arg{{Name: "id", Help: "coin ID"}},
	}, getMarkets)

	kit.Handle(app, kit.OpMeta{
		Name:    "global",
		Group:   "read",
		Single:  true,
		Summary: "Fetch global crypto market stats",
	}, getGlobal)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type coinsInput struct {
	Limit  int     `kit:"flag,inherit" help:"max coins" default:"25"`
	Start  int     `kit:"flag" help:"pagination start offset" default:"0"`
	Client *Client `kit:"inject"`
}

type coinInput struct {
	ID     string  `kit:"arg" help:"coin ID (numeric, e.g. 90 for Bitcoin)"`
	Client *Client `kit:"inject"`
}

type marketsInput struct {
	ID     string  `kit:"arg" help:"coin ID"`
	Limit  int     `kit:"flag,inherit" help:"max markets" default:"10"`
	Client *Client `kit:"inject"`
}

type globalInput struct {
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listCoins(ctx context.Context, in coinsInput, emit func(*Coin) error) error {
	coins, err := in.Client.ListCoins(ctx, in.Limit, in.Start)
	if err != nil {
		return mapErr(err)
	}
	for i := range coins {
		if err := emit(&coins[i]); err != nil {
			return err
		}
	}
	return nil
}

func getCoin(ctx context.Context, in coinInput, emit func(*Coin) error) error {
	coin, err := in.Client.GetCoin(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(coin)
}

func getMarkets(ctx context.Context, in marketsInput, emit func(*Market) error) error {
	markets, err := in.Client.GetMarkets(ctx, in.ID, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range markets {
		if err := emit(&markets[i]); err != nil {
			return err
		}
	}
	return nil
}

func getGlobal(ctx context.Context, in globalInput, emit func(*GlobalStats) error) error {
	stats, err := in.Client.GetGlobal(ctx)
	if err != nil {
		return mapErr(err)
	}
	return emit(stats)
}

// --- Resolver: the URI-native string functions, pure and network-free ---

// Classify turns a numeric coin ID or ticker symbol into a canonical (type, id).
// Pure numeric input → ("coin", id); letters+digits (ticker like BTC) → ("symbol", id);
// otherwise → ("query", id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty coinlore reference")
	}
	if isNumeric(input) {
		return "coin", input, nil
	}
	if isTickerSymbol(input) {
		return "symbol", input, nil
	}
	return "query", input, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "coin":
		return "https://coinlore.com/crypto/" + id, nil
	case "symbol":
		return "https://coinlore.com/crypto/", nil
	default:
		return "", errs.Usage("coinlore has no resource type %q", uriType)
	}
}

// --- helpers ---

// isNumeric returns true if every rune in s is a decimal digit.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// isTickerSymbol returns true if s looks like a ticker (letters and digits, at
// least one letter, no spaces or punctuation).
func isTickerSymbol(s string) bool {
	if s == "" {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
		} else if !unicode.IsDigit(r) {
			return false
		}
	}
	return hasLetter
}

// mapErr converts a library error into the appropriate kit error kind.
func mapErr(err error) error {
	return err
}
