package main

import (
	"fmt"
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
// 		result := hu.Evaluate(environment, expression)
// 		if result != nil {
// 			r.out <- Message{Id: <-r.ids, Type: "message", Channel: DEV, Text: fmt.Sprintf("%v", result)}
// 		}
// 	}
// }()

func schedule(environment hu.Environment, term hu.Term) hu.Term {
	log.Println(fmt.Sprintf("ee: %p", environment))
	terms := term.(hu.Tuple)
	when := hu.Evaluate(environment, terms[0]).(hu.Term).String()
	what := hu.Evaluate(environment, terms[1]).(hu.Term).String()
	interval := hu.Evaluate(environment, terms[2]).(hu.Term).String()
	e := scheduler.Event{When: when, What: what, Interval: interval}
	v, _ := environment.Get("scheduler")
	s := v.(*scheduler.Scheduler)
	hu.Evaluate(environment, hu.Application([]hu.Term{hu.Symbol("schedule+"), terms}))
	sc := scheduler.Schedule{}
	sc = append(sc, &e)
	s.In <- sc
	log.Println("??")
	ns, _ := environment.Get("_schedule")
	for _, eterms := range ns.(hu.Tuple) {
		et := eterms.(hu.Tuple)
		when := hu.Evaluate(environment, et[0]).(hu.Term).String()
		what := hu.Evaluate(environment, et[1]).(hu.Term).String()
		interval := hu.Evaluate(environment, et[2]).(hu.Term).String()
		e := scheduler.Event{When: when, What: what, Interval: interval}
		sc = append(sc, &e)
	}
	return ns
}
