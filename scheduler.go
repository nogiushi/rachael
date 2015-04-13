package main

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eikeon/hu"
)

type Event struct {
	When   string
	What   string
	Repeat string
	On     time.Time
}

func (e *Event) init() {
	if e.On.IsZero() {
		now := time.Now()
		zone, _ := now.Zone()

		if on, err := time.Parse("2006-01-02 "+time.Kitchen+" MST", now.Format("2006-01-02 ")+e.When+" "+zone); err != nil {
			log.Println("could not parse when of '" + e.When + "' for " + e.What)
		} else {
			e.On = on
		}
		e.On = e.next()
	}
}

func (e *Event) duration() time.Duration {
	if d, err := time.ParseDuration(e.Repeat); err != nil {
		log.Println("could not parse repeat of '" + e.Repeat + "' for " + e.What)
		return 60 * 60 * 24 * time.Second
	} else {
		return d
	}
}

func (e *Event) next() time.Time {
	duration := e.duration()
	t := e.On
	for {
		wait := time.Duration(t.UnixNano() - time.Now().UnixNano())
		if wait > 0 {
			break
		}
		if e.Repeat == "daily" {
			t = t.AddDate(0, 0, 1)
		} else {
			t = t.Add(duration)
		}
	}
	log.Println("next '" + e.What + "' on: " + t.String())
	return t
}

type Schedule []Event

func (s Schedule) String() string {
	var events []string
	for i := 0; i < len(s); i++ {
		events = append(events, fmt.Sprintf("%d) %s %s", i+1, s[i].When, s[i].What))
	}
	return strings.Join(events, "\n")
}

func (s Schedule) Len() int { return len(s) }

func (s Schedule) Less(i, j int) bool {
	return s[i].On.UnixNano() < s[j].On.UnixNano()
}

func (s Schedule) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type Scheduler struct {
	In   chan<- Event
	Out  <-chan Event
	Stop chan<- bool
	S    Schedule
}

func (scheduler *Scheduler) String() string {
	return fmt.Sprintf("schedule: \n %s", scheduler.S)
}

func (s *Scheduler) Run(environment hu.Environment) {
	in := make(chan Event, 10)
	out := make(chan Event, 10)
	stop := make(chan bool, 1)
	s.In = in
	s.Out = out
	s.Stop = stop

	sort.Sort(s.S)

	timer := time.NewTimer(0)
	go func() {
		for {
			select {
			case v := <-stop:
				if v {
					timer.Stop()
				} else {
					timer.Reset(0)
				}
			case e := <-in:
				e.init()
				s.S = append(s.S, e)
				sort.Sort(s.S)
				environment.Set(hu.Symbol("scheduler"), s)
				timer.Reset(0)
			case <-timer.C:
				//log.Println("tick", s.S)
				if s.S.Len() > 0 {
					e := s.S[0]
					now := time.Now()
					d := time.Duration(e.On.UnixNano() - now.UnixNano())
					log.Println("next event: ", e, "in: ", d)
					if d <= 0 {
						log.Println(e.What + " at " + now.String())
						out <- e

						if s.S[0].Repeat != "" {
							s.S[0].On = e.next()
							sort.Sort(s.S)
						} else {
							i := 0
							s.S = append(s.S[:i], s.S[i+1:]...) // remove event at index i
						}
						environment.Set(hu.Symbol("scheduler"), s)
						timer.Reset(0)
					} else {
						timer.Reset(d)
					}
				} else {
					timer.Reset(time.Hour)
				}
			}
		}
	}()
	go func() {
		for e := range s.Out {
			input := e.What
			input = strings.Replace(input, `“`, `"`, -1)
			input = strings.Replace(input, `”`, `"`, -1)
			hu.GuardedEvaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("receiveMessage"), hu.String(DEV), hu.String("Scheduler"), hu.String(input)})))
			channel, ok := environment.Get(hu.Symbol("channel"))
			if !ok {
				channel = hu.String(DEV)
			}
			hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), channel, hu.String(fmt.Sprintf("running scheduled `%s`", input))})))
		}
	}()
}

func (s *Scheduler) runAt(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	when := hu.Evaluate(environment, terms[0]).(hu.Term).String()
	what := terms[1].(hu.Term).String()

	var repeat string
	if len(terms) > 2 {
		repeat = hu.Evaluate(environment, terms[2]).(hu.Term).String()
	}

	channel, ok := environment.Get(hu.Symbol("channel"))
	if !ok {
		channel = hu.String(DEV)
	}
	e := Event{When: when, What: what, Repeat: repeat}
	e.init()

	hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), channel, hu.String(fmt.Sprintf("scheduled `%s` to run At %s", what, e.On.Format("Monday, January 2, 3:04pm")))})))
	s.In <- e
	return nil
}

func (s *Scheduler) runIn(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)

	d := hu.Evaluate(environment, terms[0]).(hu.Term).String()
	wait, err := time.ParseDuration(d)
	if err != nil {
		log.Println("err: ", err)
		return nil
	}
	on := time.Now().Add(wait)

	what := terms[1].(hu.Term).String()

	var repeat string
	if len(terms) > 2 {
		repeat = hu.Evaluate(environment, terms[2]).(hu.Term).String()
	}

	channel, ok := environment.Get(hu.Symbol("channel"))
	if !ok {
		channel = hu.String(DEV)
	}
	e := Event{On: on, What: what, Repeat: repeat}
	e.init()

	hu.Evaluate(environment, hu.Application(hu.Tuple([]hu.Term{hu.Symbol("sendMessage"), channel, hu.String(fmt.Sprintf("scheduled `%s` to run At %s", what, e.On.Format("Monday, January 2, 3:04pm")))})))
	s.In <- e
	return nil
}

func (s *Scheduler) stop(environment hu.Environment, term hu.Term) hu.Term {
	s.Stop <- true
	return nil
}

func (s *Scheduler) start(environment hu.Environment, term hu.Term) hu.Term {
	s.Stop <- false
	return nil
}

func (s *Scheduler) remove(environment hu.Environment, term hu.Term) hu.Term {
	terms := term.(hu.Tuple)
	which := hu.Evaluate(environment, terms[0]).(*hu.Number)
	i1, err := strconv.Atoi(which.String())
	if err != nil {
		return hu.Error(err.Error())
	}

	i := i1 - 1
	s.S = append(s.S[:i], s.S[i+1:]...) // remove event at index i
	environment.Set(hu.Symbol("scheduler"), s)

	return s.schedule(environment, nil)
}

func (sr *Scheduler) schedule(environment hu.Environment, term hu.Term) hu.Term {
	s := sr.S
	buffer := bytes.NewBufferString("")
	heading := ""
	for i := 0; i < len(s); i++ {
		e := s[i]
		new_heading := e.On.Format("Monday, January 2")
		if new_heading != heading {
			heading = new_heading
			fmt.Fprintf(buffer, "*%s*\n", heading)
		}

		fmt.Fprintf(buffer, "  %d) %s ", i+1, e.On.Format("3:04pm"))
		if e.Repeat != "" {
			fmt.Fprintf(buffer, "*%s* ", e.Repeat)
		}
		fmt.Fprintf(buffer, "`%s`\n", e.What)
	}
	return hu.String(buffer.String())
}
