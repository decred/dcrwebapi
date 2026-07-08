// Copyright (c) 2017-2026 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/decred/vspd/types/v3"
)

// Vsp contains information about a single Voting Service Provider. Includes
// info hard-coded in dcrwebapi and info retrieved from the VSPs /vspinfo
// endpoint.
type Vsp struct {
	// Hard-coded in dcrwebapi.
	Network  string `json:"network"`
	Launched int64  `json:"launched"`
	// Set by dcrwebapi each time info is successfully updated.
	LastUpdated int64 `json:"lastupdated"`
	// Retrieved from the /api/vspinfo.
	APIVersions                []int64 `json:"apiversions"`
	FeePercentage              float64 `json:"feepercentage"`
	Closed                     bool    `json:"closed"`
	Voting                     int64   `json:"voting"`
	Voted                      int64   `json:"voted"`
	Expired                    int64   `json:"expired"`
	Missed                     int64   `json:"missed"`
	VspdVersion                string  `json:"vspdversion"`
	BlockHeight                uint32  `json:"blockheight"`
	EstimatedNetworkProportion float32 `json:"estimatednetworkproportion"`
}
type vspSet map[string]Vsp

type priceInfo struct {
	BitcoinUSD  float64 `json:"bitcoin_usd"`
	DecredUSD   float64 `json:"decred_usd"`
	LastUpdated int64   `json:"lastupdated"`
}

type webInfo struct {
	Circulating float64 `json:"circulatingsupply"`
	Ultimate    float64 `json:"ultimatesupply"`
	Staked      float64 `json:"stakedsupply"`
	BlockReward float64 `json:"blockreward"`
	Treasury    float64 `json:"treasury"`
	TicketPrice float64 `json:"ticketprice"`
	Height      uint32  `json:"height"`
	LastUpdated int64   `json:"lastupdated"`
}

// Service represents a dcrweb service.
type Service struct {
	// the http client
	HTTPClient *http.Client
	// the http router
	Router *http.ServeMux

	// Data cached by the service, protected by a mutex.
	sync.RWMutex
	Vsps      vspSet
	WebInfo   webInfo
	PriceInfo priceInfo
}

// NewService creates a new dcrwebapi service.
func NewService() *Service {
	service := Service{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 2,
			},
			Timeout: time.Second * 10,
		},
		Router: http.NewServeMux(),

		Vsps: vspSet{
			"teststakepool.decred.org": Vsp{
				Network:  "testnet",
				Launched: getUnixTime(2020, 6, 1),
			},
			"testnet-vsp.jholdstock.uk": Vsp{
				Network:  "testnet",
				Launched: getUnixTime(2021, 1, 20),
			},
			"dcrvsp.ubiqsmart.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 12, 25),
			},
			"stakey.net": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 10, 22),
			},
			"vsp.stakeminer.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 11, 9),
			},
			"vsp.decredcommunity.org": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 11, 05),
			},
			"ultravsp.uk": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 12, 1),
			},
			"vsp.dcr.farm": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 12, 9),
			},
			"decredvoting.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2021, 2, 1),
			},
			"decred.stake.fun": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2021, 1, 28),
			},
			"big.decred.energy": {
				Network:  "mainnet",
				Launched: getUnixTime(2022, 5, 1),
			},
			"dcr.cerebro.host": {
				Network:  "mainnet",
				Launched: getUnixTime(2024, 9, 9),
			},
			"dcrhive.com": {
				Network:  "mainnet",
				Launched: getUnixTime(2022, 6, 23),
			},
			"vspd.bass.cf": {
				Network:  "mainnet",
				Launched: getUnixTime(2022, 5, 1),
			},
			"vote.dcr-swiss.ch": {
				Network:  "mainnet",
				Launched: getUnixTime(2023, 6, 30),
			},
		},
	}

	// Start update ticker.
	go func() {
		for {
			service.vspData()
			err := service.info()
			if err != nil {
				log.Printf("Error updating web info: %v", err)
			}
			err = service.price()
			if err != nil {
				log.Printf("Error updating price info: %v", err)
			}
			<-time.After(time.Minute * 5)
		}
	}()

	// setup route
	service.Router.HandleFunc("/", service.HandleRoutes)
	return &service
}

// getHTTP will use the services HTTP client to send a GET request to the
// provided URL. Returns the response body, or an error.
func (s *Service) getHTTP(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to create request: %w", url, err)
	}

	req.Header.Set("User-Agent", "decred/dcrweb bot")
	poolResp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to send request: %w", url, err)
	}
	defer poolResp.Body.Close()

	if poolResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v: non-success status: %d",
			url, poolResp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(poolResp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return nil, fmt.Errorf("%v: failed to read body: %w", url, err)
	}

	return respBody, nil
}

func (s *Service) vspStats(url string, vsp Vsp) error {
	infoURL := fmt.Sprintf("https://%s/api/v3/vspinfo", url)

	infoResp, err := s.getHTTP(infoURL)
	if err != nil {
		return err
	}

	var info types.VspInfoResponse
	err = json.Unmarshal(infoResp, &info)
	if err != nil {
		return fmt.Errorf("%v: unmarshal failed: %w", infoURL, err)
	}

	vsp.APIVersions = info.APIVersions
	vsp.FeePercentage = info.FeePercentage
	vsp.Closed = info.VspClosed
	vsp.Voting = info.Voting
	vsp.Voted = info.Voted
	vsp.Expired = info.Expired
	vsp.Missed = info.Missed
	vsp.VspdVersion = info.VspdVersion
	vsp.BlockHeight = info.BlockHeight
	vsp.EstimatedNetworkProportion = info.NetworkProportion

	vsp.LastUpdated = time.Now().Unix()

	s.Lock()
	s.Vsps[url] = vsp
	s.Unlock()

	return nil
}

func (s *Service) vspData() {
	var wg sync.WaitGroup
	s.RLock()
	wg.Add(len(s.Vsps))
	for url, vsp := range s.Vsps {
		go func(url string, vsp Vsp) {
			defer wg.Done()
			err := s.vspStats(url, vsp)
			if err != nil {
				log.Println(err)
			}
		}(url, vsp)
	}
	s.RUnlock()
	wg.Wait()
}

// dcrdata gets an API response from dcrdata and unmarshals it.
func (s *Service) dcrdata(path string, response interface{}) error {
	body, err := s.getHTTP("https://dcrdata.decred.org/api" + path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, response)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) price() error {
	var exchange struct {
		DcrPrice float64 `json:"dcrPrice"`
		BtcPrice float64 `json:"btcPrice"`
	}
	err := s.dcrdata("/exchangerate", &exchange)
	if err != nil {
		return err
	}

	s.Lock()
	s.PriceInfo = priceInfo{
		BitcoinUSD:  exchange.BtcPrice,
		DecredUSD:   exchange.DcrPrice,
		LastUpdated: time.Now().Unix(),
	}
	s.Unlock()

	return nil
}

func (s *Service) info() error {
	var supply CoinSupply
	err := s.dcrdata("/supply", &supply)
	if err != nil {
		return err
	}

	var bestBlock BlockDataBasic
	err = s.dcrdata("/block/best", &bestBlock)
	if err != nil {
		return err
	}

	var treasury TreasuryBalance
	err = s.dcrdata("/treasury/balance", &treasury)
	if err != nil {
		return err
	}

	var subsidy BlockSubsidies
	err = s.dcrdata("/block/best/subsidy", &subsidy)
	if err != nil {
		return err
	}

	// toDCR converts atoms to DCR.
	toDCR := func(atoms int64) float64 {
		return float64(atoms) / math.Pow10(8)
	}

	s.Lock()
	s.WebInfo = webInfo{
		Circulating: toDCR(supply.Mined),
		Ultimate:    toDCR(supply.Ultimate),
		Staked:      bestBlock.PoolInfo.Value,
		BlockReward: toDCR(subsidy.Work * 100),
		Treasury:    toDCR(treasury.Balance),
		TicketPrice: bestBlock.StakeDiff,
		Height:      bestBlock.Height,
		LastUpdated: time.Now().Unix(),
	}
	s.Unlock()

	return nil
}

// HandleRoutes is the handler func for all endpoints exposed by the service
func (s *Service) HandleRoutes(writer http.ResponseWriter, request *http.Request) {
	route := request.URL.Query().Get("c")
	switch route {

	case "vsp":
		s.RLock()
		respJSON, err := json.Marshal(s.Vsps)
		s.RUnlock()
		if err != nil {
			writeJSONErrorResponse(&writer, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return

	case "webinfo":
		s.RLock()
		respJSON, err := json.Marshal(s.WebInfo)
		s.RUnlock()
		if err != nil {
			writeJSONErrorResponse(&writer, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return

	case "price":
		s.RLock()
		respJSON, err := json.Marshal(s.PriceInfo)
		s.RUnlock()
		if err != nil {
			writeJSONErrorResponse(&writer, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return

	default:
		writer.WriteHeader(http.StatusNotFound)
		return
	}
}
