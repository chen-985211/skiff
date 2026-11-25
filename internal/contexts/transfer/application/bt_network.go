package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"time"
)

type BTSettings struct {
	Seed                       SeedPolicy `json:",omitzero"`
	Sources                    string
	DisableDHT, LocalDiscovery bool
	ListenPort, MaxPeers       int
}
type BTSubscription struct {
	URL        string
	Trackers   []string
	Checked    time.Time
	Failure    ErrorCode `json:",omitempty"`
	HTTPStatus int       `json:",omitempty"`
	Error      string    `json:",omitempty"` // Legacy persisted error text.
}

// SubscriptionHTTPError identifies an HTTP rejection without retaining response content.
type SubscriptionHTTPError struct{ Status int }

func (e SubscriptionHTTPError) Error() string {
	return fmt.Sprintf("%s: HTTP status %d", ErrReadSubscription, e.Status)
}

func (SubscriptionHTTPError) Unwrap() error { return ErrReadSubscription }

func (s BTSubscription) FailureReason() error {
	if s.Failure == ErrReadSubscription && s.HTTPStatus != 0 {
		return SubscriptionHTTPError{Status: s.HTTPStatus}
	}
	if s.Failure != "" {
		return s.Failure
	}
	return nil
}

type Peer struct {
	Address, Download, Upload string
	Seeder                    bool
}
type BTDetails struct {
	Peers    []Peer
	Trackers []string
	Bitfield string
	Pieces   int
	Uploaded int64
	Seeder   bool
}
type BTNetwork interface {
	ChangePeerLimit(context.Context, string, int) error
	FetchTrackers(context.Context, string) ([]string, error)
	ReadTorrentDetails(context.Context, string, string) (BTDetails, error)
}

func (b BTSettings) Validate() error {
	if b.Seed.Minutes < 0 || b.Seed.Ratio < 0 || math.IsNaN(b.Seed.Ratio) || math.IsInf(b.Seed.Ratio, 0) {
		return ErrInvalidSeedPolicy
	}
	if b.ListenPort < 0 || b.ListenPort > 65535 || b.MaxPeers < 0 || b.MaxPeers > 65535 {
		return ErrBTPortRange
	}
	for _, src := range strings.Fields(b.Sources) {
		u, err := url.Parse(src)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return ErrBadSubscriptionURL
		}
	}
	return nil
}
func unique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range values {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
func (s *Service) SetBTSettings(ctx context.Context, b BTSettings) error {
	if err := b.Validate(); err != nil {
		return err
	}
	b.Sources = strings.Join(unique(strings.Fields(b.Sources)), "\n")
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.preferences
	next.BT = b
	return s.savePreferences(ctx, next)
}
func (s *Service) btSources() []BTSubscription {
	var out []BTSubscription
	for _, u := range strings.Fields(s.preferences.BT.Sources) {
		v := s.subscriptions[u]
		v.URL = u
		v.Trackers = slices.Clone(v.Trackers)
		out = append(out, v)
	}
	return out
}
func (s *Service) trackers() string {
	var list []string
	for _, sub := range s.btSources() {
		list = append(list, sub.Trackers...)
	}
	return strings.Join(unique(list), ",")
}
func (s *Service) RefreshBTSubscriptions(ctx context.Context, force bool) error {
	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()
	s.mu.Lock()
	sources := s.btSources()
	s.mu.Unlock()
	if len(sources) == 0 {
		return nil
	}
	engine, ok := s.engine.(BTNetwork)
	if !ok {
		return ErrSubscriptionsUnsupported
	}
	var errs []error
	for _, src := range sources {
		interval := 12 * time.Hour
		if src.Failure != "" {
			interval = 30 * time.Minute
		}
		if !force && time.Since(src.Checked) < interval {
			continue
		}
		list, err := engine.FetchTrackers(ctx, src.URL)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		src.Checked = time.Now()
		src.Failure = ""
		src.HTTPStatus = 0
		if err != nil {
			src.Failure = CodeOf(err)
			var httpFailure SubscriptionHTTPError
			if errors.As(err, &httpFailure) {
				src.HTTPStatus = httpFailure.Status
			}
			errs = append(errs, err)
		} else {
			src.Trackers = unique(list)
		}
		s.mu.Lock()
		if slices.Contains(strings.Fields(s.preferences.BT.Sources), src.URL) {
			s.subscriptions[src.URL] = src
			if err := s.persist(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		s.mu.Unlock()
	}
	return joinOperationFailures(errs)
}
func (s *Service) TorrentDetails(ctx context.Context, id string) (BTDetails, error) {
	s.mu.Lock()
	hash := ""
	for _, t := range s.tasks {
		if t.Record().ID == id {
			hash = t.Record().TorrentHash
			break
		}
	}
	s.mu.Unlock()
	if hash == "" {
		return BTDetails{}, ErrTorrentNotFound
	}
	api, ok := s.engine.(BTNetwork)
	if !ok {
		return BTDetails{}, ErrReadPeers
	}
	return api.ReadTorrentDetails(ctx, id, hash)
}
func (s *Service) enqueue(ctx context.Context, spec TransferSpec) error {
	if spec.TorrentHash != "" {
		spec.BT = s.preferences.BT
		if spec.PeerLimit > 0 {
			spec.BT.MaxPeers = spec.PeerLimit
		}
		spec.Trackers = s.trackers()
	}
	return s.engine.Enqueue(ctx, spec)
}

func (s *Service) SetTorrentPeerLimit(ctx context.Context, id string, limit int) error {
	if limit < 0 || limit > 65535 {
		return ErrBTPeerLimit
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		task := &s.tasks[i]
		r := task.Record()
		if r.ID != id || r.TorrentHash == "" {
			continue
		}
		old := *task
		task.SetPeerLimit(limit)
		if err := s.persist(ctx); err != nil {
			*task = old
			return err
		}
		if r.Status != "complete" && r.Status != "error" {
			api := s.engine.(BTNetwork)
			n := limit
			if n == 0 {
				n = s.preferences.BT.MaxPeers
			}
			if n == 0 {
				n = 55
			}
			if err := api.ChangePeerLimit(ctx, id, n); err != nil {
				*task = old
				return errors.Join(err, s.persist(ctx))
			}
		}
		return nil
	}
	return ErrTorrentNotFound
}
