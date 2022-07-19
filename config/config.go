package config

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"cloud.google.com/go/storage"
)

// Conf is the JSON config for various football-gobot mods.
type Conf struct {
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

	LeagueConfig []struct {
		LeagueType        string `json:"type"`
		Name              string `json:"name"`
		ID                string `json:"id"`
		DiscordCategoryID string `json:"discord_category_id"`
	} `json:"leagues"`
}

// LoadConfig fetches the config from GCS.
func LoadConfig() (Conf, error) {
	configBucket := os.Getenv("CONFIG_BUCKET")
	configObject := os.Getenv("CONFIG_OBJECT")

	c := Conf{}
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
