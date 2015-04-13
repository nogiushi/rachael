package main

import (
	"time"

	"github.com/eikeon/hu"
)

func weekday(environment hu.Environment) hu.Term {
	wd := time.Now().Weekday()
	return hu.Boolean((0 < wd) && (wd < 6))
}

func weekend(environment hu.Environment) hu.Term {
	wd := time.Now().Weekday()
	return hu.Boolean((0 == wd) || (wd == 6))
}
