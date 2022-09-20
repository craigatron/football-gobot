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
	"time"

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

	err = initFirestoreClient()
	if err != nil {
		log.Fatalf("error initializing firestore client: %s", err)
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
	command = &discordgo.ApplicationCommand{
		Name:        "activity",
		Type:        discordgo.ChatApplicationCommand,
		Description: "Show recent activity for this league",
	}
	_, err = dg.ApplicationCommandCreate(botConfig.AppID, "", command)
	if err != nil {
		log.Fatalf("Error creating application command: %s", err)
	}
	command = &discordgo.ApplicationCommand{
		Name:        "charts",
		Type:        discordgo.ChatApplicationCommand,
		Description: "Get link to current projections charts",
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

	// strip out mentions, links, channels, etc.  only reacc the legit stuff.
	ignoreMessageRe := regexp.MustCompile(`(<@!\d+>)|(<#\d+>)|(<@\d+>)|(<@&\d+)|((https|http)?://(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*))`)
	message := strings.ToLower(m.Content)
	message = ignoreMessageRe.ReplaceAllString(message, "")

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
		handleBotVersionCommand(s, i)
	case "debug":
		handleDebugCommand(s, i, league, channel)
	case "activity":
		handleActivityCommand(s, i, league, channel)
	case "charts":
		handleChartsCommand(s, i, league, channel)
	}
}

func handleBotVersionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

func handleDebugCommand(s *discordgo.Session, i *discordgo.InteractionCreate, league *config.LeagueClient, channel *discordgo.Channel) {
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

func handleActivityCommand(s *discordgo.Session, i *discordgo.InteractionCreate, league *config.LeagueClient, channel *discordgo.Channel) {
	if league.LeagueType != config.LeagueTypeESPN {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("/activity not implemented for league type %s", league.LeagueType),
			},
		})
		return
	}
	fmt.Printf("handling activity command for league %v and channel %v", *league, *channel)
	recentActivity, err := getRecentESPNActivity(league.ESPNLeague)
	if err != nil {
		log.Printf("error getting recent activity: %s\n", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "could not get recent activity for league",
			},
		})
		return
	}
	fields := make([]*discordgo.MessageEmbedField, 0)
	for _, ra := range recentActivity {
		actionStrings := make([]string, 0)
		for _, action := range ra.Actions {
			player := league.ESPNLeague.Players[action.PlayerID]
			actionStrings = append(actionStrings, fmt.Sprintf("%s %s %s (%s, %s)", league.ESPNLeague.Teams[action.TeamID].Name, action.Action, player.FullName, player.Position, player.Team))
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  time.UnixMilli(ra.Timestamp).String(),
			Value: strings.Join(actionStrings, "\n"),
		})
	}
	log.Printf("got recent activity: %v", recentActivity)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Fields: fields,
				},
			},
		},
	})
}

func handleChartsCommand(s *discordgo.Session, i *discordgo.InteractionCreate, league *config.LeagueClient, channel *discordgo.Channel) {
	var week int
	var season string
	var id string
	if league.LeagueType == config.LeagueTypeESPN {
		week = league.ESPNLeague.CurrentWeek
		season = fmt.Sprintf("%d", league.ESPNLeague.Year)
		id = league.ESPNLeague.ID
	} else if league.LeagueType == config.LeagueTypeSleeper {
		status, err := league.SleeperLeague.Client.GetNflStatus()
		if err != nil {
			log.Printf("error getting sleeper status: %s\n", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "could not get Sleeper league status",
				},
			})
			return
		}
		week = status.Week
		season = league.SleeperLeague.Season
		id = league.SleeperLeague.ID
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title: fmt.Sprintf("Week %d charts", week),
					URL:   fmt.Sprintf("https://storage.googleapis.com/%s/%s/%s/%d/index.html", os.Getenv("PROJECTION_BUCKET"), id, season, week),
				},
			},
		},
	})
}
