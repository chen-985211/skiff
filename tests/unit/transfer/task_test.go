package transfer_test

import (
	"reflect"
	"strings"
	"testing"

	"skiff/internal/contexts/transfer/domain"
)

func TestBatchAcceptsLinksAndDeduplicatesWithoutChangingSignedQuery(t *testing.T) {
	want := []string{"https://example.com/file.zip?token=a%2Bb", "http://localhost:8123/a"}
	got, err := domain.ParseBatch(" \r\n" + want[0] + "\r\n" + want[1] + "\n" + want[0] + "\n")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
}

func TestBatchRejectsInvalidInputAsAWhole(t *testing.T) {
	for _, value := range []string{"", "file:///tmp/a", "magnet:?xt=abc", "https://", "https://example.com/a b", "https://example.com/good\nftp://example.com/bad", "https://example.com:bad/a"} {
		t.Run(value, func(t *testing.T) {
			got, err := domain.ParseBatch(value)
			if err == nil || len(got) != 0 {
				t.Fatalf("invalid batch accepted: %v, %v", got, err)
			}
		})
	}
	var batch strings.Builder
	for i := 0; i < 101; i++ {
		batch.WriteString("https://example.com/" + strings.Repeat("a", i+1) + "\n")
	}
	if _, err := domain.ParseBatch(batch.String()); err == nil {
		t.Fatal("more than 100 unique links accepted")
	}
}

func TestTaskOwnsActionEligibility(t *testing.T) {
	cases := []struct {
		status  domain.Status
		allowed []domain.Command
	}{
		{domain.Waiting, []domain.Command{domain.Pause, domain.Remove}},
		{domain.Active, []domain.Command{domain.Pause, domain.Remove}},
		{domain.Paused, []domain.Command{domain.Resume, domain.Remove}},
		{domain.Failed, []domain.Command{domain.Retry, domain.Remove}},
		{domain.Complete, []domain.Command{domain.Remove}},
	}
	for _, c := range cases {
		task := domain.Restore(domain.Record{Status: c.status})
		for _, cmd := range []domain.Command{domain.Pause, domain.Resume, domain.Retry, domain.Remove} {
			want := false
			for _, allowed := range c.allowed {
				want = want || cmd == allowed
			}
			if got := task.Allows(cmd); got != want {
				t.Errorf("%s allows %s = %v, want %v", c.status, cmd, got, want)
			}
		}
	}
}

func TestTaskProgressIsBoundedAndCompletionIsAuthoritative(t *testing.T) {
	for _, c := range []struct {
		record domain.Record
		want   float64
	}{
		{domain.Record{Status: domain.Waiting}, 0},
		{domain.Record{Status: domain.Active, Total: 100, Completed: 25}, .25},
		{domain.Record{Status: domain.Active, Total: 100, Completed: 120}, 1},
		{domain.Record{Status: domain.Active, Total: 100, Completed: -1}, 0},
		{domain.Record{Status: domain.Complete}, 1},
	} {
		if got := domain.Restore(c.record).Progress(); got != c.want {
			t.Errorf("%+v progress %v, want %v", c.record, got, c.want)
		}
	}
}

func TestPausedTransferKeepsLastKnownProgressUntilEngineMeasuresAgain(t *testing.T) {
	task := domain.Restore(domain.Record{Status: domain.Paused, Total: 100, Completed: 54})
	for _, status := range []domain.Status{domain.Paused, domain.Waiting, domain.Active} {
		task.Observe(domain.Observation{Status: status})
		if r := task.Record(); r.Status != status || r.Total != 100 || r.Completed != 54 || r.Speed != 0 {
			t.Fatalf("empty %s observation changed progress: %+v", status, r)
		}
	}
	// Once the engine knows the length, even a lower count is authoritative.
	task.Observe(domain.Observation{Status: domain.Active, Total: 120, Completed: 0})
	if r := task.Record(); r.Total != 120 || r.Completed != 0 {
		t.Fatalf("new measurement ignored: %+v", r)
	}
	task.Observe(domain.Observation{Status: domain.Complete})
	if r := task.Record(); r.Total != 0 || r.Completed != 0 || task.Progress() != 1 {
		t.Fatalf("empty completed file kept old counters: %+v", r)
	}
}

func TestNewTaskKeepsStableIdentityAndStartsWaiting(t *testing.T) {
	r := domain.NewTask("stable", "https://example.com/archive.zip", "/downloads").Record()
	if r.ID != "stable" || r.Status != domain.Waiting || r.URL != "https://example.com/archive.zip" || r.Directory != "/downloads" || r.Name != "archive.zip" {
		t.Fatalf("unexpected task %+v", r)
	}
}
