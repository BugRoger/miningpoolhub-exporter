package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	NAMESPACE = "miningpoolhub"
	VERSION   = "latest"
)

var (
	infoGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   NAMESPACE,
			Name:        "info",
			Help:        "Info about this exporter",
			ConstLabels: prometheus.Labels{"version": VERSION},
		},
	)

	balanceGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: NAMESPACE,
			Name:      "balance",
			Help:      "Balances by coin, wallet and confirmation status",
		},
		[]string{"coin", "symbol", "wallet", "status"},
	)

	balanceConvertedGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: NAMESPACE,
			Name:      "balance_converted",
			Help:      "Balances by coin, wallet and confirmation status",
		},
		[]string{"coin", "symbol", "wallet", "status"},
	)

	SYMBOLS = map[string]string{
		"adzcoin":             "ADZ",
		"auroracoin":          "AUR",
		"bitcoin":             "BTC",
		"bitcoin-cash":        "BCH",
		"bitcoin-gold":        "BTG",
		"dash":                "DSH",
		"digibyte":            "DGB",
		"digibyte-groestl":    "DGB",
		"digibyte-skein":      "DGB",
		"digibyte-qubit":      "DGB",
		"ethereum":            "ETH",
		"ethereum-classic":    "ETC",
		"expanse":             "EXP",
		"feathercoin":         "FTC",
		"gamecredits":         "GAME",
		"geocoin":             "GEO",
		"globalboosty":        "BSTY",
		"groestlcoin":         "GRS",
		"litecoin":            "LTC",
		"maxcoin":             "MAX",
		"monacoin":            "MONA",
		"monero":              "XMR",
		"musicoin":            "MUSIC",
		"myriadcoin":          "XMY",
		"myriadcoin-skein":    "XMY",
		"myriadcoin-groestl":  "XMY",
		"myriadcoin-yescrypt": "XMY",
		"sexcoin":             "SXC",
		"siacoin":             "SC",
		"startcoin":           "START",
		"verge":               "XVG",
		"vertcoin":            "VTC",
		"zcash":               "ZEC",
		"zclassic":            "ZCL",
		"zcoin":               "XZC",
		"zencash":             "ZEN",
	}
)

type Exporter struct {
	APIKey           string
	FiatCurrency     string
	MiningPoolHubURL *url.URL
}

type GetUserAllBalances struct {
	Data Data `json:"getuserallbalances"`
}

type Data struct {
	Version  string    `json:"version"`
	Runtime  float64   `json:"runtime"`
	Balances []Balance `json:"data"`
}

type Balance struct {
	Coin          string  `json:"coin"`
	Confirmed     float64 `json:"confirmed"`
	Unconfirmed   float64 `json:"unconfirmed"`
	AEConfirmed   float64 `json:"ae_confirmed"`
	AEUnconfirmed float64 `json:"ae_unconfirmed"`
	Exchange      float64 `json:"exchange"`
}

type Prices map[string]float64

func main() {
	var (
		listenAddress = flag.String("web.listen-address", ":9401", "Address to listen on for web interface and telemetry.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		mphURL        = flag.String("url", "https://miningpoolhub.com", "Overwrite base URL for MiningPoolHub")
	)
	flag.Parse()

	url, err := url.Parse(*mphURL)
	if err != nil {
		log.Fatalf("%v is not a valid URL", *mphURL)
	}

	http.HandleFunc(*metricsPath, func(w http.ResponseWriter, r *http.Request) {
		balancesHandler(w, r, url)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>MiningPoolHub Exporter</title></head>
             <body>
             <h1>MiningPoolHub Exporter</h1>
						 <p>Usage: ` + *metricsPath + `?apikey=apikey&conversion=EUR</p>
             </body>
             </html>`))
	})
	fmt.Println("Starting HTTP server on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func balancesHandler(w http.ResponseWriter, r *http.Request, mphURL *url.URL) {
	apikey := r.URL.Query().Get("apikey")
	if apikey == "" {
		http.Error(w, "apikey must be provided", http.StatusBadRequest)
		return
	}

	fiat := r.URL.Query().Get("fiat")
	if fiat == "" {
		fiat = "EUR"
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(&Exporter{
		APIKey:           apikey,
		FiatCurrency:     fiat,
		MiningPoolHubURL: mphURL,
	})

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func (e *Exporter) Collect(metrics chan<- prometheus.Metric) {
	balances, err := e.getBalances()
	if err != nil {
		log.Printf("Couldn't fetch balances: %v", err)
		return
	}

	minedCoinSymbols := []string{}
	for _, balance := range balances.Data.Balances {
		minedCoinSymbols = append(minedCoinSymbols, SYMBOLS[balance.Coin])
	}

	prices, err := e.getPrices(minedCoinSymbols)
	if err != nil {
		log.Printf("Couldn't fetch prices: %v", err)
		return
	}

	infoGauge.Set(1)
	for _, balance := range balances.Data.Balances {
		symbol := SYMBOLS[balance.Coin]

		balanceGauge.WithLabelValues(balance.Coin, symbol, "normal", "confirmed").Set(balance.Confirmed)
		balanceGauge.WithLabelValues(balance.Coin, symbol, "normal", "unconfirmed").Set(balance.Unconfirmed)
		balanceGauge.WithLabelValues(balance.Coin, symbol, "auto", "confirmed").Set(balance.AEConfirmed)
		balanceGauge.WithLabelValues(balance.Coin, symbol, "auto", "unconfirmed").Set(balance.AEUnconfirmed)
		balanceGauge.WithLabelValues(balance.Coin, symbol, "exchange", "confirmed").Set(balance.Exchange)

		balanceConvertedGauge.WithLabelValues(balance.Coin, symbol, "normal", "confirmed").Set(balance.Confirmed / prices[symbol])
		balanceConvertedGauge.WithLabelValues(balance.Coin, symbol, "normal", "unconfirmed").Set(balance.Unconfirmed / prices[symbol])
		balanceConvertedGauge.WithLabelValues(balance.Coin, symbol, "auto", "confirmed").Set(balance.AEConfirmed / prices[symbol])
		balanceConvertedGauge.WithLabelValues(balance.Coin, symbol, "auto", "unconfirmed").Set(balance.AEUnconfirmed / prices[symbol])
		balanceConvertedGauge.WithLabelValues(balance.Coin, symbol, "exchange", "confirmed").Set(balance.Exchange / prices[symbol])
	}

	infoGauge.Collect(metrics)
	balanceGauge.Collect(metrics)
	balanceConvertedGauge.Collect(metrics)
}

func (e *Exporter) Describe(descs chan<- *prometheus.Desc) {
	infoGauge.Describe(descs)
	balanceGauge.Describe(descs)
	balanceConvertedGauge.Describe(descs)
}

func (e *Exporter) getBalances() (*GetUserAllBalances, error) {
	balancesURL := url.URL{}
	balancesURL.Host = e.MiningPoolHubURL.Host
	balancesURL.Scheme = e.MiningPoolHubURL.Scheme
	balancesURL.Path = "index.php"
	q := balancesURL.Query()
	q.Add("page", "api")
	q.Add("action", "getuserallbalances")
	q.Add("api_key", e.APIKey)
	balancesURL.RawQuery = q.Encode()

	client := http.Client{
		Timeout: time.Second * 10,
	}

	req, err := http.NewRequest(http.MethodGet, balancesURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "miningpoolhub-exporter")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf(res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	balances := &GetUserAllBalances{}
	err = json.Unmarshal(body, balances)
	if err != nil {
		return nil, err
	}

	return balances, nil
}

func (e *Exporter) getPrices(symbols []string) (Prices, error) {
	priceURL, _ := url.Parse("https://min-api.cryptocompare.com/data/price")
	q := priceURL.Query()
	q.Add("fsym", e.FiatCurrency)
	q.Add("tsyms", strings.Join(symbols, ","))
	priceURL.RawQuery = q.Encode()

	client := http.Client{
		Timeout: time.Second * 10,
	}

	req, err := http.NewRequest(http.MethodGet, priceURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "miningpoolhub-exporter")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf(res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	prices := Prices{}
	err = json.Unmarshal(body, &prices)
	if err != nil {
		return nil, err
	}

	return prices, nil
}
