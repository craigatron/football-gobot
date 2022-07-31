package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"cloud.google.com/go/storage"
	"github.com/craigatron/espn-fantasy-go"
	"github.com/craigatron/sleeper-go"
)

// LeagueConfigJSON is the JSON config for an individual league.
type LeagueConfigJSON struct {
	LeagueType         string   `json:"type"`
	Name               string   `json:"name"`
	ID                 string   `json:"id"`
	DiscordCategoryIDs []string `json:"discord_category_ids"`
}

// JSON is the JSON config for various football-gobot mods.
type JSON struct {
	AppID string `json:"appId"`
	Token string `json:"token"`

	ReaccConfig struct {
		Reaccs []struct {
			Pattern string `json:"pattern"`
			Reacc   string `json:"reacc"`
		} `json:"reaccs"`

		IgnoreReaccs []struct {
			UserID      string `json:"user_id"`
			IgnoreReacc string `json:"ignore_reacc"`
		} `json:"ignore_reaccs"`
	} `json:"reacc_config"`

	ESPNConfig struct {
		SWID   string `json:"swid"`
		ESPNS2 string `json:"s2"`
	} `json:"espn_config"`

	SleeperConfig struct {
		Token string `json:"token"`
	} `json:"sleeper_config"`

	LeagueConfig []LeagueConfigJSON `json:"leagues"`
}

// LoadConfig fetches the config from GCS.
func LoadConfig() (JSON, error) {
	configBucket := os.Getenv("CONFIG_BUCKET")
	configObject := os.Getenv("CONFIG_OBJECT")

	c := JSON{}
	if configBucket == "" || configObject == "" {
		return c, errors.New("no CONFIG_BUCKET and/or CONFIG_OBJECT provided")
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return c, err
	}

	r, err := client.Bucket(configBucket).Object(configObject).NewReader(ctx)
	if err != nil {
		return c, err
	}

	f, err := ioutil.ReadAll(r)
	r.Close()
	if err != nil {
		return c, err
	}

	err = json.Unmarshal(f, &c)
	return c, err
}

// LeagueClientsKey is a key in the map returned by CreateLeagueClients.
type LeagueClientsKey struct {
	LeagueType LeagueType
	LeagueID   string
}

// LeagueClient is an ESPN or Sleeper league client.
type LeagueClient struct {
	LeagueType    LeagueType
	ESPNLeague    *espn.League
	SleeperLeague *sleeper.League
	LeagueConfig  LeagueConfigJSON
}

const defaultEspnYear = 2022

// CreateLeagueClients creates ESPN/Sleeper clients based on the given config.
func CreateLeagueClients(c JSON) (map[LeagueClientsKey]*LeagueClient, error) {
	clients := make(map[LeagueClientsKey]*LeagueClient)

	espnYearOverride := os.Getenv("ESPN_YEAR_OVERRIDE")
	var espnYear int
	if espnYearOverride == "" {
		espnYear = defaultEspnYear
	} else {
		var err error
		espnYear, err = strconv.Atoi(espnYearOverride)
		log.Printf("overriding default ESPN year with %d", espnYear)
		if err != nil {
			return clients, err
		}
	}

	for _, l := range c.LeagueConfig {
		if l.LeagueType == "sleeper" {
			league, err := sleeper.NewLeague(l.ID, c.SleeperConfig.Token)
			if err != nil {
				return clients, err
			}
			lc := &LeagueClient{
				LeagueType:    LeagueTypeSleeper,
				SleeperLeague: &league,
				LeagueConfig:  l,
			}
			clients[LeagueClientsKey{LeagueType: LeagueTypeSleeper, LeagueID: l.ID}] = lc
		} else if l.LeagueType == "espn" {
			var league espn.League
			var err error
			if c.ESPNConfig.ESPNS2 == "" && c.ESPNConfig.SWID == "" {
				league, err = espn.NewPublicLeague(espn.GameTypeNfl, l.ID, espnYear)
			} else {
				league, err = espn.NewPrivateLeague(espn.GameTypeNfl, l.ID, espnYear, c.ESPNConfig.ESPNS2, c.ESPNConfig.SWID)
			}
			if err != nil {
				return clients, err
			}
			lc := &LeagueClient{
				LeagueType:   LeagueTypeESPN,
				ESPNLeague:   &league,
				LeagueConfig: l,
			}
			clients[LeagueClientsKey{LeagueType: LeagueTypeESPN, LeagueID: l.ID}] = lc
		} else {
			return clients, fmt.Errorf("unknown league type %s", l.LeagueType)
		}
	}

	return clients, nil
}
