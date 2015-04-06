package main

import (
	"fmt"
	"log"
	"time"

	"github.com/eikeon/hu"
)

func runIn(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	d := environment.Evaluate(terms[0]).(hu.Term).String()
	action := terms[1]
	wait, err := time.ParseDuration(d)
	if err != nil {
		log.Println("err: ", err)
		return nil
	}
	t := time.Now().Add(wait)
	return runAtTime(environment, action, t)
}

func runAt(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	when := environment.Evaluate(terms[0]).(hu.Term).String()
	action := terms[1]
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

	t := now.Add(wait)
	return runAtTime(environment, action, t)
}

func runAtTime(environment hu.Environment, application hu.Term, t time.Time) hu.Term {
	log.Println(environment.Evaluate(hu.Symbol("sendMessage")))
	channel := environment.Get(hu.Symbol("channel"))
	log.Println("CHANNEL: ", channel)
	if channel == nil {
		channel = hu.String(DEV)
	}
	environment.Evaluate(hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), channel, hu.String(fmt.Sprintf("scheduled `%s` to run at %s", application, t.Format("Monday, January 2, 3:04pm")))})))
	wait := time.Duration((t.UnixNano() - time.Now().UnixNano()))
	time.AfterFunc(wait, func() {
		environment.Evaluate(hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), channel, hu.String(fmt.Sprintf("As requested running `%s` now", application))})))
		time.Sleep(time.Second)
		environment.Evaluate(application)
	})
	return nil
}
