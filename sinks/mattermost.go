package sinks

import "context"

type MattermostConfig struct {
}

type Mattermost struct {
	config MattermostConfig
}

func NewMattermostSink(ctx context.Context, config MattermostConfig) (*Mattermost, error) {
	//TODO implement me
	panic("implement me")
}

func (m *Mattermost) ForwardEvent(payload *Payload) error {
	//TODO implement me
	panic("implement me")
}

func (m *Mattermost) init(ctx context.Context) {
	//TODO implement me
	panic("implement me")
}
