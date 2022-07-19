package updateactivity

import (
	"context"
	"log"

	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"

	"github.com/craigatron/football-gobot/config"
)

type PubsubMessage struct {
	Data []byte `json:"data"`
}

func UpdateActivity(ctx context.Context, m PubsubMessage) error {
	conf, err := config.LoadConfig()
	if err != nil {
		log.Printf("error loading config: %s", err)
		return err
	}
	log.Printf("config: %s", conf)

	log.Printf("data: %s", m.Data)
	return nil
}
