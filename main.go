package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var botID string

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Could not load config file: %s", err)
	}

	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %s", err)
	}

	u, err := dg.User("@me")
	if err != nil {
		log.Fatalf("Error retrieving bot user: %s", err)
	}
	botID = u.ID

	dg.AddHandler(messageHandler)

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %s", err)
	}

	fmt.Println("FOOTBALL GOBOT ONLINE")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	fmt.Println("FOOTBALL GOBOT POWERING DOWN")
	dg.Close()
}

func messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == botID {
		return
	}

	checkReaccs(s, m)
}

func checkReaccs(s *discordgo.Session, m *discordgo.MessageCreate) {
	for _, u := range m.Mentions {
		if u.ID == botID {
			s.MessageReactionAdd(m.ChannelID, m.ID, "🤖")
			break
		}
	}

	message := strings.ToLower(m.Content)
	if strings.Contains(message, "69") {
		s.MessageReactionAdd(m.ChannelID, m.ID, "🇳")
		s.MessageReactionAdd(m.ChannelID, m.ID, "🇮")
		s.MessageReactionAdd(m.ChannelID, m.ID, "🇨")
		s.MessageReactionAdd(m.ChannelID, m.ID, "🇪")
	}
	if strings.Contains(message, "football") {
		s.MessageReactionAdd(m.ChannelID, m.ID, "🏈")
	}
	if strings.Contains(message, "butt") {
		s.MessageReactionAdd(m.ChannelID, m.ID, "🍑")
	}
}
