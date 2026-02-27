//go:build windows

package platform

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

const helperPipe = `\\.\pipe\nekkus-gate`

// helperRequest — запрос к helper (совпадает с протоколом cmd/helper).
type helperRequest struct {
	Command string            `json:"command"`
	Params  map[string]string `json:"params"`
	Body    string            `json:"body,omitempty"`
}

// helperResponse — ответ helper.
type helperResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// IsHelperRunning возвращает true, если helper установлен и отвечает на ping.
func IsHelperRunning() bool {
	conn, err := winio.DialPipe(helperPipe, ptr(2*time.Second))
	if err != nil {
		return false
	}
	defer conn.Close()
	resp, err := helperSendCommand(conn, helperRequest{Command: "ping"})
	return err == nil && resp.Success
}

// HelperSetDNS просит helper выставить системный DNS на ip (только 127.0.0.1 или ::1).
func HelperSetDNS(ip string) error {
	resp, err := helperCall(helperRequest{
		Command: "set-dns",
		Params:  map[string]string{"ip": ip},
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("helper: %s", resp.Error)
	}
	return nil
}

// HelperRestoreDNS просит helper восстановить DNS (wasDHCP=true — DHCP, иначе servers).
func HelperRestoreDNS(adapters []string, wasDHCP bool, servers []string) error {
	params := map[string]string{}
	if len(adapters) > 0 {
		params["adapters"] = strings.Join(adapters, ",")
	}
	if wasDHCP {
		params["was_dhcp"] = "true"
	} else if len(servers) > 0 {
		params["servers"] = strings.Join(servers, ",")
	}
	resp, err := helperCall(helperRequest{Command: "restore-dns", Params: params})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("helper: %s", resp.Error)
	}
	return nil
}

// HelperFlushDNS просит helper выполнить ipconfig /flushdns.
func HelperFlushDNS() error {
	resp, err := helperCall(helperRequest{Command: "flush-dns"})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("helper: %s", resp.Error)
	}
	return nil
}

// HelperWriteHosts просит helper записать содержимое в системный файл hosts.
func HelperWriteHosts(content string) error {
	resp, err := helperCall(helperRequest{Command: "write-hosts", Body: content})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("helper: %s", resp.Error)
	}
	return nil
}

// HelperGetDNSStatus возвращает вывод netsh interface ipv4 show config (для бэкапа состояния).
func HelperGetDNSStatus() (string, error) {
	resp, err := helperCall(helperRequest{Command: "get-dns"})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("helper: %s", resp.Error)
	}
	return resp.Data, nil
}

func helperCall(req helperRequest) (*helperResponse, error) {
	timeout := 10 * time.Second
	conn, err := winio.DialPipe(helperPipe, &timeout)
	if err != nil {
		return nil, fmt.Errorf("helper not running: %w", err)
	}
	defer conn.Close()
	return helperSendCommand(conn, req)
}

func helperSendCommand(conn net.Conn, req helperRequest) (*helperResponse, error) {
	data, _ := json.Marshal(req)
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no response from helper")
	}
	var resp helperResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func ptr(d time.Duration) *time.Duration { return &d }
