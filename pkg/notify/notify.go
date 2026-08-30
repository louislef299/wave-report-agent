// Package notify delivers surf alerts.
//
// The transport sits behind a one-method interface so that swapping ntfy for
// Slack, email, or anything else touches one file and no calling code. Nothing
// in here knows what a wave is; see alert.go for the mapping from a verdict to
// a message.
package notify

import "context"

// Priority is a transport-agnostic urgency level. Implementations map it onto
// whatever their service understands.
type Priority int

const (
	PriorityLow Priority = iota + 1
	PriorityDefault
	PriorityHigh
	PriorityUrgent
)

// Notification is a message ready to send.
type Notification struct {
	Title string
	Body  string

	Priority Priority

	// Tags are short labels. Some services render known names as emoji.
	Tags []string

	// ClickURL is opened when the notification is tapped. Optional.
	ClickURL string
}

// Notifier delivers a notification. Implementations must respect ctx and
// should be safe for concurrent use.
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

// Discard drops everything. It stands in for a real transport during dry runs
// and in tests, so calling code never needs a nil check.
type Discard struct{}

func (Discard) Notify(context.Context, Notification) error { return nil }

// Func adapts a plain function into a Notifier.
type Func func(ctx context.Context, n Notification) error

func (f Func) Notify(ctx context.Context, n Notification) error { return f(ctx, n) }
