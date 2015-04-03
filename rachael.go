package main

import (
	"encoding/json"
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
	user := environment.Evaluate(terms[0]).(hu.Term).String()
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

func (r *Rachael) run() {
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

func (r *Rachael) sendMessage(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	channel := environment.Evaluate(terms[0]).(hu.Term).String()
	text := environment.Evaluate(terms[1]).(hu.Term).String()
	log.Println(fmt.Sprintf(`{sendMessage "%s" "%s"}\n`, channel, text))
	r.out <- Message{Id: <-r.ids, Type: "message", Channel: channel, Text: text}
	return nil
}

type messageEnvironment struct {
	frame  hu.Frame
	parent hu.Environment
}

func (environment *messageEnvironment) String() string {
	return "#<Rachael's Message Environment>"
}

func (e *messageEnvironment) NewChildEnvironment() hu.Environment {
	return hu.NewEnvironmentWithParent(e)
}

func (environment *messageEnvironment) Extend(variables, values hu.Term) {
	environment.parent.Extend(variables, values)
}

func (environment *messageEnvironment) Define(variable hu.Symbol, value hu.Term) {
	environment.parent.Define(variable, value)
}

func (environment *messageEnvironment) Set(variable hu.Symbol, value hu.Term) {
	environment.parent.Set(variable, value)
}

func (environment *messageEnvironment) Get(variable hu.Symbol) hu.Term {
	value, ok := environment.frame.Get(variable)
	if ok {
		return value
	} else if environment.parent != nil {
		return environment.parent.Get(variable)
	} else {
		panic("unbound variable:" + variable) //hu.UnboundVariableError{variable, "get"})
	}
	return nil
}

func (environment *messageEnvironment) AddPrimitive(name string, function hu.PrimitiveFunction) {
	environment.Define(hu.Symbol(name), function)
}

func (e *messageEnvironment) Evaluate(term hu.Term) (result hu.Term) {
	defer func() {
		switch x := recover().(type) {
		case hu.Term:
			result = x
		case interface{}:
			result = hu.Error(fmt.Sprintf("%v", x))
		}
	}()
tailcall:
	switch t := term.(type) {
	case hu.Reducible:
		term = t.Reduce(e)
		goto tailcall
	}
	return term
}

func main() {
	t := os.Getenv("SLACK_TOKEN")
	if t == "" {
		log.Fatal("SLACK_TOKEN not defined")
	}
	r := &Rachael{token: t, in: make(chan Message, 50), out: make(chan Message, 50)}
	//environment := hu.NewEnvironment()
	env := hu.NewEnvironmentWithFrame(&dbframe{})
	hu.AddDefaultBindings(env)
	env.AddPrimitive("sendMessage", r.sendMessage)
	env.AddPrimitive("tell", r.sendMessage)
	env.AddPrimitive("imopen", r.imOpen)
	env.AddPrimitive("HueSetState", hueSetState)
	env.AddPrimitive("turn", hueSetState)
	env.AddPrimitive("in", runIn)
	env.AddPrimitive("at", runAt)
	env.AddPrimitive("schedule", schedule) //env.Define(hu.Symbol("schedule"), hu.PrimitiveFunction(schedule))

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
				var input string
				if strings.HasPrefix(m.Channel, "D") {
					input = m.Text
				} else {
					const prefix = "<@U03V77HBT>: "
					log.Printf("%v\n", strings.HasPrefix(m.Text, prefix))
					if strings.HasPrefix(m.Text, prefix) {
						input = m.Text[len(prefix):]
					}
				}
				if input != "" {
					input = strings.Replace(input, `“`, `"`, -1)
					input = strings.Replace(input, `”`, `"`, -1)
					reader := strings.NewReader(input)
					expression := hu.ReadMessage(reader)
					log.Println(fmt.Sprintf("expression: %#v", expression))
					frame := hu.LocalFrame{}
					frame[hu.Symbol("message")] = m
					frame[hu.Symbol("channel")] = hu.String(m.Channel)
					frame[hu.Symbol("user")] = hu.String(m.User)
					frame[hu.Symbol("text")] = hu.String(m.Text)
					me := &messageEnvironment{frame, env}

					result := me.Evaluate(hu.Application(expression))
					if result != nil {
						r.out <- Message{Id: <-r.ids, Type: "message", Channel: m.Channel, Text: fmt.Sprintf("%v", result)}
					}
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
