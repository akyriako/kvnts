package sinks

import "time"

type Payload struct {
	Level        string
	Note         string
	CommonLabels map[string]string
	ExtraLabels  map[string]string
	FirstSeen    time.Time
	LastSeen     time.Time
	Logs         string
}

func NewPayload(level string, note string, commonLabels map[string]string, extraLabels map[string]string, firstSeen time.Time, lastSeen time.Time, logs string) *Payload {
	return &Payload{
		level,
		note,
		commonLabels,
		extraLabels,
		firstSeen,
		lastSeen,
		logs,
	}
}
