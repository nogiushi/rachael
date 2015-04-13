package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/eikeon/hu"
	"github.com/eikeon/hue"
)

type Hue struct {
	Hue *hue.Hue
}

func NewHue(environment hu.Environment) *Hue {
	h := &hue.Hue{}
	username, ok := environment.Get(hu.Symbol("hue_username"))
	if ok {
		h.Username = username.(hu.String).String()
	}
	return &Hue{Hue: h}
}

func (h *Hue) SetState(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	address := hu.Evaluate(environment, terms[0])
	value := hu.Evaluate(environment, terms[1])

	var state map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(value.String()))
	if err := dec.Decode(&state); err != nil {
		log.Println("hue decode err:", err)
	}

	retry := 0
retry:
	text := fmt.Sprintf("Set Hue address `%s` to state `%s`", address, value)
	err := h.Hue.Set(address.String(), &state)
	if err != nil {
		log.Println("error:", err)
		text = fmt.Sprintf("error: `%s` while trying to %s", err.Error(), text)
		if retry < 5 {
			time.Sleep(time.Second)
			retry += 1
			goto retry
		}
	}
	channel, _ := environment.Get(hu.Symbol("channel"))
	hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), channel, hu.String(text)})))

	return nil
}

// TODO createUser
// if err := h.Hue.CreateUser(h.Hue.Username, "Marvin"); err == nil {
// 	log.Println("h:", h)
// } else {
// 	text := fmt.Sprintf("%s: press hue link button to authenticate", err)
// 	log.Println(text)
// 	hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), hu.String(DEV), hu.String(text)})))
// 	time.Sleep(time.Second)
// }
