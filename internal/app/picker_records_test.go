package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alchemmist/lazy-tmux/internal/config"
	"github.com/alchemmist/lazy-tmux/internal/snapshot"
)

type pickerTestTmux struct {
	liveSessions       []string
	previousSession    string
	previousSessionErr error
}

func (t pickerTestTmux) ListSessions() ([]string, error) {
	return t.liveSessions, nil
}

func (t pickerTestTmux) CurrentSession() (string, error) {
	return "", nil
}

func (t pickerTestTmux) PreviousSession() (string, error) {
	if t.previousSessionErr != nil {
		return "", t.previousSessionErr
	}

	return t.previousSession, nil
}

func (t pickerTestTmux) SessionExists(string) bool {
	return false
}

func (t pickerTestTmux) SocketPath() string {
	return "test"
}

func (t pickerTestTmux) CaptureSession(string) (snapshot.SessionSnapshot, error) {
	return snapshot.SessionSnapshot{}, nil
}

func (t pickerTestTmux) CapturePaneScrollback(string, int) (string, error) {
	return "", nil
}

func (t pickerTestTmux) RestoreSession(snapshot.SessionSnapshot) error {
	return nil
}

func (t pickerTestTmux) SwitchClient(string) error {
	return nil
}

func (t pickerTestTmux) NewSession(string) error {
	return nil
}

func (t pickerTestTmux) NewWindow(string, string) error {
	return nil
}

func (t pickerTestTmux) KillWindow(string, int) error {
	return nil
}

func (t pickerTestTmux) KillSession(string) error {
	return nil
}

func (t pickerTestTmux) RenameWindow(string, int, string) error {
	return nil
}

func (t pickerTestTmux) RenameSession(string, string) error {
	return nil
}

func TestPickerRecordsEmpty(t *testing.T) {
	app := New(config.Config{DataDir: t.TempDir(), TmuxBin: "tmux"})

	_, err := app.pickerRecords(DefaultPickerSortOptions())
	if err == nil {
		t.Fatal("expected error for empty records")
	}

	if !strings.Contains(err.Error(), "no saved sessions found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPickerRecordsSortedByCapturedAt(t *testing.T) {
	app := New(config.Config{DataDir: t.TempDir(), TmuxBin: "tmux"})
	base := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)

	snaps := []snapshot.SessionSnapshot{
		{
			Version:     snapshot.FormatVersion,
			SessionName: "old",
			CapturedAt:  base.Add(-2 * time.Hour),
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
		{
			Version:     snapshot.FormatVersion,
			SessionName: "new",
			CapturedAt:  base.Add(-1 * time.Hour),
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
		{
			Version:     snapshot.FormatVersion,
			SessionName: "latest",
			CapturedAt:  base,
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
	}
	for _, s := range snaps {
		err := app.store.SaveSession(s)
		if err != nil {
			t.Fatalf("save session %q: %v", s.SessionName, err)
		}
	}

	recs, err := app.pickerRecords(DefaultPickerSortOptions())
	if err != nil {
		t.Fatalf("pickerRecords: %v", err)
	}

	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}

	got := []string{recs[0].SessionName, recs[1].SessionName, recs[2].SessionName}
	want := []string{"latest", "new", "old"}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("unexpected order: got %v want %v", got, want)
	}
}

func TestPickerRecordsSortedByLastAccessed(t *testing.T) {
	app := New(config.Config{DataDir: t.TempDir(), TmuxBin: "tmux"})
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	for _, snap := range []snapshot.SessionSnapshot{
		{
			Version:     snapshot.FormatVersion,
			SessionName: "alpha",
			CapturedAt:  base,
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
		{
			Version:     snapshot.FormatVersion,
			SessionName: "beta",
			CapturedAt:  base.Add(1 * time.Hour),
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
	} {
		err := app.store.SaveSession(snap)
		if err != nil {
			t.Fatalf("save session %q: %v", snap.SessionName, err)
		}
	}

	// Access alpha later than beta: alpha should be listed first in picker.
	err := app.store.MarkSessionAccessed("beta", base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("mark beta: %v", err)
	}

	err = app.store.MarkSessionAccessed("alpha", base.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("mark alpha: %v", err)
	}

	recs, err := app.pickerRecords(DefaultPickerSortOptions())
	if err != nil {
		t.Fatalf("pickerRecords: %v", err)
	}

	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}

	if recs[0].SessionName != "alpha" || recs[1].SessionName != "beta" {
		t.Fatalf("unexpected order by last_accessed: %#v", recs)
	}
}

func TestPickerRecordsPromotesPreviousSession(t *testing.T) {
	app := NewWithTmux(
		config.Config{DataDir: t.TempDir(), TmuxBin: "tmux"},
		pickerTestTmux{previousSession: "beta"},
	)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	for _, snap := range []snapshot.SessionSnapshot{
		{
			Version:     snapshot.FormatVersion,
			SessionName: "alpha",
			CapturedAt:  base.Add(2 * time.Hour),
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
		{
			Version:     snapshot.FormatVersion,
			SessionName: "beta",
			CapturedAt:  base,
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
		{
			Version:     snapshot.FormatVersion,
			SessionName: "gamma",
			CapturedAt:  base.Add(time.Hour),
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
	} {
		err := app.store.SaveSession(snap)
		if err != nil {
			t.Fatalf("save session %q: %v", snap.SessionName, err)
		}
	}

	recs, err := app.pickerRecords(DefaultPickerSortOptions())
	if err != nil {
		t.Fatalf("pickerRecords: %v", err)
	}

	got := []string{recs[0].SessionName, recs[1].SessionName, recs[2].SessionName}
	want := []string{"beta", "alpha", "gamma"}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("unexpected promoted order: got %v want %v", got, want)
	}
}

func TestPickerRecordsKeepsSortedOrderWhenPreviousSessionUnavailable(t *testing.T) {
	app := NewWithTmux(
		config.Config{DataDir: t.TempDir(), TmuxBin: "tmux"},
		pickerTestTmux{previousSessionErr: errors.New("no client")},
	)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	for _, snap := range []snapshot.SessionSnapshot{
		{
			Version:     snapshot.FormatVersion,
			SessionName: "alpha",
			CapturedAt:  base.Add(time.Hour),
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
		{
			Version:     snapshot.FormatVersion,
			SessionName: "beta",
			CapturedAt:  base,
			Windows:     []snapshot.Window{{Index: 0, Panes: []snapshot.Pane{{Index: 0}}}},
		},
	} {
		err := app.store.SaveSession(snap)
		if err != nil {
			t.Fatalf("save session %q: %v", snap.SessionName, err)
		}
	}

	recs, err := app.pickerRecords(DefaultPickerSortOptions())
	if err != nil {
		t.Fatalf("pickerRecords: %v", err)
	}

	got := []string{recs[0].SessionName, recs[1].SessionName}
	want := []string{"alpha", "beta"}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("unexpected fallback order: got %v want %v", got, want)
	}
}
