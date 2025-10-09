package email

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/tidwall/gjson"
)

type emailSuite struct {
	suite.Suite

	email *Email
}

func (e *emailSuite) SetupTest() {
	cfg := Config{
		ServerConfig: ServerConfig{
			ServerHost: "localhost",
			ServerPort: "2525",
			Username:   "test",
			Password:   "test",
		},
		SenderEmail:    "test1@servflow.io",
		RecipientEmail: "test2@servflow.io",
		Name:           "Servflow",
		Subject:        "Verify Email Action",
		Content:        []byte("This is a test email to verify that email actions work"),
	}

	e.email = New(cfg)
}

func TestEmailSuite(t *testing.T) {
	suite.Run(t, new(emailSuite))
}

func (e *emailSuite) TestEmailAction() {
	cfg := e.email.Config()

	go e.startMockSMTPServer(fmt.Sprintf("%s:%s", gjson.Get(cfg, "auth.serverHostname"), gjson.Get(cfg, "auth.serverPort")))
	time.Sleep(time.Second)

	_, err := e.email.Execute(context.Background(), cfg)
	e.Require().NoError(err)
}

func (e *emailSuite) startMockSMTPServer(address string) {
	listener, err := net.Listen("tcp", address)
	e.Require().NoError(err)

	defer listener.Close()

	log.Printf("Mock SMTP server running on %s\n", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			e.handleSMTPConnection(c)
		}(conn)
	}
}

func (e *emailSuite) handleSMTPConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	e.Require().NoError(writeResponse(writer, "220 Mock SMTP Server"))

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "HELO") || strings.HasPrefix(line, "EHLO"):
			e.Require().NoError(writeResponse(writer, "250-Hello, pleased to meet you"))
			e.Require().NoError(writeResponse(writer, "250-AUTH PLAIN LOGIN"))
			e.Require().NoError(writeResponse(writer, "250 OK"))
		case strings.HasPrefix(line, "AUTH PLAIN"):
			e.Require().NoError(writeResponse(writer, "235 Authentication successful"))
		case strings.HasPrefix(line, "MAIL FROM:"):
			e.Require().NoError(writeResponse(writer, "250 OK"))
		case strings.HasPrefix(line, "RCPT TO:"):
			e.Require().NoError(writeResponse(writer, "250 OK"))
		case strings.HasPrefix(line, "DATA"):
			e.Require().NoError(writeResponse(writer, "354 Start mail input; end with <CRLF>.<CRLF>"))

			// Handle email content
			for {
				contentLine, err := reader.ReadString('\n')
				if err != nil {
					e.Require().NoError(err)
				}

				contentLine = strings.TrimSpace(contentLine)
				if contentLine == "." {
					e.Require().NoError(writeResponse(writer, "250 OK: Message accepted"))
					break
				}
			}
		case strings.HasPrefix(line, "QUIT"):
			e.Require().NoError(writeResponse(writer, "221 Bye"))
			return
		default:
			e.Require().NoError(writeResponse(writer, "500 Unrecognized command"))
		}
	}
}

func writeResponse(writer *bufio.Writer, response string) error {
	_, err := writer.WriteString(fmt.Sprintf("%s\r\n", response))

	if err != nil {
		return err
	}

	return writer.Flush()
}
