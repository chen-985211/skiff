package aria2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"skiff/internal/contexts/transfer/application"
	"strconv"
	"strings"
)

func (c *Client) FetchTrackers(ctx context.Context, source string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("订阅返回 HTTP %d", res.StatusCode)
	}
	const limit = 2 << 20
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || len(body) > limit {
		return nil, errors.New("无法读取完整订阅")
	}
	var trackers []string
	for line := range strings.Lines(string(body)) {
		text := strings.TrimSpace(line)
		u, err := url.Parse(text)
		if err == nil && u.Hostname() != "" && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "udp") && !strings.ContainsAny(text, ", \t") {
			trackers = append(trackers, text)
		}
	}
	if len(trackers) == 0 {
		return nil, errors.New("订阅未返回有效 Tracker")
	}
	return trackers, nil
}
func (c *Client) TorrentDetails(ctx context.Context, id string) (application.BTDetails, error) {
	var d application.BTDetails
	var status struct {
		Bitfield, NumPieces, UploadLength, Seeder, Status string
		Bittorrent                                        struct{ AnnounceList [][]string }
	}
	err := c.call(ctx, "aria2.tellStatus", []any{id, []string{"status", "bitfield", "numPieces", "uploadLength", "seeder", "bittorrent"}}, &status)
	if err != nil {
		return d, err
	}
	d.Bitfield = status.Bitfield
	d.Pieces, _ = strconv.Atoi(status.NumPieces)
	d.Uploaded, _ = strconv.ParseInt(status.UploadLength, 10, 64)
	d.Seeder = status.Seeder == "true"
	for _, tier := range status.Bittorrent.AnnounceList {
		d.Trackers = append(d.Trackers, tier...)
	}
	if status.Status != "active" {
		return d, nil
	}
	var peers []struct{ IP, Port, Seeder, DownloadSpeed, UploadSpeed string }
	if err = c.call(ctx, "aria2.getPeers", []any{id}, &peers); err != nil {
		return d, err
	}
	for _, p := range peers {
		d.Peers = append(d.Peers, application.Peer{Address: net.JoinHostPort(p.IP, p.Port), Seeder: p.Seeder == "true", Download: p.DownloadSpeed, Upload: p.UploadSpeed})
	}
	return d, nil
}
func (c *Client) ChangePeerLimit(ctx context.Context, id string, n int) error {
	return c.call(ctx, "aria2.changeOption", []any{id, map[string]string{"bt-max-peers": strconv.Itoa(n)}}, nil)
}
