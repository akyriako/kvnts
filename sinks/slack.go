package sinks

import (
	"context"
	"fmt"
	"github.com/akyriako/kvnts/openai"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"time"
)

var (
	slackApiRateLimit = 1 * time.Second
)

type SlackConfig struct {
	BotToken      string `json:"botToken"`
	ChannelID     string `json:"channelID"`
	AppLevelToken string `json:"appLevelToken"`
	Debug         bool   `json:"debug,omitempty"`
}

type Slack struct {
	client   *slack.Client
	socket   *socketmode.Client
	config   SlackConfig
	SinkType SinkType
	logger   logr.Logger
}

func (s *Slack) ForwardEvent(payload *Payload) error {
	dividerSection := slack.NewDividerBlock()

	headerText := fmt.Sprintf("🔔Cluster: *%s*, Type: *%s*, Reason: *%s*, Kind: *%s* \n\n 🚦*Alert:* %s", payload.CommonLabels["cluster_name"], payload.Level, payload.ExtraLabels["reason"], payload.ExtraLabels["kind"], payload.Note)
	headerTextBlock := slack.NewTextBlockObject("mrkdwn", headerText, false, false)
	headerSection := slack.NewSectionBlock(headerTextBlock, nil, nil)

	podText := fmt.Sprintf("• *namespace:* %s\n• *pod:* %s", payload.ExtraLabels["namespace"], payload.ExtraLabels["pod"])
	podTextBlock := slack.NewTextBlockObject("mrkdwn", podText, false, false)
	podSectionBlock := slack.NewSectionBlock(podTextBlock, nil, nil)

	timeStampText := fmt.Sprintf("🔛 *First seen:* %s\n 🔚 *Last Seen:* %s", payload.FirstSeen, payload.LastSeen)
	timeStampTextBlock := slack.NewTextBlockObject("mrkdwn", timeStampText, false, false)
	timeStampSectionBlock := slack.NewSectionBlock(timeStampTextBlock, nil, nil)

	//askGptButtonTxt := slack.NewTextBlockObject("plain_text", "Ask ChatGPT", false, false)
	//askGptButton := slack.NewButtonBlockElement("", payload.Note, askGptButtonTxt)
	//actionBlock := slack.NewActionBlock("", askGptButton)

	msgOptionBlocks := slack.MsgOptionBlocks(
		dividerSection,
		headerSection,
		podSectionBlock,
		timeStampSectionBlock,
		//actionBlock,
	)

	chatGptAttachment := slack.Attachment{
		Pretext:    "🆘 *Use OpenAI Chat API  to analyse the Event and suggest you a course of action:*",
		Fallback:   "Your client is not supported",
		CallbackID: "askGPT",
		Color:      "#3AA3E3",
		Actions: []slack.AttachmentAction{
			slack.AttachmentAction{
				Name:  "askgpt_action",
				Text:  "💬 Ask ChatGPT for help",
				Type:  "button",
				Value: payload.Note,
				Style: "primary",
			},
		},
	}

	attachments := slack.MsgOptionAttachments(chatGptAttachment)

	_, _, err := s.client.PostMessage(
		s.config.ChannelID,
		msgOptionBlocks,
		attachments,
		slack.MsgOptionAsUser(true))
	if err != nil {
		return err
	}

	if strings.TrimSpace(payload.Logs) != "" {
		filename := fmt.Sprintf("%s/%s.log", payload.ExtraLabels["namespace"], payload.ExtraLabels["pod"])
		params := slack.FileUploadParameters{
			Title:          filename,
			Filename:       filename,
			Filetype:       "log",
			Content:        payload.Logs,
			Channels:       []string{s.config.ChannelID},
			InitialComment: filename,
		}
		_, err = s.client.UploadFile(params)
		if err != nil {
			return err
		}
	}

	time.Sleep(slackApiRateLimit)

	return nil
}

func NewSlackSink(ctx context.Context, config SlackConfig) (*Slack, error) {
	if !strings.HasPrefix(config.BotToken, "xoxb-") {
		return nil, fmt.Errorf("no valid bot token")
	}

	if !strings.HasPrefix(config.AppLevelToken, "xapp-") {
		return nil, fmt.Errorf("no valid app level token")
	}

	slackSink := &Slack{
		config:   config,
		SinkType: SinkTypeSlack,
	}

	slackSink.init(ctx)

	return slackSink, nil
}

func (s *Slack) init(ctx context.Context) {
	s.logger = ctrl.Log.WithName(string(s.SinkType))

	s.client = slack.New(
		s.config.BotToken,
		slack.OptionDebug(s.config.Debug),
		slack.OptionAppLevelToken(s.config.AppLevelToken),
	)

	s.socket = socketmode.New(s.client, socketmode.OptionDebug(s.config.Debug))
	go s.handlerSocketMode(ctx)
}

func (s *Slack) handlerSocketMode(ctx context.Context) {
	go s.listenSocketEvents(ctx)
	s.socket.RunContext(ctx)
}

func (s *Slack) listenSocketEvents(ctx context.Context) {
	defer s.logger.Info("closed slack socketmode listener")
	s.logger.Info("starting slack socketmode listener")

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("shutting down slack socketmode listener")
			return
		case event := <-s.socket.Events:
			switch event.Type {
			case socketmode.EventTypeInteractive:
				interactionCallback, ok := event.Data.(slack.InteractionCallback)
				if !ok {
					s.logger.Info("could not typecast event.Data to: %t\n", interactionCallback)
					continue
				}
				s.socket.Ack(*event.Request)

				prompt := interactionCallback.ActionCallback.AttachmentActions[0].Value
				response, err := s.getChatGptResponse(ctx, prompt)
				if err != nil {
					s.logger.Error(err, "failed to complete with chatgpt")
					response = fmt.Sprintf("⚠ %s", err.Error())
				}

				if strings.TrimSpace(response) != "" {
					err := s.postChatGptResponse(prompt, response)
					if err != nil {
						s.logger.Error(err, "failed to send chatgpt response back to channel")
					}
				}
			}
		}
	}
}

func (s *Slack) getChatGptResponse(ctx context.Context, prompt string) (string, error) {
	client, err := openai.NewClientFromEnviroment()
	if err != nil {
		return "", errors.Wrap(err, "failed to get an openai client")
	}

	completion, err := client.CreateChatCompletion(ctx, prompt)
	if err != nil {
		return "", err
	}

	return completion, nil
}

func (s *Slack) postChatGptResponse(prompt string, response string) error {
	headerText := fmt.Sprintf("🤖 *ChatGPT response for the Event:* \n\n🚦 %s :", prompt)
	headerTextBlock := slack.NewTextBlockObject("mrkdwn", headerText, false, false)
	headerSection := slack.NewSectionBlock(headerTextBlock, nil, nil)

	chatGptTextBlock := slack.NewTextBlockObject("mrkdwn", response, false, false)
	chatGptSection := slack.NewSectionBlock(chatGptTextBlock, nil, nil)

	dividerSection := slack.NewDividerBlock()

	msgOption := slack.MsgOptionBlocks(
		headerSection,
		chatGptSection,
		dividerSection,
	)

	_, _, err := s.client.PostMessage(
		s.config.ChannelID,
		msgOption,
		slack.MsgOptionAsUser(true), // Add this if you want that the bot would post message as a user, otherwise it will send response using the default slackbot
	)
	if err != nil {
		return err
	}

	return nil
}
