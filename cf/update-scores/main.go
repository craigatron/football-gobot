package updatescores

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"time"

	// Need this for cloud functions
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
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

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("error creating storage client: %s", err)
		return err
	}

	projectionBucket := storageClient.Bucket(os.Getenv("PROJECTION_BUCKET"))

	for _, league := range leagues {
		if league.LeagueType == config.LeagueTypeESPN {
			err = processESPNLeague(ctx, fsClient, league.ESPNLeague, projectionBucket)
			if err != nil {
				log.Printf("error processing ESPN league %s: %s", league.ESPNLeague.ID, err)
			}
		} else if league.LeagueType == config.LeagueTypeSleeper {
			err = processSleeperLeague(ctx, fsClient, league.SleeperLeague, projectionBucket)
			if err != nil {
				log.Printf("error processing Sleeper league %s: %s", league.SleeperLeague.ID, err)
			}
		} else {
			log.Printf("skipping unknown league type %s", league.LeagueType)
		}
	}

	return nil
}

type Projection struct {
	MatchupID  int64   `firestore:"matchup_id"`
	Projection float64 `firestore:"projection"`
	TeamID     int64   `firestore:"team_id"`
	Timestamp  int64   `firestore:"timestamp"`
}

func processESPNLeague(ctx context.Context, fsClient *firestore.Client, league *espn.League, projectionBucket *storage.BucketHandle) error {
	leagueYearKey := fmt.Sprintf("leagues/espn-%s/years/%d", league.ID, league.Year)
	log.Printf("processing key %s", leagueYearKey)
	leagueYear := fsClient.Doc(leagueYearKey)

	batch := fsClient.Batch()

	weeksCollection := fsClient.Collection(fmt.Sprintf("%s/weeks", leagueYearKey))
	currentWeekDoc := weeksCollection.Doc(fmt.Sprintf("%d", league.CurrentWeek))
	projectionsCollection := currentWeekDoc.Collection("projections")

	allProjections := make([]Projection, 0)
	allProjectionsIter := projectionsCollection.Documents(ctx)
	for {
		doc, err := allProjectionsIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		var projectionData Projection
		if err := doc.DataTo(&projectionData); err != nil {
			return err
		}
		allProjections = append(allProjections, projectionData)
	}

	newUpdateTime := time.Now()
	// UnixMilli doesn't exist in Go 1.16, which is the latest version cloud functions has :(
	newUpdateMillis := newUpdateTime.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))

	matchups, err := league.Scoreboard()
	if err != nil {
		return err
	}

	for _, matchup := range matchups {
		homeDoc := projectionsCollection.Doc(fmt.Sprintf("%d-%d", newUpdateMillis, matchup.HomeTeam.ID))
		homeData := Projection{
			MatchupID:  matchup.ID,
			Projection: matchup.HomeScore,
			TeamID:     matchup.HomeTeam.ID,
			Timestamp:  newUpdateMillis,
		}
		batch.Set(homeDoc, homeData)
		allProjections = append(allProjections, homeData)

		awayDoc := projectionsCollection.Doc(fmt.Sprintf("%d-%d", newUpdateMillis, matchup.AwayTeam.ID))
		awayData := Projection{
			MatchupID:  matchup.ID,
			Projection: matchup.AwayScore,
			TeamID:     matchup.AwayTeam.ID,
			Timestamp:  newUpdateMillis,
		}
		batch.Set(awayDoc, awayData)
		allProjections = append(allProjections, awayData)

	}

	batch.Set(leagueYear, map[string]interface{}{"scores_updated": newUpdateTime}, firestore.MergeAll)
	_, err = batch.Commit(ctx)
	if err != nil {
		return err
	}

	teamIDToName := make(map[int64]string)
	for _, team := range league.Teams {
		teamIDToName[int64(team.ID)] = team.Name
	}
	writeHTML(projectionBucket, config.LeagueTypeESPN, league.ID, fmt.Sprintf("%d", league.Year), league.CurrentWeek, teamIDToName, allProjections)
	return nil
}

func processSleeperLeague(ctx context.Context, fsClient *firestore.Client, league *sleeper.League, projectionBucket *storage.BucketHandle) error {
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

	allProjections := make([]Projection, 0)
	allProjectionsIter := projectionsCollection.Documents(ctx)
	for {
		doc, err := allProjectionsIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		var projectionData Projection
		if err := doc.DataTo(&projectionData); err != nil {
			return err
		}
		allProjections = append(allProjections, projectionData)
	}

	newUpdateTime := time.Now()
	// UnixMilli doesn't exist in Go 1.16, which is the latest version cloud functions has :(
	newUpdateMillis := newUpdateTime.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))

	projections, err := league.GetProjections()
	if err != nil {
		return err
	}

	for _, projection := range projections {
		doc := projectionsCollection.Doc(fmt.Sprintf("%d-%d", newUpdateMillis, projection.Matchup.RosterID))
		data := Projection{
			MatchupID:  int64(projection.Matchup.MatchupID),
			Projection: projection.Projection,
			TeamID:     int64(projection.Matchup.RosterID),
			Timestamp:  newUpdateMillis,
		}
		batch.Set(doc, data)
		allProjections = append(allProjections, data)
	}

	batch.Set(leagueYear, map[string]interface{}{"scores_updated": newUpdateTime}, firestore.MergeAll)
	_, err = batch.Commit(ctx)
	if err != nil {
		return err
	}

	teamIDToName := make(map[int64]string)
	for _, team := range league.Rosters {
		owner := league.Users[team.OwnerID]
		var teamName string
		if owner.Metadata.TeamName != "" {
			teamName = owner.Metadata.TeamName
		} else {
			teamName = owner.DisplayName
		}
		teamIDToName[int64(team.RosterID)] = teamName
	}

	writeHTML(projectionBucket, config.LeagueTypeSleeper, league.ID, league.Season, status.Week, teamIDToName, allProjections)

	return nil
}

type ChartProjection struct {
	Timestamp  string  `json:"timestamp"`
	Projection float64 `json:"projection"`
}

type TemplateData struct {
	Team1Name string
	Team2Name string
	Team1Data string
	Team2Data string
}

type IndexMatchupData struct {
	Team1Name string
	Team2Name string
	URL       string
}

type IndexData struct {
	Week     int
	Matchups []IndexMatchupData
}

//go:embed matchup.html
var matchupTemplate string

//go:embed week_index.html
var indexTemplate string

func writeHTML(projectionBucket *storage.BucketHandle, leagueType config.LeagueType, leagueID string, season string, week int, teamIDToName map[int64]string, projections []Projection) error {
	log.Printf("generating HTML for %s league %s", leagueType, leagueID)

	dir, err := ioutil.TempDir("", fmt.Sprintf("%s-%s", leagueType, leagueID))
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	matchupProjections := make(map[int64]map[int64][]ChartProjection)
	for _, p := range projections {
		if _, ok := matchupProjections[p.MatchupID]; !ok {
			matchupProjections[p.MatchupID] = make(map[int64][]ChartProjection, 0)
		}
		if _, ok := matchupProjections[p.MatchupID][p.TeamID]; !ok {
			matchupProjections[p.MatchupID][p.TeamID] = make([]ChartProjection, 0)
		}
		tp := matchupProjections[p.MatchupID][p.TeamID]
		matchupProjections[p.MatchupID][p.TeamID] = append(tp, ChartProjection{
			Timestamp:  time.Unix(0, p.Timestamp*int64(time.Millisecond)).Format(time.RFC3339),
			Projection: p.Projection,
		})
	}

	indexData := IndexData{Week: week, Matchups: make([]IndexMatchupData, 0)}

	matchupTmpl, err := template.New("matchupHTML").Parse(matchupTemplate)
	if err != nil {
		return err
	}

	for matchupID := range matchupProjections {
		thisMatchupProjections := matchupProjections[matchupID]

		teams := make([]int64, 0, 2)
		for t, _ := range thisMatchupProjections {
			teams = append(teams, t)
		}

		if len(teams) != 2 {
			return errors.New("unexpected number of teams in matchup")
		}

		team1Data, err := json.Marshal(thisMatchupProjections[teams[0]])
		if err != nil {
			return err
		}
		team2Data, err := json.Marshal(thisMatchupProjections[teams[1]])
		if err != nil {
			return err
		}

		templateData := TemplateData{
			Team1Name: teamIDToName[teams[0]],
			Team2Name: teamIDToName[teams[1]],
			Team1Data: string(team1Data),
			Team2Data: string(team2Data),
		}

		ctx := context.Background()
		objectName := fmt.Sprintf("%s/%s/%d/%d.html", leagueID, season, week, matchupID)
		obj := projectionBucket.Object(objectName)
		w := obj.NewWriter(ctx)
		w.ContentType = "text/html"

		err = matchupTmpl.Execute(w, templateData)
		if err != nil {
			return err
		}

		if err := w.Close(); err != nil {
			return err
		}

		indexData.Matchups = append(indexData.Matchups, IndexMatchupData{
			Team1Name: teamIDToName[teams[0]],
			Team2Name: teamIDToName[teams[1]],
			URL:       fmt.Sprintf("https://storage.googleapis.com/%s/%s/%s/%d/%d.html", os.Getenv("PROJECTION_BUCKET"), leagueID, season, week, matchupID),
		})
	}

	// write index file
	tmpl, err := template.New("indexHTML").Parse(indexTemplate)
	if err != nil {
		return err
	}

	ctx := context.Background()
	objectName := fmt.Sprintf("%s/%s/%d/index.html", leagueID, season, week)
	obj := projectionBucket.Object(objectName)
	w := obj.NewWriter(ctx)
	w.ContentType = "text/html"

	err = tmpl.Execute(w, indexData)
	if err != nil {
		return err
	}

	w.Close()

	return nil
}
