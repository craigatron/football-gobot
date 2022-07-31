package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/craigatron/football-gobot/config"
)

var botID string
var botConfig *config.JSON
var leaguesByCategory map[string]*config.LeagueClient

var buildCommit string
var buildDate string

func main() {
	log.Printf("build at commit %s on %s", buildCommit, buildDate)

	bc, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Could not load config file: %s", err)
	}
	botConfig = &bc

	err = loadLeagues()
	if err != nil {
		log.Fatalf("Error initializing FFL leagues: %s", err)
	}

	dg, err := discordgo.New("Bot " + botConfig.Token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %s", err)
	}

	u, err := dg.User("@me")
	if err != nil {
		log.Fatalf("Error retrieving bot user: %s", err)
	}
	botID = u.ID

	dg.AddHandler(messageHandler)

	command := &discordgo.ApplicationCommand{
		Name:        "bot-version",
		Type:        discordgo.ChatApplicationCommand,
		Description: "See what version of FOOTBALL GOBOT is active",
	}
	_, err = dg.ApplicationCommandCreate(botConfig.AppID, "", command)
	if err != nil {
		log.Fatalf("Error creating application command: %s", err)
	}
	command = &discordgo.ApplicationCommand{
		Name:        "debug",
		Type:        discordgo.ChatApplicationCommand,
		Description: "Get debug info about GOBOT",
	}
	_, err = dg.ApplicationCommandCreate(botConfig.AppID, "", command)
	if err != nil {
		log.Fatalf("Error creating application command: %s", err)
	}

	dg.AddHandler(commandHandler)

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %s", err)
	}

	log.Println("FOOTBALL GOBOT ONLINE")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("FOOTBALL GOBOT POWERING DOWN")
	dg.Close()
}

func loadLeagues() error {
	leagues, err := config.CreateLeagueClients(*botConfig)
	if err != nil {
		return err
	}

	leaguesByCategory = make(map[string]*config.LeagueClient)
	for _, v := range leagues {
		for _, d := range v.LeagueConfig.DiscordCategoryIDs {
			leaguesByCategory[d] = v
		}
	}
	return nil
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
	for _, reacc := range botConfig.ReaccConfig.Reaccs {
		match, _ := regexp.MatchString(reacc.Pattern, message)
		if match {
			for _, ignoreConfig := range botConfig.ReaccConfig.IgnoreReaccs {
				if ignoreConfig.IgnoreReacc == reacc.Reacc && ignoreConfig.UserID == m.Author.ID {
					continue
				}
			}
			for _, reaccChar := range reacc.Reacc {
				s.MessageReactionAdd(m.ChannelID, m.ID, string(reaccChar))
			}
		}
	}
}

func commandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	channel, err := s.Channel(i.ChannelID)
	if err != nil {
		log.Printf("error getting channel: %s", err)
		return
	}

	league, ok := leaguesByCategory[channel.ParentID]
	if !ok {
		log.Printf("no league mapped for channel %s with category ID %s", channel.ID, channel.ParentID)
		return
	}

	data := i.ApplicationCommandData()
	switch data.Name {
	case "bot-version":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "FOOTBALL GOBOT",
						URL:         fmt.Sprintf("https://github.com/craigatron/football-gobot/tree/%s", buildCommit),
						Description: fmt.Sprintf("Built %s at commit hash %s", buildDate, buildCommit),
					},
				},
			},
		})
	case "debug":
		var leagueID string
		if league.LeagueType == config.LeagueTypeESPN {
			leagueID = league.ESPNLeague.ID
		} else if league.LeagueType == config.LeagueTypeSleeper {
			leagueID = league.SleeperLeague.ID
		} else {
			leagueID = "N/A"
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Description: "debug info",
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:  "Discord Category ID",
								Value: channel.ParentID,
							},
							{
								Name:  "League Type",
								Value: league.LeagueType.String(),
							},
							{
								Name:  "League ID",
								Value: leagueID,
							},
						},
					},
				},
			},
		})
	}
}
