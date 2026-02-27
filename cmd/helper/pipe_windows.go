//go:build windows

package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

const pipeName = `\\.\pipe\nekkus-gate`

// Request — запрос от Gate к helper.
type Request struct {
	Command string            `json:"command"`
	Params  map[string]string `json:"params"`
	Body    string            `json:"body,omitempty"` // для write-hosts: полное содержимое файла
}

// Response — ответ helper.
type Response struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

var allowedCommands = map[string]bool{
	"ping":        true,
	"set-dns":     true,
	"restore-dns": true,
	"get-dns":     true,
	"flush-dns":   true,
	"write-hosts": true,
}

func startPipeServer(stopCh <-chan struct{}) {
	// WD = World (все процессы), чтобы процесс Gate от обычного пользователя мог подключаться к сервису (LocalSystem).
	// BU иногда недостаточен для сессии пользователя.
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;WD)",
		MessageMode:        false,
		InputBufferSize:    4096,
		OutputBufferSize:   4096,
	}
	listener, err := winio.ListenPipe(pipeName, cfg)
	if err != nil {
		log.Printf("Pipe listen error: %v", err)
		return
	}
	defer listener.Close()

	go func() {
		<-stopCh
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			log.Printf("Pipe accept error: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeResponse(conn, Response{Success: false, Error: "invalid request"})
			continue
		}
		if !allowedCommands[req.Command] {
			writeResponse(conn, Response{Success: false, Error: "command not allowed"})
			continue
		}
		if req.Command == "set-dns" {
			ip := req.Params["ip"]
			if ip != "127.0.0.1" && ip != "::1" {
				writeResponse(conn, Response{Success: false, Error: "only loopback DNS allowed (127.0.0.1 or ::1)"})
				continue
			}
		}
		resp := executeCommand(req)
		writeResponse(conn, resp)
	}
}

func writeResponse(conn net.Conn, resp Response) {
	data, _ := json.Marshal(resp)
	conn.Write(append(data, '\n'))
}

func executeCommand(req Request) Response {
	switch req.Command {
	case "ping":
		return Response{Success: true, Data: "pong"}
	case "set-dns":
		return setDNS(req.Params["ip"])
	case "restore-dns":
		return restoreDNS(req)
	case "get-dns":
		return getDNSStatus()
	case "flush-dns":
		return flushDNS()
	case "write-hosts":
		return writeHosts(req.Body)
	default:
		return Response{Success: false, Error: "unknown command"}
	}
}
