package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/ledger"
	"github.com/louislef299/wave-report-agent/pkg/notify"
	"github.com/louislef299/wave-report-agent/pkg/spot"
	"github.com/louislef299/wave-report-agent/pkg/surf"
)

// Environment holding the ntfy configuration. The topic name is the only thing
// protecting a public ntfy channel, so it stays out of the flag set and out of
// shell history.
const (
	envNtfyTopic  = "NTFY_TOPIC"
	envNtfyToken  = "NTFY_TOKEN"
	envNtfyServer = "NTFY_SERVER"
)

// buildNotifier returns the transport to alert through. On a dry run it prints
// what would have been sent instead of sending it, so the message can be
// reviewed without a topic configured.
func buildNotifier(dryRun bool, out *os.File) (notify.Notifier, error) {
	if dryRun {
		return notify.Func(func(_ context.Context, n notify.Notification) error {
			fmt.Fprintf(out, "\n--- would send (priority %d, tags %v) ---\n%s\n%s\n",
				n.Priority, n.Tags, n.Title, n.Body)
			return nil
		}), nil
	}

	topic := os.Getenv(envNtfyTopic)
	if topic == "" {
		return nil, fmt.Errorf("-notify needs %s set (see https://ntfy.sh)", envNtfyTopic)
	}

	n, err := notify.NewNtfy(topic)
	if err != nil {
		return nil, err
	}
	n.Token = os.Getenv(envNtfyToken)
	if server := os.Getenv(envNtfyServer); server != "" {
		n.Server = server
	}
	return n, nil
}

// alertOpts controls which decisions are worth interrupting someone for.
type alertOpts struct {
	MinRating surf.Rating
	Cooldown  time.Duration
	Location  *time.Location
}

// sendAlerts notifies for the qualifying decisions of a run.
//
// It works from what the ledger stored rather than from the in-memory
// verdicts, so an alert can never go out for a decision that failed to
// persist: there would be no row to mark, and the next run would send it
// again.
func sendAlerts(
	ctx context.Context,
	l *ledger.Ledger,
	n notify.Notifier,
	runID int64,
	spots map[string]spot.Spot,
	verdicts map[string]surf.Verdict,
	opts alertOpts,
	out *os.File,
) error {
	decisions, err := l.DecisionsForRun(ctx, runID)
	if err != nil {
		return err
	}

	for _, d := range decisions {
		rating, err := surf.ParseRating(d.Rating)
		if err != nil {
			return fmt.Errorf("decision %d: %w", d.ID, err)
		}
		if !rating.AtLeast(opts.MinRating) {
			continue
		}

		recent, err := l.NotifiedSince(ctx, d.Spot, d.Board, d.StartedAt.Add(-opts.Cooldown))
		if err != nil {
			return err
		}
		if recent {
			fmt.Fprintf(out, "  skipping %s/%s: already alerted within %s\n",
				d.Spot, d.Board, opts.Cooldown)
			continue
		}

		v, ok := verdicts[verdictKey(d.Spot, d.Board)]
		if !ok {
			return fmt.Errorf("no verdict in memory for decision %d (%s/%s)", d.ID, d.Spot, d.Board)
		}

		msg := notify.FromVerdict(v, spots[d.Spot].City, opts.Location)
		if err := n.Notify(ctx, msg); err != nil {
			// A failed send must not mark the decision notified, or the alert
			// is lost silently. Report it and let the next run retry.
			fmt.Fprintf(os.Stderr, "surfcheck: alerting %s/%s: %v\n", d.Spot, d.Board, err)
			continue
		}
		if err := l.MarkNotified(ctx, d.ID); err != nil {
			return err
		}
		fmt.Fprintf(out, "  alerted %s/%s (%s)\n", d.Spot, d.Board, d.Rating)
	}
	return nil
}

// previewAlerts shows what a run would send. A dry run has no recorded run to
// check history against, so the cooldown is skipped and the output says so.
func previewAlerts(ctx context.Context, n notify.Notifier, runs []ledger.SpotRun, opts alertOpts, out *os.File) error {
	for _, sr := range runs {
		for _, v := range sr.Verdicts {
			if !v.Rating.AtLeast(opts.MinRating) {
				continue
			}
			if err := n.Notify(ctx, notify.FromVerdict(v, sr.Spot.City, opts.Location)); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(out, "\n(dry run: cooldown not applied — no run recorded to compare against)\n")
	return nil
}

func verdictKey(spotName, board string) string { return spotName + "\x00" + board }
