// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
	"github.com/line/line-bot-sdk-go/linebot"
)

var bot *linebot.Client

var (
	loc, _ = time.LoadLocation("Asia/Taipei")
	sqlObj = SqlDoc{
		Host:   "localhost",
		Port:   5432,
		User:   os.Getenv("db_user"),
		Pass:   os.Getenv("db_pass"),
		DbName: "postgres",
	}
)

type SqlDoc struct {
	Host   string
	Port   int
	User   string
	Pass   string
	DbName string
	Client *sql.DB
}

type DayInfo struct {
	Serial int
	Pos    string
	Day    string
}

func init() {
	sqlObj.init()
}
func main() {
	var err error
	bot, err = linebot.New(os.Getenv("ChannelSecret"), os.Getenv("ChannelAccessToken"))
	log.Println("Bot:", bot, " err:", err)
	http.HandleFunc("/callback", callbackHandler)
	port := os.Getenv("PORT")
	addr := fmt.Sprintf(":%s", port)
	http.ListenAndServe(addr, nil)
	// time.Now()
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	var position string
	var err error
	events, err := bot.ParseRequest(r)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(500)
		}
		return
	}

	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				quota, err := bot.GetMessageQuota().Do()
				if err != nil {
					log.Println("Quota err:", err)
				}
				if message.Text == "Where" {

					position, err = sqlObj.queryTarget()
					if err != nil {
						sqlObj.insertToday(position)
						position, _ = sqlObj.queryTarget()
					}
					reply := "Today is " + position
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(reply)).Do(); err != nil {
						log.Print(err)
					}
				} else if message.Text == "Debug:Where" {

					position, err = sqlObj.queryTarget()
					bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Done querying err:"+err.Error())).Do()
					if err != nil {
						sqlObj.insertToday(position)
						bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Done inserting err:"+err.Error())).Do()
						position, _ = sqlObj.queryTarget()
						bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Done post querying err:"+err.Error())).Do()
					}
					reply := "Today is " + position
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(reply)).Do(); err != nil {
						log.Print(err)
					}
				} else {
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(message.ID+":"+message.Text+" OK! remain message:"+strconv.FormatInt(quota.Value, 10))).Do(); err != nil {
						log.Print(err)
					}
				}

			}
		}
	}
}

func (s *SqlDoc) init() {
	var err error
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		s.Host, s.Port, s.User, s.Pass, s.DbName)
	s.Client, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
}

func (s *SqlDoc) insertToday(oldPos string) {
	var newPos string
	sqlStatement := `
INSERT INTO injection (position, day)
VALUES ($1, $2)`
	if oldPos == "right" {
		newPos = "left"
	} else {
		newPos = "right"
	}
	_, err := s.Client.Exec(sqlStatement, newPos, time.Now().In(loc).String())
	if err != nil {
		panic(err)
	}
}

func (s *SqlDoc) queryTarget() (string, error) {
	info := DayInfo{}
	sqlStatement := `SELECT * FROM injection WHERE day=$1`
	// Get current date time
	today := time.Now().In(loc).String()
	row := s.Client.QueryRow(sqlStatement, today[:10])
	err := row.Scan(&info.Serial, &info.Pos, &info.Day)
	switch err {
	case sql.ErrNoRows:
		fmt.Println("No rows were returned! A new row should be created")
		return info.Pos, errors.New("should create today's record")
	case nil:
		fmt.Println(info.Pos, info.Day[:10])
		return info.Pos, nil
	default:
		panic(err)
	}
	return "", nil
}
