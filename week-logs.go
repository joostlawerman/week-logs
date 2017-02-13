package main

import (
	"log"
	"google.golang.org/api/sheets/v4"
	"time"
	"html/template"
	"fmt"
	"bytes"
	"os/exec"
	"os"
	"encoding/json"
	"io/ioutil"
)

func main() {
	config := &Config{}

	configBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Unable to read the config file. %v", err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatalf("Unable to unmarshal the config file. %v", err)
	}

	oauthClient := setupOAuth()

	sheetsService, err := sheets.New(oauthClient)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets Client %v", err)
	}

	logs, err := collectLogs(sheetsService, config.Sheet.Id, config.Sheet.Selection)
	if err != nil {
		log.Fatalf("Unable to generate the records from the sheet. %v", err)
	}

	funcMap	:= template.FuncMap{
		"date": func(format string, date time.Time) string {
			return date.Format(format)
		},
		"minutes": func(duration time.Duration) string {
			return fmt.Sprintf("%vmin", duration.Minutes())
		},
	}

	t, err := template.New("templates/"+ config.Language +".html").Funcs(funcMap).ParseFiles("template.html")

	for week, logs := range logs {
		htmlBuffer := bytes.NewBufferString("")

		err := t.Execute(htmlBuffer, struct{
			Name string
			Week int
			Logs []*Log
			Company *Company
			Columns []string
		}{
			Name: config.Name,
			Week: week,
			Logs: logs,
			Company: &config.Company,
			Columns: config.Sheet.Columns,
		})

		if err != nil {
			log.Fatalf("Unable to render html. %v", err)
		}

		cmd := exec.Command("wkhtmltopdf", "-", fmt.Sprintf(config.Result, week))
		cmd.Stdin = htmlBuffer
		cmd.Stdout = os.Stdout
		err = cmd.Run()

		if err != nil {
			log.Fatalf("Unable to render pdf. %v", err)
		}
	}
}

func collectLogs(srv *sheets.Service, spreadsheetId, readRange string) (logs map[int][]*Log, _ error) {
	logs = make(map[int][]*Log)

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet. %v", err)
	}

	for _, row := range resp.Values {
		log, err := NewLog(row[0].(string), row[3].(string), row[1].(string), row[2].(string))
		if err != nil {
			return nil, err
		}
		_, week := log.Day.ISOWeek()

		logs[week] = append(logs[week], log)
	}
	return
}

func NewLog(day, duration, description, particularities string) (*Log, error) {
	parsedDay, err := time.Parse("02/01/06", day)
	if err != nil {
		return nil, err
	}
	parsedDuration, err := time.ParseDuration(duration)
	if err != nil {
		return nil, err
	}

	return &Log{
		Day:             parsedDay,
		Duration:        parsedDuration,
		Description:     description,
		Particularities: particularities,
	}, nil
}

type Config struct {
	Language string `json:"language"`
	Name string `json:"name"`
	Result string `json:"result"`
	Company Company `json:"company"`
	Sheet Sheet `json:"sheet"`
}

type Company struct {
	Name string `json:"name"`
	Leader string `json:"leader"`
}

type Sheet struct {
	Id string `json:"id"`
	Selection string `json:"selection"`
	Columns []string `json:"columns"`
}

type Log struct {
	Day time.Time
	Duration time.Duration
	Description string
	Particularities string
}

