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

var funcMap = template.FuncMap{
	"date": func(format string, date time.Time) string {
		return date.Format(format)
	},
	"minutes": func(duration time.Duration) string {
		return fmt.Sprintf("%vmin", duration.Minutes())
	},
}
var config *Config

func main() {

	config = &Config{}

	configBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Unable to read the config file. %v", err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatalf("Unable to unmarshal the config file. %v", err)
	}

	oauthClient, err := setupOAuth()
	if err != nil {
		log.Fatalf("Unable to setup OAuth. %v", err)
	}

	sheetsService, err := sheets.New(oauthClient)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets Client %v", err)
	}

	logCollection, err := collectLogs(sheetsService, config.Sheet.Id, config.Sheet.Selection)
	if err != nil {
		log.Fatalf("Unable to generate the records from the sheet. %v", err)
	}

	t, err := template.New("pdf.html").Funcs(funcMap).ParseFiles("templates/" + config.Language + "/pdf.html")
	if err != nil {
		log.Fatalf("Unable to parse template files. %v", err)
	}

	for week, logs := range logCollection {
		err = renderLogs(t, logs, week, config.Result)
		if err != nil {
			log.Fatalf("Unable to render logs. %v", err)
		}
	}
}

func renderLogs(t *template.Template, logs[]*Log, week int, filenameFormat string) error {
	htmlBuffer := bytes.NewBufferString("")

	err := t.Execute(htmlBuffer, &TemplateData{
		Name:    config.Name,
		Week:    week,
		Logs:    logs,
		Company: &config.Company,
		Columns: config.Sheet.Columns,
	})

	if err != nil {
		return err
	}

	renderPdfFromHtml(htmlBuffer, fmt.Sprintf(filenameFormat, week))

	return nil
}

func renderPdfFromHtml(html *bytes.Buffer, file string) error {
	cmd := exec.Command("wkhtmltopdf", "-", file)
	cmd.Stdin = html
	cmd.Stdout = os.Stdout
	return cmd.Run()
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

type TemplateData struct {
	Name string
	Week int
	Logs []*Log
	Company *Company
	Columns []string
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

