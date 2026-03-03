package email

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/huipeng/anyfeed/internal/config"
	"github.com/huipeng/anyfeed/internal/source"
)

func TestEmailSource_New(t *testing.T) {
	cfg := config.FeedConfig{
		Name:    "test-email",
		Type:    config.FeedTypeEmail,
		Match:   "from:*@example.com",
		Tags:    []string{"newsletter"},
		Enabled: true,
	}

	src, err := New(cfg, 2525)
	if err != nil {
		t.Fatalf("failed to create email source: %v", err)
	}

	if src.Name() != "test-email" {
		t.Errorf("expected name 'test-email', got '%s'", src.Name())
	}
	if src.Type() != "email" {
		t.Errorf("expected type 'email', got '%s'", src.Type())
	}
}

func TestEmailSource_InvalidMatch(t *testing.T) {
	cfg := config.FeedConfig{
		Name:    "test-email",
		Type:    config.FeedTypeEmail,
		Match:   "invalid-match-expression",
		Enabled: true,
	}

	_, err := New(cfg, 2525)
	if err == nil {
		t.Error("expected error for invalid match expression")
	}
}

func TestEmailSource_EmailToEntry(t *testing.T) {
	cfg := config.FeedConfig{
		Name:    "test-email",
		Type:    config.FeedTypeEmail,
		Match:   "from:*@example.com",
		Tags:    []string{"newsletter"},
		Enabled: true,
	}

	src, _ := New(cfg, 2525)

	email := &Email{
		From:    "sender@example.com",
		To:      []string{"recipient@domain.com"},
		Subject: "Test Newsletter",
		Body:    "This is the newsletter content.",
		HTML:    "<h1>Newsletter</h1><p>Content</p>",
		Date:    time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	entry := src.emailToEntry(email)

	if entry.Title != "Test Newsletter" {
		t.Errorf("expected title 'Test Newsletter', got '%s'", entry.Title)
	}
	if entry.Author != "sender@example.com" {
		t.Errorf("expected author 'sender@example.com', got '%s'", entry.Author)
	}
	if entry.Content != "<h1>Newsletter</h1><p>Content</p>" {
		t.Errorf("expected HTML content, got '%s'", entry.Content)
	}
	if entry.SourceName != "test-email" {
		t.Errorf("expected source name 'test-email', got '%s'", entry.SourceName)
	}
	if entry.SourceType != "email" {
		t.Errorf("expected source type 'email', got '%s'", entry.SourceType)
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != "newsletter" {
		t.Errorf("expected tags ['newsletter'], got %v", entry.Tags)
	}
}

func TestEmailSource_PlainTextEmail(t *testing.T) {
	cfg := config.FeedConfig{
		Name:    "test-email",
		Type:    config.FeedTypeEmail,
		Match:   "from:*@example.com",
		Enabled: true,
	}

	src, _ := New(cfg, 2525)

	email := &Email{
		From:    "sender@example.com",
		Subject: "Plain Text Email",
		Body:    "This is plain text content.",
		Date:    time.Now(),
	}

	entry := src.emailToEntry(email)

	// Plain text should be wrapped in pre tags
	if entry.Content != "<pre>This is plain text content.</pre>" {
		t.Errorf("expected wrapped content, got '%s'", entry.Content)
	}
}

func TestSMTPServer_StartStop(t *testing.T) {
	// Find available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	server := NewSMTPServer(port, func(email *Email) {
		t.Log("received email:", email.Subject)
	})

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start SMTP server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test connection
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 5*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to SMTP server: %v", err)
	}
	conn.Close()

	if err := server.Stop(); err != nil {
		t.Errorf("failed to stop SMTP server: %v", err)
	}
}

func TestSMTPServer_ReceiveEmail(t *testing.T) {
	// Find available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	var received *Email
	var mu sync.Mutex
	done := make(chan struct{})

	server := NewSMTPServer(port, func(email *Email) {
		mu.Lock()
		received = email
		mu.Unlock()
		select {
		case <-done:
		default:
			close(done)
		}
	})

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start SMTP server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Send test email using a helper function
	sendTestEmail(t, port, "sender@example.com", "recipient@domain.com", "Test Email", "This is the email body.")

	// Wait for email to be processed
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for email")
	}

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("no email received")
	}
	if received.From != "sender@example.com" {
		t.Errorf("expected from 'sender@example.com', got '%s'", received.From)
	}
	if received.Subject != "Test Email" {
		t.Errorf("expected subject 'Test Email', got '%s'", received.Subject)
	}
}

// sendTestEmail sends a test email via SMTP with proper timeout handling.
func sendTestEmail(t *testing.T, port int, from, to, subject, body string) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 5*time.Second)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Set timeout for all operations
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	buf := make([]byte, 1024)

	// Read greeting
	_, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read greeting: %v", err)
	}

	// Helper to send command and read response
	sendCmd := func(cmd string) {
		_, err := conn.Write([]byte(cmd))
		if err != nil {
			t.Fatalf("failed to write command: %v", err)
		}
		_, err = conn.Read(buf)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}
	}

	sendCmd("EHLO localhost\r\n")
	sendCmd(fmt.Sprintf("MAIL FROM:<%s>\r\n", from))
	sendCmd(fmt.Sprintf("RCPT TO:<%s>\r\n", to))
	sendCmd("DATA\r\n")

	// Send email content (no response expected until final .)
	emailContent := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n.\r\n", from, to, subject, body)
	_, err = conn.Write([]byte(emailContent))
	if err != nil {
		t.Fatalf("failed to write email content: %v", err)
	}
	_, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read DATA response: %v", err)
	}

	sendCmd("QUIT\r\n")
}

func TestEmailSource_Integration(t *testing.T) {
	// Find available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	cfg := config.FeedConfig{
		Name:    "test-email",
		Type:    config.FeedTypeEmail,
		Match:   "from:*@example.com",
		Tags:    []string{"test"},
		Enabled: true,
	}

	src, err := New(cfg, port)
	if err != nil {
		t.Fatalf("failed to create email source: %v", err)
	}

	var receivedEntries []*source.Entry
	var mu sync.Mutex
	done := make(chan struct{})

	src.SetHandler(func(entries []*source.Entry) {
		mu.Lock()
		receivedEntries = append(receivedEntries, entries...)
		mu.Unlock()
		select {
		case <-done:
		default:
			close(done)
		}
	})

	ctx := context.Background()
	if err := src.Start(ctx); err != nil {
		t.Fatalf("failed to start email source: %v", err)
	}
	defer src.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Send matching email
	sendTestEmail(t, port, "sender@example.com", "recipient@domain.com", "Newsletter from Example", "Newsletter content.")

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for email entry")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(receivedEntries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(receivedEntries))
	}

	entry := receivedEntries[0]
	if entry.Title != "Newsletter from Example" {
		t.Errorf("expected title 'Newsletter from Example', got '%s'", entry.Title)
	}
	if entry.SourceName != "test-email" {
		t.Errorf("expected source 'test-email', got '%s'", entry.SourceName)
	}
}
