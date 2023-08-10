package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type DomProblem struct {
	Ordinal   int     `json:"ordinal,omitempty"`
	Id        string  `json:"id"`
	ShortName string  `json:"short_name"`
	Label     string  `json:"label"`
	TimeLimit float64 `json:"time_limit"`
	Name      string  `json:"name"`
}

type DomSubmission struct {
	Id        string `json:"id"`
	ProblemId string `json:"problem_id"`
	TeamId    string `json:"team_id"`
}

type DomJudgements struct {
	Id              string `json:"id"`
	JudgementTypeId string `json:"judgement_type_id"`
	SubmissionId    string `json:"submission_id"`
}

type ContestData struct {
	FormalName               string `json:"formal_name"`
	PenaltyTime              int    `json:"penalty_time"`
	StartTime                string `json:"start_time"`
	EndTime                  string `json:"end_time"`
	Duration                 string `json:"duration"`
	ScoreboardFreezeDuration string `json:"scoreboard_freeze_duration"`
	Id                       string `json:"id"`
	ExternalId               string `json:"external_id"`
	Name                     string `json:"name"`
	Shortname                string `json:"shortname"`
}

type IEnvironment struct {
	Api                           string
	UserApi                       string
	PasswordApi                   string
	GuildID                       string
	AppId                         string
	BotToken                      string
	ArtemisaUrl                   string
	DomJudgeUrl                   string
	VjudgeUrl                     string
	ClassRecordUrl                string
	GPCUEBurl                     string
	RedProgramacionCompetitivaUrl string
	IdesCompetitivaUrl            string
}

var payload IEnvironment

var ContestDataJSON []ContestData

func CreateCommand(s *discordgo.Session, guildID string, command *discordgo.ApplicationCommand) {
	_, err := s.ApplicationCommandCreate(payload.AppId, guildID, command)
	if err != nil {
		panic(err)
	}
}

func calificateJudgements(contest int, domSub []DomSubmission, wg *sync.WaitGroup) (map[string]int, map[string]int) {
	mapa := map[string]int{}
	mapawr := map[string]int{}
	client := &http.Client{}
	answerSub, err := http.NewRequest("GET", payload.Api+"/contests/"+strconv.Itoa(contest)+"/judgements/?strict=false", nil)
	answerSub.SetBasicAuth(payload.UserApi, payload.PasswordApi)
	if err != nil {
		panic(err)
	}
	responseSub, err := client.Do(answerSub)
	if err != nil {
		panic(err)
	}
	domJudgements := &[]DomJudgements{}
	jsonJudgements, err := ioutil.ReadAll(responseSub.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(jsonJudgements, &domJudgements)
	if err != nil {
		panic(err)
	}

	for _, current := range *domJudgements {
		if current.JudgementTypeId == "AC" {
			for _, submission := range domSub {
				if current.SubmissionId == submission.Id {
					mapa[submission.ProblemId]++
				}
			}
		}
		if current.JudgementTypeId == "WA" {
			for _, submission := range domSub {
				if current.SubmissionId == submission.Id {
					mapawr[submission.ProblemId]++
				}
			}
		}
	}
	return mapa, mapawr
}

func GenerateBarChart(s *discordgo.Session, i *discordgo.InteractionCreate, contestIdx int, maxAcc float64, ticks []chart.Tick, values []chart.Value) {
	barChart := chart.BarChart{
		Title: ContestDataJSON[contestIdx].FormalName,
		TitleStyle: chart.Style{
			FontSize:  30,
			FontColor: drawing.ColorWhite,
			Padding: chart.Box{
				Top: 20,
			},
		},
		Background: chart.Style{
			Padding: chart.Box{
				Top:    100,
				Bottom: 40,
				Right:  40,
			},
			StrokeColor: drawing.ColorBlack,
			FontColor:   drawing.ColorWhite,
			FillColor:   drawing.ColorBlack,
		},
		XAxis: chart.Style{
			StrokeColor: drawing.ColorBlack,
			FontColor:   drawing.ColorWhite,
			FillColor:   drawing.ColorBlack,
		},
		YAxis: chart.YAxis{
			Range: &chart.ContinuousRange{
				Min: 0.0,
				Max: maxAcc,
			},
			Ticks: ticks,
			Style: chart.Style{
				StrokeColor: drawing.ColorBlack,
				FontColor:   drawing.ColorWhite,
				FillColor:   drawing.ColorBlack,
			},
		},
		Width:    1500,
		Height:   800,
		BarWidth: 50,
		Bars:     values,
		Canvas: chart.Style{
			StrokeColor: drawing.ColorBlack,
			FontColor:   drawing.ColorWhite,
			FillColor:   drawing.ColorBlack,
		},
	}

	// Crea una imagen del gráfico
	buffer := bytes.NewBuffer([]byte{})
	err := barChart.Render(chart.PNG, buffer)
	if err != nil {
		log.Fatalf("Error rendering chart: %v\n", err)
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Files: []*discordgo.File{
				{
					Name:   "chart.png",
					Reader: buffer,
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}
}

func Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	switch i.ApplicationCommandData().Name {
	case "ping":
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong!",
			},
		})
		if err != nil {
			panic(err)
		}
	case "chart":
		shortNameContest := i.ApplicationCommandData().Options[0].StringValue()

		contest := 0
		contestIdx := -1
		var err error
		for idx, i := range ContestDataJSON {
			if i.Shortname == shortNameContest {
				contest, err = strconv.Atoi(i.Id)
				contestIdx = idx
			}
		}

		if contestIdx == -1 {
			return
		}
		domGraph, err := http.Get(payload.Api + "/contests/" + strconv.Itoa(contest) + "/problems?strict=false")
		if err != nil {
			panic(err)
		}
		domSubmission, err := http.Get(payload.Api + "/contests/" + strconv.Itoa(contest) + "/submissions?strict=false")
		if err != nil {
			panic(err)
		}

		dataDom := &[]DomProblem{}
		submDom := &[]DomSubmission{}
		responseData, err := ioutil.ReadAll(domGraph.Body)
		err = json.Unmarshal(responseData, &dataDom)
		if err != nil {
			panic(err)
		}
		responseDataSubmit, err := ioutil.ReadAll(domSubmission.Body)
		err = json.Unmarshal(responseDataSubmit, &submDom)
		if err != nil {
			panic(err)
		}
		var values, valuesWr []chart.Value
		wg := sync.WaitGroup{}
		acc, wr := calificateJudgements(int(contest), *submDom, &wg)
		var maxAcc = 0.0
		for _, current := range *dataDom {
			values = append(values, chart.Value{
				Label: current.Label,
				Value: float64(acc[current.Id]),
				Style: chart.Style{
					StrokeColor: drawing.ColorWhite,
				},
			})
			valuesWr = append(valuesWr, chart.Value{
				Label: current.Label,
				Value: float64(wr[current.Id]),
				Style: chart.Style{
					StrokeColor: drawing.ColorWhite,
					FillColor:   drawing.ColorRed,
				},
			})
			maxAcc = math.Max(maxAcc, float64(acc[current.Id]))
		}

		var ticks []chart.Tick
		for i := 0; i < int(maxAcc)+1; i++ {
			ticks = append(ticks, chart.Tick{
				Value: float64(i),
				Label: strconv.Itoa(i),
			})
		}
		//GenerateBarChart(s, i, contestIdx, maxAcc, ticks, valuesWr)
		GenerateBarChart(s, i, contestIdx, maxAcc, ticks, values)
	case "links":
		if len(i.Member.Roles) > 0 {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Links de las plataformas y recursos que usamos 🤓",
					//Flags:   discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "📚",
									},
									Label: "Artemisa",
									Style: discordgo.LinkButton,
									URL:   payload.ArtemisaUrl,
								},
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "👨‍⚖️",
									},
									Label: "DomJudge UEB",
									Style: discordgo.LinkButton,
									URL:   payload.DomJudgeUrl,
								},
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "🧑‍🏫",
									},
									Label: "Clases Grabadas",
									Style: discordgo.LinkButton,
									URL:   payload.ClassRecordUrl,
								},
							},
						},
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "👨‍💻",
									},
									Label: "GitHub GPC",
									Style: discordgo.LinkButton,
									URL:   payload.GPCUEBurl,
								},
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "😎",
									},
									Label: "Vjudge",
									Style: discordgo.LinkButton,
									URL:   payload.VjudgeUrl,
								},
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "💪",
									},
									Label: "registro RPC",
									Style: discordgo.LinkButton,
									URL:   payload.RedProgramacionCompetitivaUrl,
								},
							},
						},
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Emoji: discordgo.ComponentEmoji{
										Name: "⌨️",
									},
									Label: "IDES",
									Style: discordgo.LinkButton,
									URL:   payload.IdesCompetitivaUrl,
								},
							},
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}
			return
		}
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    "😕 No cuentas con los permisos para visualizar esta sección.",
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{},
			},
		})
		if err != nil {
			panic(err)
		}
	case "test":
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Are you comfortable with buttons and other message components?",
				Flags:   discordgo.MessageFlagsEphemeral,
				// Buttons and other components are specified in Components field.
				Components: []discordgo.MessageComponent{
					// ActionRow is a container of all buttons within the same row.
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								// Label is what the user will see on the button.
								Label: "Yes",
								// Style provides coloring of the button. There are not so many styles tho.
								Style: discordgo.SuccessButton,
								// Disabled allows bot to disable some buttons for users.
								Disabled: false,
								// CustomID is a thing telling Discord which data to send when this button will be pressed.
								CustomID: "fd_yes",
							},
							discordgo.Button{
								Label:    "No",
								Style:    discordgo.DangerButton,
								Disabled: false,
								CustomID: "fd_no",
							},
							discordgo.Button{
								Label:    "I don't know",
								Style:    discordgo.LinkButton,
								Disabled: false,
								// Link buttons don't require CustomID and do not trigger the gateway/HTTP event
								URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
								Emoji: discordgo.ComponentEmoji{
									Name: "🤷",
								},
							},
						},
					},
					// The message may have multiple actions rows.
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "Discord Developers server",
								Style:    discordgo.LinkButton,
								Disabled: false,
								URL:      "https://discord.gg/discord-developers",
							},
						},
					},
				},
			},
		})
		if err != nil {
			panic(err)
		}
	case "info":
		readFile, err := os.Open("./README.md")
		if err != nil {
			panic(err)
		}
		fileScanner := bufio.NewScanner(readFile)
		fileScanner.Split(bufio.ScanLines)
		var fileLines []string

		for fileScanner.Scan() {
			fileLines = append(fileLines, fileScanner.Text())
		}

		err = readFile.Close()
		if err != nil {
			return
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: strings.Join(fileLines[:], "\n"),
			},
		})
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	content, err := ioutil.ReadFile("./environment.json")
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(content, &payload)
	if err != nil {
		panic(err)
	}

	bot, err := discordgo.New("Bot " + payload.BotToken)
	if err != nil {
		panic(err)
	}

	contestSelected, err := http.Get(payload.Api + "/contests")
	responseContest, err := ioutil.ReadAll(contestSelected.Body)
	err = json.Unmarshal(responseContest, &ContestDataJSON)
	if err != nil {
		panic(err)
	}
	var choicesCompetitive []*discordgo.ApplicationCommandOptionChoice
	for _, i := range ContestDataJSON {
		choicesCompetitive = append(choicesCompetitive, &discordgo.ApplicationCommandOptionChoice{
			Value: i.Shortname,
			Name:  i.FormalName,
		})
	}
	var commands []discordgo.ApplicationCommand
	commands = append(commands,
		discordgo.ApplicationCommand{
			Name:        "ping",
			Description: "Utilizado para conocer el estado del Bot.",
		},
		discordgo.ApplicationCommand{
			Name:        "chart",
			Description: "Muestra una lista deplegable con las competencias activas en DomJudge",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "competencia",
					Description: "nombre corto de la competencia",
					Required:    true,
					Choices:     choicesCompetitive,
				},
			},
		},
		discordgo.ApplicationCommand{
			Name:        "links",
			Description: "Despliega un listado de recursos y utilidades para los entrenamientos.",
		},
		discordgo.ApplicationCommand{
			Name:        "info",
			Description: "Muestra una descripción del Bot y el listado de comandos disponibles.",
		},
	)
	for _, command := range commands {
		CreateCommand(bot, payload.GuildID, &command)
	}

	bot.AddHandler(Handler)
	err = bot.Open()
	if err != nil {
		panic(err)
	}

	fmt.Println("Bot is running")
	<-make(chan struct{})
}
