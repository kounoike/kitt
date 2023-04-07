package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/livekit/protocol/logger"
	openai "github.com/sashabaranov/go-openai"
)

type ChatCompletion struct {
	client   *openai.Client
	language *Language
}

func NewChatCompletion(client *openai.Client, language *Language) *ChatCompletion {
	return &ChatCompletion{
		client:   client,
		language: language,
	}
}

func (c *ChatCompletion) Complete(ctx context.Context, history []*Sentence, prompt *Sentence) (*ChatStream, error) {
	var sb strings.Builder
	for _, s := range history {
		if s.IsBot {
			sb.WriteString(fmt.Sprintf("You (%s)", BotName))
		} else {
			sb.WriteString(s.ParticipantName)
		}

		sb.WriteString(" said ")
		sb.WriteString(s.Text)
		sb.WriteString("\n\n")
	}

	conversation := sb.String()
	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: "You are a voice assistant in a meeting named KITT, make concise/short answers." +
					"Always answer in English. Always prepend the language code before answering. e.g: <fr-FR> <sentence in French>",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "History of the conversation:\n" + conversation,
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: fmt.Sprintf("You are talking to %s", prompt.ParticipantName),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt.Text,
			},
		},
		Stream: true,
	})

	if err != nil {
		logger.Errorw("error creating chat completion stream", err)
		return nil, err
	}

	return &ChatStream{
		stream: stream,
	}, nil
}

// Wrapper around openai.ChatCompletionStream to return only complete sentences
type ChatStream struct {
	stream *openai.ChatCompletionStream
}

func (c *ChatStream) Recv() (string, error) {
	sb := strings.Builder{}
	for {
		response, err := c.stream.Recv()
		if err != nil {
			content := sb.String()
			if err == io.EOF && len(strings.TrimSpace(content)) != 0 {
				return content, nil
			}
			return "", err
		}

		delta := response.Choices[0].Delta.Content
		sb.WriteString(delta)

		if strings.HasSuffix(strings.TrimSpace(delta), ".") {
			return sb.String(), nil
		}
	}
}

func (c *ChatStream) Close() {
	c.stream.Close()
}
