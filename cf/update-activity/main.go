package updateactivity

import (
	"context"
	"log"

	// Need this for cloud functions
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"

	"github.com/craigatron/espn-fantasy-go"
	"github.com/craigatron/football-gobot/config"
	"github.com/craigatron/sleeper-go"
)

// PubsubMessage is the type of the message triggering this function.
type PubsubMessage struct {
	Data []byte `json:"data"`
}

const year = 2021

// UpdateActivity is the entry point for this cloud function.
func UpdateActivity(ctx context.Context, m PubsubMessage) error {
	log.Printf("Starting update activity run with data %s", m)
	conf, err := config.LoadConfig()
	if err != nil {
		log.Printf("error loading config: %s", err)
		return err
	}

	for _, lc := range conf.LeagueConfig {
		if lc.LeagueType == "sleeper" {
			log.Printf("Creating sleeper league %s", lc.ID)
			_, err = sleeper.NewLeague(lc.ID, conf.SleeperConfig.Token)
			if err != nil {
				log.Print("error creating sleeper league")
				return err
			}
		} else if lc.LeagueType == "espn" {
			log.Printf("Creating espn league %s", lc.ID)
			// TODO: support public leagues
			_, err = espn.NewPrivateLeague(espn.GameTypeNfl, lc.ID, year, conf.ESPNConfig.ESPNS2, conf.ESPNConfig.SWID)
			if err != nil {
				log.Print("error creating sleeper league")
				return err
			}
		}
	}

	return nil
}
