package main

import (
	"log"

	"github.com/eikeon/hu"
	"github.com/eikeon/scheduler"
)

// // store in env?
// s := scheduler.Scheduler{}
// s.Run()
// environment.Define("scheduler", &s)
// go func() {
// 	for e := range s.Out {
// 		input := e.What
// 		input = strings.Replace(input, `“`, `"`, -1)
// 		input = strings.Replace(input, `”`, `"`, -1)
// 		reader := strings.NewReader(input)
// 		expression := hu.Read(reader)
// 		result := environment.Evaluate(expression)
// 		if result != nil {
// 			r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: fmt.Sprintf("%v", result)}
// 		}

// 	}
// }()

func schedule(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	when := environment.Evaluate(terms[0]).(hu.Term).String()
	what := environment.Evaluate(terms[1]).(hu.Term).String()
	interval := environment.Evaluate(terms[2]).(hu.Term).String()
	e := scheduler.Event{When: when, What: what, Interval: interval}
	s := environment.Get("scheduler").(*scheduler.Scheduler)
	environment.Evaluate(hu.Application([]hu.Term{hu.Symbol("schedule+"), terms}))
	sc := scheduler.Schedule{}
	sc = append(sc, &e)
	s.In <- sc
	log.Println("??")
	ns := environment.Get("_schedule")
	for _, eterms := range ns.(hu.Tuple) {
		et := eterms.(hu.Tuple)
		when := environment.Evaluate(et[0]).(hu.Term).String()
		what := environment.Evaluate(et[1]).(hu.Term).String()
		interval := environment.Evaluate(et[2]).(hu.Term).String()
		e := scheduler.Event{When: when, What: what, Interval: interval}
		sc = append(sc, &e)
	}
	return ns
}
