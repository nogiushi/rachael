package main

import (
	"fmt"
	"strings"

	"github.com/eikeon/hu"
	"github.com/eikeon/scheduler"
)

type Scheduler struct {
	scheduler scheduler.Scheduler
}

func (s *Scheduler) Run(environment hu.Environment) {
	go func() {
		for e := range s.scheduler.Out {
			input := e.What
			input = strings.Replace(input, `“`, `"`, -1)
			input = strings.Replace(input, `”`, `"`, -1)
			hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("receiveMessage"), hu.String(DEV), hu.String("Scheduler"), hu.String(input)})))
			channel, _ := environment.Get(hu.Symbol("channel"))
			if channel == nil {
				channel = hu.String(DEV)
			}
			hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), channel, hu.String(fmt.Sprintf("running scheduled `%s` to run at %s", input))})))
		}
	}()
	s.scheduler.Run()

}

func (s *Scheduler) update(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	ns := hu.Evaluate(environment, terms[0])
	sc := scheduler.Schedule{}
	for _, eterms := range ns.(hu.Tuple) {
		et := eterms.(hu.Tuple)
		when := hu.Evaluate(environment, et[0]).(hu.Term).String()
		what := hu.Evaluate(environment, et[1]).(hu.Term).String()
		interval := hu.Evaluate(environment, et[2]).(hu.Term).String()
		e := scheduler.Event{When: when, What: what, Interval: interval}
		sc = append(sc, &e)
	}
	s.scheduler.In <- sc
	return ns
}

func (s *Scheduler) scheduleAdd(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	when := hu.Evaluate(environment, terms[0]).(hu.Term).String()
	what := hu.Evaluate(environment, terms[1]).(hu.Term).String()
	interval := hu.Evaluate(environment, terms[2]).(hu.Term).String()
	e := scheduler.Event{When: when, What: what, Interval: interval}
	hu.Evaluate(environment, hu.Application([]hu.Term{hu.Symbol("schedule+"), terms}))
	sc := scheduler.Schedule{}
	sc = append(sc, &e)
	s.scheduler.In <- sc
	ns, _ := environment.Get("schedule")
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
