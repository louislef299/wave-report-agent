package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultNtfyServer is the public ntfy instance.
const DefaultNtfyServer = "https://ntfy.sh"

// Ntfy publishes to an ntfy topic.
//
// Note on privacy: on the public server a topic name is the only thing
// standing between your alerts and anyone who guesses it. Topics are not
// enumerable, but they are not secret either — treat the name like a password
// and use a long random one, or self-host and set Token.
type Ntfy struct {
	// Server is the base URL. Defaults to DefaultNtfyServer.
	Server string

	// Topic is the channel to publish to. Required.
	Topic string

	// Token is an optional access token for protected topics.
	Token string

	// Client defaults to a 10s client.
	Client *http.Client
}

// ntfyMessage is ntfy's JSON publishing format.
//
// The alternative is putting the title and tags in HTTP headers, which only
// carry ASCII. Surf summaries contain en dashes and middots, so header
// publishing would mangle or reject them; JSON sidesteps the encoding question
// entirely.
type ntfyMessage struct {
	Topic    string   `json:"topic"`
	Title    string   `json:"title,omitempty"`
	Message  string   `json:"message,omitempty"`
	Priority int      `json:"priority,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Click    string   `json:"click,omitempty"`
}

// NewNtfy builds a notifier for a topic.
func NewNtfy(topic string) (*Ntfy, error) {
	if strings.TrimSpace(topic) == "" {
		return nil, errors.New("ntfy topic is required")
	}
	return &Ntfy{
		Server: DefaultNtfyServer,
		Topic:  topic,
		Client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (n *Ntfy) Notify(ctx context.Context, msg Notification) error {
	server := n.Server
	if server == "" {
		server = DefaultNtfyServer
	}
	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	body, err := json.Marshal(ntfyMessage{
		Topic:    n.Topic,
		Title:    msg.Title,
		Message:  msg.Body,
		Priority: int(msg.Priority),
		Tags:     msg.Tags,
		Click:    msg.ClickURL,
	})
	if err != nil {
		return fmt.Errorf("encoding ntfy message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(server, "/"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building ntfy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if n.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("publishing to ntfy topic %s: %w", n.Topic, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// ntfy explains rejections in the body; surfacing a bare status code
		// would turn a clear "topic is rate limited" into a guessing game.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ntfy returned %s for topic %s: %s",
			resp.Status, n.Topic, strings.TrimSpace(string(detail)))
	}
	return nil
}
