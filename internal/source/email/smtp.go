package email

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"sync"
	"time"
)

// Email represents a parsed email message.
type Email struct {
	From    string
	To      []string
	Subject string
	Body    string
	HTML    string
	Date    time.Time
	Raw     []byte
}

// SMTPServer is a simple SMTP server for receiving emails.
type SMTPServer struct {
	addr     string
	listener net.Listener
	handler  func(*Email)
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewSMTPServer creates a new SMTP server.
func NewSMTPServer(port int, handler func(*Email)) *SMTPServer {
	return &SMTPServer{
		addr:    fmt.Sprintf(":%d", port),
		handler: handler,
		stopCh:  make(chan struct{}),
	}
}

// Start starts the SMTP server.
func (s *SMTPServer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to start SMTP server: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	slog.Info("SMTP server started", "addr", s.addr)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop(ctx)
	}()

	return nil
}

// acceptLoop accepts incoming connections.
func (s *SMTPServer) acceptLoop(ctx context.Context) {
	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		s.mu.Lock()
		listener := s.listener
		s.mu.Unlock()

		if listener == nil {
			return
		}

		// Set deadline to allow checking for stop signal
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-s.stopCh:
				return
			default:
				slog.Debug("SMTP accept error", "error", err)
				continue
			}
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

// handleConnection handles a single SMTP connection.
func (s *SMTPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Send greeting
	s.writeLine(writer, "220 anyfeed SMTP Server Ready")

	var from string
	var to []string
	var data []byte
	inData := false

	for {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)

		if inData {
			// Reading email data
			if line == "." {
				inData = false
				s.writeLine(writer, "250 OK")

				// Process the email
				email, err := s.parseEmail(data, from, to)
				if err != nil {
					slog.Warn("failed to parse email", "error", err)
				} else if s.handler != nil {
					s.handler(email)
				}

				// Reset for next message
				from = ""
				to = nil
				data = nil
			} else {
				// Handle dot-stuffing (lines starting with . have extra . prepended)
				if strings.HasPrefix(line, ".") {
					line = line[1:]
				}
				data = append(data, []byte(line+"\r\n")...)
			}
			continue
		}

		cmd := strings.ToUpper(line)
		if strings.HasPrefix(cmd, "HELO") || strings.HasPrefix(cmd, "EHLO") {
			s.writeLine(writer, "250 OK")
		} else if strings.HasPrefix(cmd, "MAIL FROM:") {
			from = extractAddress(line[10:])
			s.writeLine(writer, "250 OK")
		} else if strings.HasPrefix(cmd, "RCPT TO:") {
			to = append(to, extractAddress(line[8:]))
			s.writeLine(writer, "250 OK")
		} else if cmd == "DATA" {
			s.writeLine(writer, "354 Start mail input; end with <CRLF>.<CRLF>")
			inData = true
		} else if cmd == "QUIT" {
			s.writeLine(writer, "221 Bye")
			return
		} else if cmd == "RSET" {
			from = ""
			to = nil
			data = nil
			s.writeLine(writer, "250 OK")
		} else if cmd == "NOOP" {
			s.writeLine(writer, "250 OK")
		} else {
			s.writeLine(writer, "500 Command not recognized")
		}
	}
}

// writeLine writes a line to the connection.
func (s *SMTPServer) writeLine(w *bufio.Writer, line string) {
	w.WriteString(line + "\r\n")
	w.Flush()
}

// extractAddress extracts email address from MAIL FROM or RCPT TO.
func extractAddress(s string) string {
	s = strings.TrimSpace(s)
	// Handle <email@domain.com> format
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// parseEmail parses raw email data into an Email struct.
func (s *SMTPServer) parseEmail(data []byte, from string, to []string) (*Email, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		// If parsing fails, create a basic email from raw data
		return &Email{
			From:    from,
			To:      to,
			Subject: "",
			Body:    string(data),
			Date:    time.Now(),
			Raw:     data,
		}, nil
	}

	email := &Email{
		From:    from,
		To:      to,
		Raw:     data,
		Date:    time.Now(),
	}

	// Parse From header if envelope from is empty
	if email.From == "" {
		if fromHeader := msg.Header.Get("From"); fromHeader != "" {
			if addr, err := mail.ParseAddress(fromHeader); err == nil {
				email.From = addr.Address
			} else {
				email.From = fromHeader
			}
		}
	}

	// Parse subject
	email.Subject = decodeHeader(msg.Header.Get("Subject"))

	// Parse date
	if dateStr := msg.Header.Get("Date"); dateStr != "" {
		if t, err := mail.ParseDate(dateStr); err == nil {
			email.Date = t
		}
	}

	// Parse body
	email.Body, email.HTML = parseBody(msg)

	return email, nil
}

// decodeHeader decodes RFC 2047 encoded headers.
func decodeHeader(header string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(header)
	if err != nil {
		return header
	}
	return decoded
}

// parseBody extracts text and HTML parts from email body.
func parseBody(msg *mail.Message) (text, html string) {
	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Fallback to reading as plain text
		body, _ := io.ReadAll(msg.Body)
		return string(body), ""
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary != "" {
			text, html = parseMultipart(msg.Body, boundary)
			return
		}
	}

	body, _ := io.ReadAll(msg.Body)
	bodyStr := string(body)

	if strings.HasPrefix(mediaType, "text/html") {
		return "", bodyStr
	}
	return bodyStr, ""
}

// parseMultipart parses multipart email body.
func parseMultipart(r io.Reader, boundary string) (text, html string) {
	mr := multipart.NewReader(r, boundary)

	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}

		contentType := part.Header.Get("Content-Type")
		mediaType, _, _ := mime.ParseMediaType(contentType)

		body, err := io.ReadAll(part)
		if err != nil {
			continue
		}

		if strings.HasPrefix(mediaType, "text/plain") && text == "" {
			text = string(body)
		} else if strings.HasPrefix(mediaType, "text/html") && html == "" {
			html = string(body)
		} else if strings.HasPrefix(mediaType, "multipart/") {
			// Nested multipart
			nestedBoundary := ""
			if _, params, err := mime.ParseMediaType(contentType); err == nil {
				nestedBoundary = params["boundary"]
			}
			if nestedBoundary != "" {
				t, h := parseMultipart(bytes.NewReader(body), nestedBoundary)
				if text == "" {
					text = t
				}
				if html == "" {
					html = h
				}
			}
		}
	}

	return
}

// Stop stops the SMTP server.
func (s *SMTPServer) Stop() error {
	close(s.stopCh)

	s.mu.Lock()
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()
	slog.Info("SMTP server stopped")
	return nil
}
