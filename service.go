// Copyright (c) 2017-2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/decred/dcrd/dcrutil/v4"
	apitypes "github.com/decred/dcrdata/v6/api/types"
	"github.com/decred/dcrdata/v6/db/dbtypes"
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
	BlockHeight                uint64  `json:"blockheight"`
	EstimatedNetworkProportion float64 `json:"estimatednetworkproportion"`
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
	Vsps      vspSet
	WebInfo   webInfo
	PriceInfo priceInfo
	Mutex     sync.RWMutex
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
		Mutex:  sync.RWMutex{},

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
			"vspd.99split.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 11, 17),
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
			vspData(&service)
			err := info(&service)
			if err != nil {
				log.Printf("Error updating web info: %v", err)
			}
			err = price(&service)
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
func (service *Service) getHTTP(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to create request: %v",
			url, err)
	}

	req.Header.Set("User-Agent", "decred/dcrweb bot")
	poolResp, err := service.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to send request: %v",
			url, err)
	}
	defer poolResp.Body.Close()

	if poolResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v: non-success status: %d",
			url, poolResp.StatusCode)
	}

	respBody, err := io.ReadAll(poolResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to read body: %v",
			url, err)
	}

	return respBody, nil
}

func vspStats(service *Service, url string) error {
	var vsp Vsp

	service.Mutex.RLock()
	vsp = service.Vsps[url]
	service.Mutex.RUnlock()
	infoURL := fmt.Sprintf("https://%s/api/v3/vspinfo", url)

	infoResp, err := service.getHTTP(infoURL)
	if err != nil {
		return err
	}

	var info map[string]interface{}
	err = json.Unmarshal(infoResp, &info)
	if err != nil {
		return fmt.Errorf("%v: unmarshal failed: %v",
			infoURL, err)
	}

	apiversions, hasAPIVersions := info["apiversions"]
	feepercentage, hasFeePercentage := info["feepercentage"]
	vspclosed, hasClosed := info["vspclosed"]
	voting, hasVoting := info["voting"]
	voted, hasVoted := info["voted"]
	expired, hasExpired := info["expired"]
	missed, hasMissed := info["missed"]
	version, hasVersion := info["vspdversion"]
	blockheight, hasBlockHeight := info["blockheight"]
	networkproportion, hasnetworkproportion := info["estimatednetworkproportion"]

	hasRequiredFields := hasAPIVersions && hasFeePercentage &&
		hasClosed && hasVoting && hasVoted && hasExpired && hasMissed &&
		hasVersion && hasBlockHeight && hasnetworkproportion

	if !hasRequiredFields {
		return fmt.Errorf("%v: missing required fields: %+v", infoURL, info)
	}

	vsp.APIVersions = make([]int64, 0)
	for _, i := range apiversions.([]interface{}) {
		vsp.APIVersions = append(vsp.APIVersions, int64(i.(float64)))
	}

	vsp.FeePercentage = feepercentage.(float64)
	vsp.Closed = vspclosed.(bool)
	vsp.Voting = int64(voting.(float64))
	vsp.Voted = int64(voted.(float64))
	vsp.Expired = int64(expired.(float64))
	vsp.Missed = int64(missed.(float64))
	vsp.VspdVersion = version.(string)
	vsp.BlockHeight = uint64(blockheight.(float64))
	vsp.EstimatedNetworkProportion = networkproportion.(float64)

	vsp.LastUpdated = time.Now().Unix()

	service.Mutex.Lock()
	service.Vsps[url] = vsp
	service.Mutex.Unlock()

	return nil
}

func vspData(service *Service) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(service.Vsps))
	for url := range service.Vsps {
		go func(url string) {
			defer waitGroup.Done()
			err := vspStats(service, url)
			if err != nil {
				log.Println(err)
			}
		}(url)
	}
	waitGroup.Wait()
}

// dcrdata gets an API response from dcrdata and unmarshals it.
func (service *Service) dcrdata(path string, response interface{}) error {
	body, err := service.getHTTP("https://dcrdata.decred.org/api" + path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, response)
	if err != nil {
		return err
	}

	return nil
}

func price(service *Service) error {
	var exchange struct {
		DcrPrice float64 `json:"dcrPrice"`
		BtcPrice float64 `json:"btcPrice"`
	}
	err := service.dcrdata("/exchangerate", &exchange)
	if err != nil {
		return err
	}

	service.Mutex.Lock()
	service.PriceInfo = priceInfo{
		BitcoinUSD:  exchange.BtcPrice,
		DecredUSD:   exchange.DcrPrice,
		LastUpdated: time.Now().Unix(),
	}
	service.Mutex.Unlock()

	return nil
}

func info(service *Service) error {
	var supply apitypes.CoinSupply
	err := service.dcrdata("/supply", &supply)
	if err != nil {
		return err
	}

	var bestBlock apitypes.BlockDataBasic
	err = service.dcrdata("/block/best", &bestBlock)
	if err != nil {
		return err
	}

	var treasury dbtypes.TreasuryBalance
	err = service.dcrdata("/treasury/balance", &treasury)
	if err != nil {
		return err
	}

	var subsidy apitypes.BlockSubsidies
	err = service.dcrdata("/block/best/subsidy", &subsidy)
	if err != nil {
		return err
	}

	// toDCR converts atoms to DCR.
	toDCR := func(atoms int64) float64 {
		return dcrutil.Amount(atoms).ToCoin()
	}

	service.Mutex.Lock()
	service.WebInfo = webInfo{
		Circulating: toDCR(supply.Mined),
		Ultimate:    toDCR(supply.Ultimate),
		Staked:      bestBlock.PoolInfo.Value,
		BlockReward: toDCR(subsidy.Work * 100),
		Treasury:    toDCR(treasury.Balance),
		TicketPrice: bestBlock.StakeDiff,
		Height:      bestBlock.Height,
		LastUpdated: time.Now().Unix(),
	}
	service.Mutex.Unlock()

	return nil
}

// HandleRoutes is the handler func for all endpoints exposed by the service
func (service *Service) HandleRoutes(writer http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		writeJSONErrorResponse(&writer, err)
		return
	}

	route := request.FormValue("c")
	switch route {

	case "vsp":
		service.Mutex.RLock()
		respJSON, err := json.Marshal(service.Vsps)
		service.Mutex.RUnlock()
		if err != nil {
			writeJSONErrorResponse(&writer, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return

	case "webinfo":
		service.Mutex.RLock()
		respJSON, err := json.Marshal(service.WebInfo)
		service.Mutex.RUnlock()
		if err != nil {
			writeJSONErrorResponse(&writer, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return

	case "price":
		service.Mutex.RLock()
		respJSON, err := json.Marshal(service.PriceInfo)
		service.Mutex.RUnlock()
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
