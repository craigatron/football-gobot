package main

// LeagueType is the type of league represented by a particular Discord category.
type LeagueType int32

const (
	// LeagueTypeUnknown is an unknown league type.
	LeagueTypeUnknown LeagueType = iota
	// LeagueTypeESPN is an ESPN fantasy football league.
	LeagueTypeESPN
	// LeagueTypeSleeper is a Sleeper fantasy football league.
	LeagueTypeSleeper
)

func (lt LeagueType) String() string {
	switch lt {
	case LeagueTypeESPN:
		return "ESPN"
	case LeagueTypeSleeper:
		return "Sleeper"
	}
	return ""
}
