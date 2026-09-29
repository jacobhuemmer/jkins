package jenkins

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/gorilla/websocket"
)

const (
	cliArg      = 0
	cliLocale   = 1
	cliEncoding = 2
	cliStart    = 3
	cliExit     = 4
	cliStdin    = 5
	cliEndStdin = 6
	cliStdout   = 7
	cliStderr   = 8
)

// RunCommand speaks Jenkins' WebSocket CLI protocol directly; it does not invoke jenkins-cli.jar.
func (c *Client) RunCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 0, errors.New("CLI command is required")
	}
	endpoint := *c.base
	endpoint.Scheme = "wss"
	if c.base.Scheme == "http" { // Local test controllers only.
		endpoint.Scheme = "ws"
	}
	endpoint.Path = strings.TrimRight(c.base.Path, "/") + "/cli/ws"
	endpoint.RawPath, endpoint.RawQuery, endpoint.Fragment = "", "", ""
	transport := c.http.Transport.(*http.Transport)
	dialer := websocket.Dialer{Proxy: transport.Proxy, TLSClientConfig: transport.TLSClientConfig, HandshakeTimeout: 20 * time.Second}
	headers := http.Header{}
	headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.credential.User+":"+c.credential.Token)))
	headers.Set("Origin", c.base.Scheme+"://"+c.base.Host)
	conn, response, err := dialer.DialContext(context.Background(), endpoint.String(), headers)
	if err != nil {
		if response != nil {
			return 0, fmt.Errorf("CLI handshake failed (HTTP %d)", response.StatusCode)
		}
		return 0, fmt.Errorf("CLI connection failed: %s", c.redact(err.Error()))
	}
	defer conn.Close()
	conn.SetReadLimit(16 << 20)
	for _, arg := range args {
		encoded, err := cliUTF(arg)
		if err != nil {
			return 0, err
		}
		if err := cliFrame(conn, cliArg, encoded); err != nil {
			return 0, errors.New("CLI argument send failed")
		}
	}
	encoding, _ := cliUTF("UTF-8")
	if err := cliFrame(conn, cliEncoding, encoding); err != nil {
		return 0, errors.New("CLI encoding send failed")
	}
	if locale := localCLILocale(); locale != "" {
		encoded, err := cliUTF(locale)
		if err != nil {
			return 0, err
		}
		if err := cliFrame(conn, cliLocale, encoded); err != nil {
			return 0, errors.New("CLI locale send failed")
		}
	}
	if err := cliFrame(conn, cliStart, nil); err != nil {
		return 0, errors.New("CLI start failed")
	}
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	stdinErrors := make(chan error, 1)
	go func() {
		buf := make([]byte, 60_000)
		for {
			n, readErr := stdin.Read(buf)
			if n > 0 {
				if err := cliFrame(conn, cliStdin, buf[:n]); err != nil {
					stdinErrors <- err
					return
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					stdinErrors <- cliFrame(conn, cliEndStdin, nil)
				} else {
					stdinErrors <- readErr
				}
				return
			}
		}
	}()
	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					return
				}
			case <-stopPing:
				return
			}
		}
	}()
	for {
		messageType, frame, err := conn.ReadMessage()
		if err != nil {
			select {
			case inputErr := <-stdinErrors:
				if inputErr != nil {
					return 0, errors.New("CLI stdin failed")
				}
			default:
			}
			return 0, errors.New("CLI connection closed before exit status")
		}
		if messageType != websocket.BinaryMessage || len(frame) == 0 {
			return 0, errors.New("invalid CLI response frame")
		}
		switch frame[0] {
		case cliStdout:
			if _, err := stdout.Write(frame[1:]); err != nil {
				return 0, err
			}
		case cliStderr:
			if _, err := stderr.Write(frame[1:]); err != nil {
				return 0, err
			}
		case cliExit:
			if len(frame) != 5 {
				return 0, errors.New("invalid CLI exit frame")
			}
			return int(int32(binary.BigEndian.Uint32(frame[1:]))), nil
		default:
			return 0, errors.New("unknown CLI response operation")
		}
	}
}

func localCLILocale() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := os.Getenv(name)
		value = strings.SplitN(value, ".", 2)[0]
		value = strings.SplitN(value, "@", 2)[0]
		if value != "" && value != "C" && value != "POSIX" {
			return strings.ReplaceAll(value, "-", "_")
		}
	}
	return ""
}

func cliFrame(conn *websocket.Conn, opcode byte, payload []byte) error {
	frame := make([]byte, 1+len(payload))
	frame[0] = opcode
	copy(frame[1:], payload)
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

// Java DataOutputStream.writeUTF uses modified UTF-8 with a two-byte byte length.
func cliUTF(value string) ([]byte, error) {
	encoded := make([]byte, 2, 2+len(value)*3)
	for _, unit := range utf16.Encode([]rune(value)) {
		switch {
		case unit == 0:
			encoded = append(encoded, 0xc0, 0x80)
		case unit <= 0x7f:
			encoded = append(encoded, byte(unit))
		case unit <= 0x7ff:
			encoded = append(encoded, 0xc0|byte(unit>>6), 0x80|byte(unit&0x3f))
		default:
			encoded = append(encoded, 0xe0|byte(unit>>12), 0x80|byte((unit>>6)&0x3f), 0x80|byte(unit&0x3f))
		}
		if len(encoded)-2 > 65535 {
			return nil, errors.New("CLI argument exceeds protocol limit")
		}
	}
	binary.BigEndian.PutUint16(encoded[:2], uint16(len(encoded)-2))
	return encoded, nil
}
