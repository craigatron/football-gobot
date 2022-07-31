package updateactivity

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	// Need this for cloud functions
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"

	"cloud.google.com/go/firestore"
	"github.com/craigatron/espn-fantasy-go"
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

	leagues, err := config.CreateLeagueClients(conf)
	if err != nil {
		log.Printf("error creating leagues: %s", err)
		return err
	}

	fsClient, err := firestore.NewClient(ctx, os.Getenv("PROJECT"))
	if err != nil {
		log.Printf("error creating firestore client: %s", err)
		return err
	}

	for _, league := range leagues {
		if league.LeagueType == config.LeagueTypeESPN {
			err = processESPNLeague(ctx, fsClient, league)
			if err != nil {
				log.Printf("error processing ESPN league %s: %s", league.ESPNLeague.ID, err)
			}
		} else {
			log.Printf("skipping sleeper league %s", league.SleeperLeague.ID)
		}
	}

	return nil
}

func processESPNLeague(ctx context.Context, fsClient *firestore.Client, league *config.LeagueClient) error {
	leagueYearKey := fmt.Sprintf("leagues/espn-%s/years/%d", league.ESPNLeague.ID, league.ESPNLeague.Year)
	log.Printf("processing key %s", leagueYearKey)
	leagueYear := fsClient.Doc(leagueYearKey)

	doc, err := leagueYear.Get(ctx)
	if err != nil {
		return err
	}
	var updateTimestamp time.Time
	leagueData := doc.Data()
	if u, ok := leagueData["activity_updated"]; ok {
		updateTimestamp = u.(time.Time)
	} else {
		updateTimestamp = time.UnixMilli(0)
	}

	batch := fsClient.Batch()
	if _, ok := leagueData["config"]; !ok {

		members := make([]espn.LeagueMemberJSON, 0)
		for _, m := range league.ESPNLeague.Members {
			members = append(members, *m)
		}
		teams := make([]map[string]interface{}, 0)
		for _, t := range league.ESPNLeague.Teams {
			teams = append(teams, map[string]interface{}{
				"id":           t.ID,
				"abbreviation": t.Abbreviation,
				"name":         t.Name,
				"owners":       t.Owners,
			})
		}
		log.Printf("adding config: %v", teams)
		batch.Set(leagueYear, map[string]interface{}{
			"config": map[string]interface{}{
				"members": members,
				"teams":   teams,
			},
		}, firestore.MergeAll)
	} else {
		log.Print("not adding config")
	}

	activityCollection := fsClient.Collection(fmt.Sprintf("%s/activity", leagueYearKey))

	newUpdateTime := time.Now()
	offset := 0

Loop:
	for {
		log.Printf("processing offset %d", offset)
		ra, err := league.ESPNLeague.RecentActivity(25, offset)
		offset += 25
		if err != nil {
			return err
		}
		if len(ra) == 0 {
			log.Printf("reached end of recent activity list, breaking")
			break
		}

		for _, activity := range ra {
			if activity.Timestamp <= updateTimestamp.UnixMilli() {
				log.Printf("activity %v older than last update %v, stopping update", activity, updateTimestamp)
				break Loop
			}
			fsActivity := activityCollection.Doc(activity.ESPNID)
			batch.Set(fsActivity, map[string]interface{}{
				"actions":   activity.Actions,
				"timestamp": activity.Timestamp,
			})
		}
	}

	batch.Set(leagueYear, map[string]interface{}{"activity_updated": newUpdateTime}, firestore.MergeAll)
	_, err = batch.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}
