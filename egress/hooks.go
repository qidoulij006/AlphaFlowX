package egress

import (
	"fmt"
	"net/http"
	"net/url"
	"nofx/hook"
	"nofx/logger"
	"nofx/security"
	"nofx/store"

	"github.com/adshao/go-binance/v2/futures"
)

var exchangeStore *store.ExchangeStore

// InitHooks registers runtime hooks that attach per-exchange outbound proxy settings.
func InitHooks(st *store.Store) {
	if st == nil {
		return
	}

	exchangeStore = st.Exchange()

	hook.RegisterHook(hook.NEW_BINANCE_TRADER, func(args ...any) any {
		if len(args) < 3 {
			return &hook.NewBinanceTraderResult{Err: fmt.Errorf("invalid NEW_BINANCE_TRADER args")}
		}

		exchangeID, _ := args[1].(string)
		client, ok := args[2].(*futures.Client)
		if !ok {
			return &hook.NewBinanceTraderResult{Err: fmt.Errorf("invalid Binance client type")}
		}

		if exchangeID == "" {
			return &hook.NewBinanceTraderResult{Client: client}
		}

		proxyURL, err := getExchangeProxyURL(exchangeID)
		if err != nil {
			return &hook.NewBinanceTraderResult{Err: err, Client: client}
		}
		if proxyURL == "" {
			return &hook.NewBinanceTraderResult{Client: client}
		}

		if client.HTTPClient == nil {
			client.HTTPClient = &http.Client{}
		}
		httpClient, err := withProxy(client.HTTPClient, proxyURL)
		if err != nil {
			return &hook.NewBinanceTraderResult{Err: err, Client: client}
		}
		client.HTTPClient = httpClient
		logger.Infof("🌐 Binance trader using proxy for exchange %s", exchangeID)

		return &hook.NewBinanceTraderResult{Client: client}
	})

	hook.RegisterHook(hook.NEW_ASTER_TRADER, func(args ...any) any {
		if len(args) < 3 {
			return &hook.NewAsterTraderResult{Err: fmt.Errorf("invalid NEW_ASTER_TRADER args")}
		}

		exchangeID, _ := args[1].(string)
		client, ok := args[2].(*http.Client)
		if !ok {
			return &hook.NewAsterTraderResult{Err: fmt.Errorf("invalid Aster client type")}
		}

		proxyClient, err := applyProxyForExchange(exchangeID, client)
		if err != nil {
			return &hook.NewAsterTraderResult{Err: err, Client: client}
		}
		return &hook.NewAsterTraderResult{Client: proxyClient}
	})

	hook.RegisterHook(hook.SET_HTTP_CLIENT, func(args ...any) any {
		if len(args) < 2 {
			return &hook.SetHttpClientResult{Err: fmt.Errorf("invalid SET_HTTP_CLIENT args")}
		}

		exchangeID, _ := args[0].(string)
		client, ok := args[1].(*http.Client)
		if !ok {
			return &hook.SetHttpClientResult{Err: fmt.Errorf("invalid HTTP client type")}
		}

		proxyClient, err := applyProxyForExchange(exchangeID, client)
		if err != nil {
			return &hook.SetHttpClientResult{Err: err, Client: client}
		}
		return &hook.SetHttpClientResult{Client: proxyClient}
	})
}

func applyProxyForExchange(exchangeID string, client *http.Client) (*http.Client, error) {
	if exchangeID == "" {
		return client, nil
	}

	proxyURL, err := getExchangeProxyURL(exchangeID)
	if err != nil {
		return client, err
	}
	if proxyURL == "" {
		return client, nil
	}

	proxyClient, err := withProxy(client, proxyURL)
	if err != nil {
		logger.Warnf("⚠️  Ignoring unsafe proxy for exchange %s: %v", exchangeID, err)
		return client, nil
	}
	logger.Infof("🌐 HTTP client using proxy for exchange %s", exchangeID)
	return proxyClient, nil
}

func getExchangeProxyURL(exchangeID string) (string, error) {
	if exchangeStore == nil || exchangeID == "" {
		return "", nil
	}

	proxyURL, err := exchangeStore.GetProxyURLForExchange(exchangeID)
	if err != nil {
		return "", fmt.Errorf("failed to load exchange proxy for %s: %w", exchangeID, err)
	}
	return proxyURL, nil
}

func withProxy(client *http.Client, rawProxyURL string) (*http.Client, error) {
	if err := security.ValidateProxyURL(rawProxyURL); err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(rawProxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	clonedClient := *client
	clonedClient.Transport = cloneTransport(client.Transport)
	clonedClient.Transport.(*http.Transport).Proxy = http.ProxyURL(parsedURL)

	return &clonedClient, nil
}

func cloneTransport(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		return http.DefaultTransport.(*http.Transport).Clone()
	}

	transport, ok := rt.(*http.Transport)
	if !ok {
		return http.DefaultTransport.(*http.Transport).Clone()
	}

	return transport.Clone()
}
