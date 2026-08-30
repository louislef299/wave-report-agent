package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/surf"
)

func TestNtfyPublishesJSON(t *testing.T) {
	var (
		gotPath   string
		gotAuth   string
		gotType   string
		gotBody   ntfyMessage
		callCount int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("server got unparseable JSON: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &Ntfy{Server: srv.URL, Topic: "test-topic", Token: "tk_secret"}
	err := n.Notify(t.Context(), Notification{
		// An en dash and a middot: exactly the characters that would break if
		// this published through ASCII-only HTTP headers instead of JSON.
		Title:    "Epic surf: Stoney Point, Duluth",
		Body:     "Sun Nov 2 20:00–23:00 · 5.0ft at 6.0s",
		Priority: PriorityUrgent,
		Tags:     []string{"ocean", "fire"},
		ClickURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if callCount != 1 {
		t.Errorf("server saw %d requests, want 1", callCount)
	}
	if gotPath != "/" {
		t.Errorf("published to %q, want / (topic travels in the body)", gotPath)
	}
	if gotType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotType)
	}
	if gotAuth != "Bearer tk_secret" {
		t.Errorf("Authorization = %q, want a bearer token", gotAuth)
	}
	if gotBody.Topic != "test-topic" {
		t.Errorf("topic = %q, want test-topic", gotBody.Topic)
	}
	if !strings.Contains(gotBody.Message, "·") || !strings.Contains(gotBody.Message, "–") {
		t.Errorf("non-ASCII characters did not survive: %q", gotBody.Message)
	}
	if gotBody.Priority != int(PriorityUrgent) {
		t.Errorf("priority = %d, want %d", gotBody.Priority, PriorityUrgent)
	}
	if len(gotBody.Tags) != 2 {
		t.Errorf("tags = %v, want two", gotBody.Tags)
	}
}

func TestNtfyOmitsAuthWhenNoToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	n := &Ntfy{Server: srv.URL, Topic: "test-topic"}
	if err := n.Notify(t.Context(), Notification{Body: "hi"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want it absent on an unauthenticated topic", gotAuth)
	}
}

// TestNtfySurfacesServerError checks that a rejection explains itself. ntfy
// puts the reason in the body, and reporting only the status code turns a
// clear "rate limited" into a guessing game.
func TestNtfySurfacesServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":"visitor request limit reached"}`)
	}))
	defer srv.Close()

	n := &Ntfy{Server: srv.URL, Topic: "test-topic"}
	err := n.Notify(t.Context(), Notification{Body: "hi"})
	if err == nil {
		t.Fatal("expected an error on 429")
	}
	if !strings.Contains(err.Error(), "visitor request limit reached") {
		t.Errorf("error %q should carry the server's explanation", err)
	}
}

func TestNtfyRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n := &Ntfy{Server: srv.URL, Topic: "test-topic"}
	if err := n.Notify(ctx, Notification{Body: "hi"}); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
}

func TestNewNtfyRequiresTopic(t *testing.T) {
	for _, topic := range []string{"", "   "} {
		if _, err := NewNtfy(topic); err == nil {
			t.Errorf("NewNtfy(%q) should fail: without a topic there is nowhere to publish", topic)
		}
	}
}

func TestDiscardIsANotifier(t *testing.T) {
	var n Notifier = Discard{}
	if err := n.Notify(t.Context(), Notification{}); err != nil {
		t.Errorf("Discard should never fail, got %v", err)
	}
}

func TestFromVerdict(t *testing.T) {
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("loading timezone: %v", err)
	}
	start := time.Date(2026, 11, 2, 20, 0, 0, 0, time.UTC)

	testCases := []struct {
		name         string
		verdict      surf.Verdict
		wantPriority Priority
		wantInTitle  []string
		wantInBody   []string
	}{
		{
			name: "epic carries the window and the build",
			verdict: surf.Verdict{
				Spot: "Stoney Point", Board: surf.Longboard, Rating: surf.Epic,
				Windows: []surf.Window{{
					Start: start, End: start.Add(3 * time.Hour),
					PeakWaveFt: 5.0, PeakPeriodS: 6.0, SustainedHours: 16, Rating: surf.Epic,
				}},
				Reasons: []string{"everything lines up"},
			},
			wantPriority: PriorityUrgent,
			wantInTitle:  []string{"Epic", "Stoney Point", "Duluth", "longboard"},
			wantInBody:   []string{"5.0ft", "6.0s", "16h", "everything lines up"},
		},
		{
			name: "poor still explains itself",
			verdict: surf.Verdict{
				Spot: "Stoney Point", Board: surf.Shortboard, Rating: surf.Poor,
				Reasons: []string{"period 3.8s is below the 4.5s floor"},
			},
			wantPriority: PriorityLow,
			wantInTitle:  []string{"Poor", "shortboard"},
			wantInBody:   []string{"period 3.8s is below"},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			n := FromVerdict(tt.verdict, "Duluth", chicago)

			if n.Priority != tt.wantPriority {
				t.Errorf("priority = %d, want %d", n.Priority, tt.wantPriority)
			}
			for _, want := range tt.wantInTitle {
				if !strings.Contains(n.Title, want) {
					t.Errorf("title %q missing %q", n.Title, want)
				}
			}
			for _, want := range tt.wantInBody {
				if !strings.Contains(n.Body, want) {
					t.Errorf("body %q missing %q", n.Body, want)
				}
			}
		})
	}
}

// TestFromVerdictUsesLocalTime guards the detail that decides whether you show
// up at the right hour. The domain stores UTC; a session window quoted in UTC
// would be five hours off in Minnesota.
func TestFromVerdictUsesLocalTime(t *testing.T) {
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("loading timezone: %v", err)
	}

	// 02:00 UTC on Nov 3 is 20:00 CST on Nov 2.
	start := time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)
	v := surf.Verdict{
		Spot: "Stoney Point", Board: surf.Longboard, Rating: surf.Good,
		Windows: []surf.Window{{
			Start: start, End: start.Add(2 * time.Hour),
			PeakWaveFt: 4, PeakPeriodS: 5.5, SustainedHours: 10, Rating: surf.Good,
		}},
	}

	n := FromVerdict(v, "Duluth", chicago)
	if !strings.Contains(n.Body, "20:00") {
		t.Errorf("body %q should quote 20:00 local, not the UTC hour", n.Body)
	}
	if !strings.Contains(n.Body, "Nov 2") {
		t.Errorf("body %q should land on Nov 2 local, not Nov 3 UTC", n.Body)
	}
}

func TestFromVerdictNilLocation(t *testing.T) {
	v := surf.Verdict{Spot: "Stoney Point", Board: surf.Longboard, Rating: surf.Poor}
	// Must not panic; UTC is a reasonable fallback.
	if n := FromVerdict(v, "", nil); n.Title == "" {
		t.Error("expected a title even with no location")
	}
}
