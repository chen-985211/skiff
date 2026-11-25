//go:build integration

package transfer_test

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"skiff/internal/contexts/transfer/application"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A passive FTP origin with REST support. No public server or credentials are used.
func ftpOrigin(t *testing.T, data []byte, requireLogin bool) string {
	t.Helper()
	control, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { control.Close() })
	go func() {
		for {
			conn, err := control.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(30 * time.Second))
				reply := func(s string) { fmt.Fprint(conn, s+"\r\n") }
				reply("220 ready")
				var passive net.Listener
				defer func() {
					if passive != nil {
						passive.Close()
					}
				}()
				var user string
				offset := 0
				lines := bufio.NewScanner(conn)
				for lines.Scan() {
					command, arg, _ := strings.Cut(lines.Text(), " ")
					switch command {
					case "USER":
						user = arg
						reply("331 Password required")
					case "PASS":
						if requireLogin && (user != "reader" || arg != "test-pass") {
							reply("530 Login incorrect")
							continue
						}
						reply("230 Logged in")
					case "CWD":
						reply("250 Directory changed")
					case "TYPE":
						reply("200 OK")
					case "SYST":
						reply("215 UNIX Type: L8")
					case "PWD":
						reply("257 \"/\"")
					case "SIZE":
						reply(fmt.Sprintf("213 %d", len(data)))
					case "MDTM":
						reply("213 20261120000000")
					case "REST":
						offset, _ = strconv.Atoi(arg)
						reply("350 restart accepted")
					case "PASV", "EPSV":
						if passive != nil {
							passive.Close()
						}
						var e error
						passive, e = net.Listen("tcp4", "127.0.0.1:0")
						if e != nil {
							return
						}
						passive.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second))
						port := passive.Addr().(*net.TCPAddr).Port
						if command == "EPSV" {
							reply(fmt.Sprintf("229 Entering Extended Passive Mode (|||%d|)", port))
						} else {
							reply(fmt.Sprintf("227 Entering Passive Mode (127,0,0,1,%d,%d)", port/256, port%256))
						}
					case "RETR":
						if passive == nil || offset < 0 || offset > len(data) {
							reply("425 no data")
							return
						}
						reply("150 Opening data")
						dc, e := passive.Accept()
						if e != nil {
							return
						}
						for offset < len(data) {
							end := min(offset+16384, len(data))
							_, e = dc.Write(data[offset:end])
							if e != nil {
								break
							}
							offset = end
							time.Sleep(4 * time.Millisecond)
						}
						dc.Close()
						passive.Close()
						passive = nil
						reply("226 complete")
					case "QUIT":
						reply("221 bye")
						return
					default:
						reply("502 unsupported")
					}
				}
			}()
		}
	}()
	return control.Addr().String()
}

func TestFTPFilesPauseAndResume(t *testing.T) {
	data := bytes.Repeat([]byte("ftp-data"), 256<<10)
	for _, auth := range []bool{false, true} {
		t.Run(fmt.Sprint("login=", auth), func(t *testing.T) {
			host := ftpOrigin(t, data, auth)
			if auth {
				host = "reader:test-pass@" + host
			}
			svc := downloadService(t)
			if _, err := svc.Add(t.Context(), "ftp://"+host+"/payload.bin", svc.Snapshot().Directory); err != nil {
				t.Fatal(err)
			}
			id := svc.Snapshot().Tasks[0].ID
			if strings.Contains(svc.Snapshot().Tasks[0].URL, "test-pass") {
				t.Fatal("FTP password exposed in UI")
			}
			awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Completed >= 128<<10 })
			if err := svc.Execute(t.Context(), id, application.Pause); err != nil {
				t.Fatal(err)
			}
			awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.CanResume })
			if err := svc.Execute(t.Context(), id, application.Resume); err != nil {
				t.Fatal(err)
			}
			end := awaitTask(t, svc, id, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
			got, err := os.ReadFile(end.Path)
			if err != nil || !bytes.Equal(data, got) {
				t.Fatal(end, err)
			}
		})
	}
}

func TestFTPLoginRequired(t *testing.T) {
	svc := downloadService(t)
	host := ftpOrigin(t, []byte("ok"), true)
	if _, err := svc.Add(t.Context(), "ftp://"+host+"/private.bin", svc.Snapshot().Directory); err != nil {
		t.Fatal(err)
	}
	task := awaitTask(t, svc, svc.Snapshot().Tasks[0].ID, func(v application.TaskDTO) bool { return v.Status == "error" })
	if !task.NeedsFTPLogin || task.Failure != application.FailureFTPLoginRequired {
		t.Fatal(task.Error)
	}

	if err := svc.RetryFTP(t.Context(), task.ID, "reader", "test-pass"); err != nil {
		t.Fatal(err)
	}
	end := awaitTask(t, svc, task.ID, func(v application.TaskDTO) bool { return v.Status == "complete" || v.Status == "error" })
	got, err := os.ReadFile(end.Path)
	if err != nil || string(got) != "ok" || end.NeedsFTPLogin {
		t.Fatal(end, err)
	}
}
