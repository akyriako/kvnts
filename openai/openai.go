package openai

import (
	"context"
	"fmt"
	goopenai "github.com/sashabaranov/go-openai"
	"os"
)

var client *Client

type Client struct {
	*goopenai.Client
}

func NewClient(authToken string) *Client {
	if client == nil {
		client = &Client{
			Client: goopenai.NewClient(authToken),
		}
	}

	return client
}

func NewClientFromEnviroment() (*Client, error) {
	authToken, found := os.LookupEnv("OPENAI_API_KEY")
	if !found {
		return nil, fmt.Errorf("unable to find env variable OPENAI_API_KEY")
	}
	return NewClient(authToken), nil
}

func (o *Client) CreateChatCompletion(ctx context.Context, prompt string) (string, error) {
	response, err := o.Client.CreateChatCompletion(
		ctx,
		goopenai.ChatCompletionRequest{
			Model: goopenai.GPT3Dot5Turbo,
			Messages: []goopenai.ChatCompletionMessage{
				{
					Role:    goopenai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return response.Choices[0].Message.Content, nil
}
