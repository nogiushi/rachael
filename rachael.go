package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/eikeon/hu"

	"golang.org/x/net/websocket"
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
	Ok      bool `json:"ok"`
	ReplyTo int  `json:"reply_to"`
}

func (message Message) String() string {
	return fmt.Sprintf("#<message> channel: %s Text: %s", message.Channel, message.Text)
}

type Rachael struct {
	token         string
	ws            *websocket.Conn
	in, out       chan Message
	ids           chan int
	previousStart time.Time
}

func (r *Rachael) rtmStart() {
	if time.Now().Sub(r.previousStart) < time.Second {
		time.Sleep(10 * time.Second)
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

func (r *Rachael) imOpen(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	user := hu.Evaluate(environment, terms[0]).(hu.Term).String()
	resp, err := http.PostForm("https://slack.com/api/im.open", url.Values{"token": {r.token}, "user": {user}})
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(resp.Body)
	var v struct {
		Ok      bool `json:"ok"`
		Channel struct {
			Id string `json:"id"`
		} `json:"channel"`
	}
	if err := dec.Decode(&v); err != nil {
		log.Fatal("error decoding")
	}
	log.Printf("im.open response: %#v\n", v)
	return nil
}

func (r *Rachael) run(env hu.Environment) {
	go func() {
		r.ids = make(chan int)
		for id := 0; ; id++ {
			r.ids <- id
		}
		close(r.ids)
	}()
	r.rtmStart()
	go func() {
		for {
			var m Message
			if err := websocket.JSON.Receive(r.ws, &m); err == nil {
				r.in <- m
			} else if err == io.EOF {
				// TODO: backoff / max retry
				log.Println("EOF... restarting")
				r.rtmStart()
			} else {
				log.Println("websocket receive:", err)
				r.rtmStart()
			}
		}
	}()
	go func() {
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
	}()
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
				var input string
				if strings.HasPrefix(m.Channel, "D") {
					input = m.Text
				} else {
					const prefix = "<@U03V77HBT>: "
					if strings.HasPrefix(m.Text, prefix) {
						input = m.Text[len(prefix):]
					}
				}
				if input != "" {
					input = strings.Replace(input, `“`, `"`, -1)
					input = strings.Replace(input, `”`, `"`, -1)
					hu.Evaluate(env, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("receiveMessage"), hu.String(m.Channel), hu.String(m.User), hu.String(input)})))
				}
			}(e)
		case "team_migration_started":
			log.Printf("team migration started")
			r.rtmStart()
		case "user_typing":
			log.Println("User typing")
		case "presence_change":
			log.Println("Presence change")
		default:
			fmt.Printf("Received: %s.\n", e)
		}
	}
}

func (r *Rachael) sendMessage(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	channel := hu.Evaluate(environment, terms[0]).(hu.Term).String()
	text := hu.Evaluate(environment, terms[1]).(hu.Term).String()
	r.out <- Message{Id: <-r.ids, Type: "message", Channel: channel, Text: text}
	return nil
}

func (r *Rachael) receiveMessage(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	channel := hu.Evaluate(environment, terms[0])
	user := hu.Evaluate(environment, terms[1])
	text := hu.Evaluate(environment, terms[2])
	reader := strings.NewReader(text.String())
	expression := hu.ReadMessage(reader)

	ee := &hu.NestedEnvironment{Environment: hu.LocalEnvironment{hu.Symbol("channel"): channel, hu.Symbol("user"): user, hu.Symbol("text"): text}, Parent: environment}
	hu.AddDefaultBindings(ee)
	e := &hu.NestedEnvironment{Environment: &dbframe{}, Parent: ee}

	result := hu.GuardedEvaluate(e, hu.Application(expression))
	if result != nil {
		r.out <- Message{Id: <-r.ids, Type: "message", Channel: channel.String(), Text: fmt.Sprintf("%v", result)}
	}
	return result
}

func main() {
	config := flag.Bool("config", false, "drop into REPL to configure environment")
	flag.Parse()

	// TODO: persistent config env
	env := hu.LocalEnvironment{}

	if *config {
		hu.AddDefaultBindings(env)

		reader := bufio.NewReader(os.Stdin)

		var result hu.Term
		fmt.Printf("hu> ")
		for {
			expression := hu.Read(reader)
			if expression != nil {
				if expression == hu.Symbol("\n") {
					if result != nil {
						fmt.Fprintf(os.Stdout, "%v\n", result)
					}
					fmt.Printf("hu> ")
					continue
				} else {
					result = hu.Evaluate(env, expression)
				}
			} else {
				fmt.Fprintf(os.Stdout, "Goodbye!\n")
				break
			}
		}
	}

	// get name of Rachael environement from config environment (keying off of RESIN_DEVICE_UUID)

	// TODO: get from Rachael environment
	t := os.Getenv("SLACK_TOKEN")
	if t == "" {
		log.Fatal("SLACK_TOKEN not defined")
	}

	r := &Rachael{token: t, in: make(chan Message, 50), out: make(chan Message, 50)}

	hu.AddPrimitive(env, "sendMessage", r.sendMessage)
	hu.AddPrimitive(env, "receiveMessage", r.receiveMessage)
	hu.AddPrimitive(env, "tell", r.sendMessage)
	hu.AddPrimitive(env, "imopen", r.imOpen)

	//hu.AddPrimitive(env, "in", runIn)

	hu.AddPrimitive(env, "forecast", getForecast)
	env.Define(hu.Symbol("weekday"), hu.Primitive(weekday))
	env.Define(hu.Symbol("weekend"), hu.Primitive(weekend))

	pe := &hu.NestedEnvironment{Environment: &dbframe{}, Parent: env}
	st, ok := pe.Get(hu.Symbol("scheduler"))
	if !ok {
		st = &Scheduler{}
		pe.Define(hu.Symbol("scheduler"), st)
	}
	s := st.(*Scheduler)
	s.Run(pe)

	hu.AddPrimitive(env, "at", s.runAt)
	hu.AddPrimitive(env, "in", s.runIn)
	hu.AddPrimitive(env, "stop", s.stop)
	hu.AddPrimitive(env, "start", s.start)
	hu.AddPrimitive(env, "remove", s.remove)
	hu.AddPrimitive(env, "schedule", s.schedule)
	hu.AddPrimitive(env, "s", s.schedule)

	hue := NewHue(pe)
	hu.AddPrimitive(env, "HueSetState", hue.SetState)
	hu.AddPrimitive(env, "turn", hue.SetState)

	r.run(env)
}
