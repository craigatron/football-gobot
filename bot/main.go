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
	"github.com/craigatron/espn-fantasy-go"
	"github.com/craigatron/football-gobot/config"
	"github.com/craigatron/sleeper-go"
)

type leagueClient struct {
	LeagueType    LeagueType
	ESPNLeague    *espn.League
	SleeperLeague *sleeper.League
}

var botID string
var botConfig config.ConfigType
var leaguesByCategory map[string]leagueClient

var buildCommit string
var buildDate string

func main() {
	log.Printf("build at commit %s on %s", buildCommit, buildDate)

	botConfig, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Could not load config file: %s", err)
	}

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
	leaguesByCategory = make(map[string]leagueClient)
	for _, l := range botConfig.LeagueConfig {
		if l.LeagueType == "sleeper" {
			league, err := sleeper.NewLeague(l.ID, botConfig.SleeperConfig.Token)
			if err != nil {
				return err
			}
			leaguesByCategory[l.DiscordCategoryID] = leagueClient{
				LeagueType:    LeagueTypeSleeper,
				SleeperLeague: &league,
			}
		} else if l.LeagueType == "espn" {
			var league espn.League
			var err error
			if botConfig.ESPNConfig.ESPNS2 == "" && botConfig.ESPNConfig.SWID == "" {
				league, err = espn.NewPublicLeague(espn.GameTypeNfl, l.ID, 2022)
			} else {
				league, err = espn.NewPrivateLeague(espn.GameTypeNfl, l.ID, 2022, botConfig.ESPNConfig.ESPNS2, botConfig.ESPNConfig.SWID)
			}
			if err != nil {
				return err
			}
			leaguesByCategory[l.DiscordCategoryID] = leagueClient{
				LeagueType: LeagueTypeESPN,
				ESPNLeague: &league,
			}

		} else {
			return fmt.Errorf("unknown league type %s", l.LeagueType)
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
	}
}
