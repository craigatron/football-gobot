package updateactivity

import (
	"context"
	"log"

	// Need this for cloud functions
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"

	"github.com/craigatron/football-gobot/config"
)

// PubsubMessage is the type of the message triggering this function.
type PubsubMessage struct {
	Data []byte `json:"data"`
}

// UpdateActivity is the entry point for this cloud function.
func UpdateActivity(ctx context.Context, m PubsubMessage) error {
	log.Printf("Starting update activity run with data %s", m)
	conf, err := config.LoadConfig()
	if err != nil {
		log.Printf("error loading config: %s", err)
		return err
	}

	_, err = config.CreateLeagueClients(conf)
	if err != nil {
		log.Printf("error creating leagues: %s", err)
		return err
	}

	return nil
}
