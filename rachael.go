package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/websocket"
	"github.com/eikeon/hu"
	"github.com/eikeon/hue"
)

const DEV string = "C040BHR3K"

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Message struct {
	// Event
	Id        int    `json:"id"`
	Type      string `json:"type"`
	Error     *Error `json:"error,omitempty"`
	Channel   string `json:"channel"`
	User      string `json:"User,omitempty"`
	Text      string `json:"text"`
	TimeStamp string `json:"ts,omitempty"`
	
	//Confirmation
	Ok bool `json:"ok"`
	ReplyTo int `json:"reply_to"`
}

type RTM struct {
	token   string
	ws      *websocket.Conn
	in, out chan Message
	ids chan int
	previousStart time.Time
}

func (r *RTM) start() {
	if time.Now().Sub(r.previousStart) < time.Second {
		time.Sleep(10*time.Second)
	}
	r.previousStart = time.Now()
	resp, err := http.PostForm("https://slack.com/api/rtm.start", url.Values{"token": {r.token}})
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(resp.Body)
	var v struct {
		Ok       bool   `json:"ok"`
		Url      string `json:"url,omitempty"`
		Channels []struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"channels"`
	}
	if err := dec.Decode(&v); err != nil {
		log.Fatal("error decoding")
	}
	log.Printf("start response: %#v\n", v)
	origin := "http://localhost/"
	ws, err := websocket.Dial(v.Url, "", origin)
	if err != nil {
		log.Fatal(err)
	}
	r.ws = ws
}

func (r *RTM) run() {
	go func () {
		r.ids = make(chan int)
		for id:=0; ; id++ {
			r.ids <- id
		}
		close(r.ids)
	}()
	r.start()
	go func() {
		for {
			var m Message
			if err := websocket.JSON.Receive(r.ws, &m); err == nil {
				r.in <- m
			} else if err == io.EOF {
				// TODO: backoff / max retry
				log.Println("EOF... restarting")
				r.start()
			} else {
				log.Println("websocket receive:", err)
				r.start()				
			}
		}
	}()
	const duration = 10 * time.Second
	timer := time.NewTimer(duration)
	for {
		select {
		case m := <-r.out:
			err := websocket.JSON.Send(r.ws, m)
			if err != nil {
				log.Println("Error sending message:", err)
			}
			timer.Reset(duration)
		case <-timer.C:
			timer = time.NewTimer(duration)
			websocket.JSON.Send(r.ws, &Message{Type: "ping"})
		}
	}
}

type frame map[hu.Symbol]hu.Term

func (frame frame) Define(variable hu.Symbol, value hu.Term) {
	log.Printf("Define: %v - value: %#v\n", variable, value)
	frame[variable] = value
}

func (frame frame) Set(variable hu.Symbol, value hu.Term) bool {
	_, ok := frame[variable]
	return ok
}

func (frame frame) Get(variable hu.Symbol) (hu.Term, bool) {
	value, ok := frame[variable]
	return value, ok
}

func main() {
	t := os.Getenv("SLACK_TOKEN")
	if t == "" {
		log.Fatal("SLACK_TOKEN not defined")
	}
	r := &RTM{token: t, in: make(chan Message, 50), out: make(chan Message, 50)}
	//environment := hu.NewEnvironment()
	//environment := hu.NewEnvironmentWithFrame(make(frame))
	environment := hu.NewEnvironmentWithFrame(&dbframe{})
	hu.AddDefaultBindings(environment)
	environment.AddPrimitive("HueSetState", r.hueSetState)
	environment.AddPrimitive("in", r.runIn)
	environment.AddPrimitive("at", r.runAt)
	environment.Define("blink", hu.String(`{"alert": "select"}`))
	
	go r.run()
	for e := range r.in {
		switch e.Type {
		case "":
			log.Printf("confirmation: %#v\n", e)
		case "hello":
			r.out <- Message{Type: "ping"}
			r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: "Hello world"}
		case "pong":
			log.Println("pong")
		case "error":
			log.Println("Error: Message -> ", e.Error.Message, "Code -> ", e.Error.Code)
			r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: "Error: " + e.Error.Message}
		case "message":
			go func(m Message) {
				log.Printf("seeing message: %#v\n", m)
				//if strings.HasPrefix(m.Text, "{") {
				input := m.Text
				input = strings.Replace(input, `“`, `"`, -1)
				input = strings.Replace(input, `”`, `"`, -1)				
				reader := strings.NewReader(input)
				expression := hu.Read(reader)
				result := environment.Evaluate(expression)
				if result != nil {
					r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: fmt.Sprintf("%v", result)}
				}
				//}
			}(e)
		case "team_migration_started":
			log.Printf("team migration started")
			r.start()
		case "user_typing":
			log.Println("User typing")
		case "presence_change":
			log.Println("Presence change")
		default:
			fmt.Printf("Received: %s.\n", e)
		}
	}
}

func (r *RTM) hueSetState(environment *hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	address := environment.Evaluate(terms[0])
	value := environment.Evaluate(terms[1])
	log.Printf("hueSetState: %#v: %v", address, value)
	h := &hue.Hue{Username: "28dd21d2f61467f1d0cf7a01b9725f"}	
	for {
		var state map[string]interface{}
		dec := json.NewDecoder(strings.NewReader(value.String()))
		if err := dec.Decode(&state); err != nil {
			log.Println("hue decode err:", err)
		}

		err := h.Set(address.String(), &state)
		if err != nil {
			log.Println("error:", err)
			if err := h.CreateUser(h.Username, "Marvin"); err == nil {
				log.Println("h:", h)
			} else {
				text := fmt.Sprintf("%s: press hue link button to authenticate", err)
				log.Println(text)
				r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: text}
				time.Sleep(time.Second)
			}
		} else {
			return nil
		}
	}
	return nil
	//return &Number{result}
}

func (r *RTM) runIn(environment *hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	d := environment.Evaluate(terms[0]).(hu.Term).String()
	wait, err := time.ParseDuration(d)
	if err != nil {
		log.Println("err: ", err)
		return nil
	}
	now := time.Now()	
	r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: fmt.Sprintf("scheduled `%s` to run at %s", terms[1], now.Add(wait).Format("Monday, January 2, 3:04pm"))}	
	go func() {
		time.Sleep(wait)
		action := environment.Evaluate(terms[1])
		r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: fmt.Sprintf("As requested running `%s` now", terms[1])}
		environment.Evaluate(hu.Application([]hu.Term{action}))
	}()
	return nil
}

func (r *RTM) runAt(environment *hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	when := environment.Evaluate(terms[0]).(hu.Term).String()
	action := environment.Evaluate(terms[1])
	now := time.Now()
	zone, _ := now.Zone()
	on, err := time.Parse("2006-01-02 "+time.Kitchen+" MST", now.Format("2006-01-02 ")+when+" "+zone)
	if err != nil {
		log.Println("could not parse when of '" + when + "' for " + action.String())
		return nil
	}
	duration := 60 * 60 * 24 * time.Second
	wait := time.Duration((on.UnixNano() - now.UnixNano()) % int64(duration))
	if wait < 0 {
		wait += duration
	}
	r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: fmt.Sprintf("scheduled `%s` to run at %s", terms[1], now.Add(wait).Format("Monday, January 2, 3:04pm"))}
	go func() {
		time.Sleep(wait)
		r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: fmt.Sprintf("As requested running `%s` now", terms[1])}		
		environment.Evaluate(hu.Application([]hu.Term{action}))
	}()
	return nil
}
