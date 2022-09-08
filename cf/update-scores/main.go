package updatescores

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
	"github.com/craigatron/sleeper-go"
)

// PubsubMessage is the type of the message triggering this function.
type PubsubMessage struct {
	Data []byte `json:"data"`
}

// UpdateScores is the entry point for this cloud function.
func UpdateScores(ctx context.Context, m PubsubMessage) error {
	log.Printf("Starting update scores run with data %s", m)
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
			err = processESPNLeague(ctx, fsClient, league.ESPNLeague)
			if err != nil {
				log.Printf("error processing ESPN league %s: %s", league.ESPNLeague.ID, err)
			}
		} else if league.LeagueType == config.LeagueTypeSleeper {
			err = processSleeperLeague(ctx, fsClient, league.SleeperLeague)
			if err != nil {
				log.Printf("error processing Sleeper league %s: %s", league.SleeperLeague.ID, err)
			}
		} else {
			log.Printf("skipping unknown league type %s", league.LeagueType)
		}
	}

	return nil
}

func processESPNLeague(ctx context.Context, fsClient *firestore.Client, league *espn.League) error {
	leagueYearKey := fmt.Sprintf("leagues/espn-%s/years/%d", league.ID, league.Year)
	log.Printf("processing key %s", leagueYearKey)
	leagueYear := fsClient.Doc(leagueYearKey)

	batch := fsClient.Batch()

	weeksCollection := fsClient.Collection(fmt.Sprintf("%s/weeks", leagueYearKey))
	currentWeekDoc := weeksCollection.Doc(fmt.Sprintf("%d", league.CurrentWeek))
	projectionsCollection := currentWeekDoc.Collection("projections")

	newUpdateTime := time.Now()
	// UnixMilli doesn't exist in Go 1.16, which is the latest version cloud functions has :(
	newUpdateMillis := newUpdateTime.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))

	matchups, err := league.Scoreboard()
	if err != nil {
		return err
	}

	for _, matchup := range matchups {
		homeDoc := projectionsCollection.Doc(fmt.Sprintf("%d-%d", newUpdateMillis, matchup.HomeTeam.ID))
		batch.Set(homeDoc, map[string]interface{}{
			"matchup_id": matchup.ID,
			"projection": matchup.HomeScore,
			"team_id":    matchup.HomeTeam.ID,
			"timestamp":  newUpdateMillis,
		})
		awayDoc := projectionsCollection.Doc(fmt.Sprintf("%d-%d", newUpdateMillis, matchup.AwayTeam.ID))
		batch.Set(awayDoc, map[string]interface{}{
			"matchup_id": matchup.ID,
			"projection": matchup.AwayScore,
			"team_id":    matchup.AwayTeam.ID,
			"timestamp":  newUpdateMillis,
		})

	}

	batch.Set(leagueYear, map[string]interface{}{"scores_updated": newUpdateTime}, firestore.MergeAll)
	_, err = batch.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}

func processSleeperLeague(ctx context.Context, fsClient *firestore.Client, league *sleeper.League) error {
	status, err := league.Client.GetNflStatus()
	if err != nil {
		return err
	}
	leagueYearKey := fmt.Sprintf("leagues/sleeper-%s/years/%s", league.ID, status.Season)
	log.Printf("processing key %s", leagueYearKey)
	leagueYear := fsClient.Doc(leagueYearKey)

	batch := fsClient.Batch()

	weeksCollection := fsClient.Collection(fmt.Sprintf("%s/weeks", leagueYearKey))
	currentWeekDoc := weeksCollection.Doc(fmt.Sprintf("%d", status.Week))
	projectionsCollection := currentWeekDoc.Collection("projections")

	newUpdateTime := time.Now()
	// UnixMilli doesn't exist in Go 1.16, which is the latest version cloud functions has :(
	newUpdateMillis := newUpdateTime.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))

	projections, err := league.GetProjections()
	if err != nil {
		return err
	}

	for _, projection := range projections {
		homeDoc := projectionsCollection.Doc(fmt.Sprintf("%d-%d", newUpdateMillis, projection.Matchup.RosterID))
		batch.Set(homeDoc, map[string]interface{}{
			"matchup_id": projection.Matchup.MatchupID,
			"projection": projection.Projection,
			"team_id":    projection.Matchup.RosterID,
			"timestamp":  newUpdateMillis,
		})
	}

	batch.Set(leagueYear, map[string]interface{}{"scores_updated": newUpdateTime}, firestore.MergeAll)
	_, err = batch.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}
