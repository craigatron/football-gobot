package main

// LeagueType is the type of league represented by a particular Discord category.
type LeagueType int32

const (
	// LeagueTypeESPN is an ESPN fantasy football league.
	LeagueTypeESPN LeagueType = iota
	// LeagueTypeSleeper is a Sleeper fantasy football league.
	LeagueTypeSleeper
)
