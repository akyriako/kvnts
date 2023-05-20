package sinks

import (
	"context"
	"fmt"
)

type SinkType string

const (
	SinkTypeSlack      = SinkType("slack")
	SinkTypeMattermost = SinkType("mattermost")
	SinkTypeTeams      = SinkType("microsoft_teams")
)

var (
	sinkTypes []string
	cache     *Cache
)

type Sink interface {
	ForwardEvent(payload *Payload) error
	init(ctx context.Context)
}

func init() {
	sinkTypes = []string{
		string(SinkTypeSlack),
		string(SinkTypeMattermost),
		string(SinkTypeTeams),
	}

	cache = NewCache()
}

func indexOf(sinkType string) int {
	for index := range sinkTypes {
		if sinkTypes[index] == sinkType {
			return index
		}
	}

	return -1
}

func NewSink(ctx context.Context, namespacedName string, sinkType SinkType, config interface{}) (Sink, error) {
	if indexOf(string(sinkType)) < 0 {
		return nil, fmt.Errorf("sink %s not implemented", sinkType)
	}

	var sink Sink
	var err error

	switch sinkType {
	case SinkTypeSlack:
		slackConfig, ok := config.(SlackConfig)
		if !ok {
			return nil, fmt.Errorf("faild to cast as slack.SlackConfig")
		}

		if cache.Contains(namespacedName) {
			sink, err = cache.Get(namespacedName)
			if err != nil {
				return nil, err
			}

			cached, _ := sink.(*Slack)
			if cached.config == slackConfig {
				return sink, nil
			}

			cache.Remove(namespacedName)
		}

		sink, err = NewSlackSink(ctx, slackConfig)
		err := cache.Add(namespacedName, sink, cacheTTL)
		if err != nil {
			return nil, err
		}

		return sink, nil
	case SinkTypeMattermost:
		mattermostConfig, ok := config.(MattermostConfig)
		if !ok {
			return nil, fmt.Errorf("faild to cast as mattermost.MattermostConfig")
		}

		if cache.Contains(namespacedName) {
			sink, err = cache.Get(namespacedName)
			if err != nil {
				return nil, err
			}

			cached, _ := sink.(*Mattermost)
			if cached.config == mattermostConfig {
				return sink, nil
			}

			cache.Remove(namespacedName)
		}

		sink, err = NewMattermostSink(ctx, mattermostConfig)
		err := cache.Add(namespacedName, sink, cacheTTL)
		if err != nil {
			return nil, err
		}

		return sink, nil
	}

	return nil, fmt.Errorf("faild to find appropriate sink provider")
}

func ForwardEvent(ctx context.Context, namespacedName string, sinkType SinkType, config interface{}, payload *Payload) error {
	sink, err := NewSink(ctx, namespacedName, sinkType, config)
	if err != nil {
		return err
	}

	err = sink.ForwardEvent(payload)
	if err != nil {
		return err
	}

	return nil
}
