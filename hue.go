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

func hueSetState(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	address := hu.Evaluate(environment, terms[0])
	value := hu.Evaluate(environment, terms[1])
	log.Printf("hueSetState: %#v: %v", address, value)

	channel, _ := environment.Get(hu.Symbol("channel"))
	hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), channel, hu.String(fmt.Sprintf("Setting lights %s to %s", address, value))})))

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
				hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), hu.String(DEV), hu.String(text)})))
				time.Sleep(time.Second)
			}
		} else {
			return nil
		}
	}
	return nil
	//return &Number{result}
}
