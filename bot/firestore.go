package main

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/craigatron/espn-fantasy-go"
	"google.golang.org/api/iterator"
)

var firestoreClient *firestore.Client

func initFirestoreClient() error {
	ctx := context.Background()
	var err error
	firestoreClient, err = firestore.NewClient(ctx, os.Getenv("PROJECT"))
	if err != nil {
		return err
	}
	return nil
}

type recentActivityType struct {
	Timestamp int64 `firestore:"timestamp"`
	Actions   []struct {
		Action   string `firestore:"Action"`
		PlayerID int64  `firestore:"Player"`
		TeamID   int64  `firestore:"Team"`
	} `firestore:"actions"`
}

func getRecentESPNActivity(league *espn.League) ([]recentActivityType, error) {
	ctx := context.Background()

	recentActivity := make([]recentActivityType, 0)
	raCollection := firestoreClient.Collection(fmt.Sprintf("leagues/espn-%s/years/%d/activity", league.ID, league.Year))

	q := raCollection.OrderBy("timestamp", firestore.Desc).Limit(10)
	iter := q.Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		ra := recentActivityType{}
		if err := doc.DataTo(&ra); err != nil {
			return nil, err
		}
		recentActivity = append(recentActivity, ra)
	}
	return recentActivity, nil
}
