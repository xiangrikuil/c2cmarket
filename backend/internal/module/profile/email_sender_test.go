package profile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/quotedprintable"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

type fakeSMTPClient struct {
	authCalled bool
	mailFrom   string
	rcptTo     string
	data       bytes.Buffer
	failAuth   bool
}

func (f *fakeSMTPClient) Auth(smtp.Auth) error {
	f.authCalled = true
	if f.failAuth {
		return errors.New("auth failed")
	}
	return nil
}

func (f *fakeSMTPClient) Mail(from string) error {
	f.mailFrom = from
	return nil
}

func (f *fakeSMTPClient) Rcpt(to string) error {
	f.rcptTo = to
	return nil
}

func (f *fakeSMTPClient) Data() (io.WriteCloser, error) {
	return nopWriteCloser{Writer: &f.data}, nil
}

func (f *fakeSMTPClient) Quit() error {
	return nil
}

func (f *fakeSMTPClient) Close() error {
	return nil
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func TestSMTPEmailSenderSendsRegistrationSuccess(t *testing.T) {
	client := &fakeSMTPClient{}
	sender, err := NewSMTPEmailSender(SMTPConfig{
		Host:        "smtpdm.aliyun.com",
		Port:        465,
		Username:    "noreply@example.com",
		Password:    "unit-test-password",
		FromAddress: "noreply@example.com",
		FromName:    "C2CMarket",
	})
	if err != nil {
		t.Fatalf("new smtp sender: %v", err)
	}
	sender.dial = func(context.Context, string, string) (smtpClient, error) {
		return client, nil
	}

	appErr := sender.SendRegistrationSuccess(context.Background(), "User@Example.COM", "oauth-user", "OAuth <User>", time.Date(2026, 6, 26, 1, 0, 0, 0, time.UTC))
	if appErr != nil {
		t.Fatalf("send registration: %v", appErr)
	}
	if !client.authCalled || client.mailFrom != "noreply@example.com" || client.rcptTo != "User@Example.COM" {
		t.Fatalf("unexpected smtp calls auth=%v mail=%q rcpt=%q", client.authCalled, client.mailFrom, client.rcptTo)
	}
	message := client.data.String()
	if !strings.Contains(message, "Subject: =?utf-8?") || !strings.Contains(message, "multipart/alternative") {
		t.Fatalf("expected mime headers, got %s", message)
	}
	if !strings.Contains(message, "OAuth &lt;User&gt;") {
		t.Fatalf("html/template must escape display name in html body, got %s", message)
	}
}

func TestSMTPEmailSenderSendsCarpoolApplicationReminder(t *testing.T) {
	client := &fakeSMTPClient{}
	sender, err := NewSMTPEmailSender(SMTPConfig{
		Host:        "smtpdm.aliyun.com",
		Port:        465,
		Username:    "noreply@example.com",
		Password:    "unit-test-password",
		FromAddress: "noreply@example.com",
		FromName:    "C2CMarket",
	})
	if err != nil {
		t.Fatalf("new smtp sender: %v", err)
	}
	sender.dial = func(context.Context, string, string) (smtpClient, error) {
		return client, nil
	}

	appErr := sender.SendCarpoolApplicationCreated(context.Background(), "owner@example.com", "ChatGPT Pro <拼车>", "app-123", time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC))
	if appErr != nil {
		t.Fatalf("send carpool application reminder: %v", appErr)
	}
	if !client.authCalled || client.mailFrom != "noreply@example.com" || client.rcptTo != "owner@example.com" {
		t.Fatalf("unexpected smtp calls auth=%v mail=%q rcpt=%q", client.authCalled, client.mailFrom, client.rcptTo)
	}
	message := client.data.String()
	if !strings.Contains(message, "Subject: =?utf-8?") || !strings.Contains(message, "multipart/alternative") {
		t.Fatalf("expected mime headers, got %s", message)
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(message)))
	if err != nil {
		t.Fatalf("decode quoted-printable message: %v", err)
	}
	decodedMessage := string(decoded)
	if !strings.Contains(decodedMessage, "ChatGPT Pro &lt;拼车&gt;") || !strings.Contains(decodedMessage, "app-123") || !strings.Contains(decodedMessage, "订单管理") {
		t.Fatalf("expected escaped listing title and owner workflow copy, got %s", decodedMessage)
	}
}

func TestSMTPEmailSenderErrorDoesNotLeakPassword(t *testing.T) {
	sender, err := NewSMTPEmailSender(SMTPConfig{
		Host:        "smtpdm.aliyun.com",
		Port:        465,
		Username:    "noreply@example.com",
		Password:    "secret-password-value",
		FromAddress: "noreply@example.com",
		FromName:    "C2CMarket",
	})
	if err != nil {
		t.Fatalf("new smtp sender: %v", err)
	}
	sender.dial = func(context.Context, string, string) (smtpClient, error) {
		return &fakeSMTPClient{failAuth: true}, nil
	}

	appErr := sender.SendVerificationCode(context.Background(), "user@example.com", "123456", time.Date(2026, 6, 26, 1, 15, 0, 0, time.UTC))
	if appErr == nil {
		t.Fatalf("expected send failure")
	}
	if strings.Contains(appErr.Error(), "secret-password-value") || strings.Contains(appErr.Detail, "secret-password-value") || strings.Contains(appErr.Title, "secret-password-value") {
		t.Fatalf("error leaked smtp password: %v", appErr)
	}
}

func TestNewSMTPEmailSenderRequiresImplicitTLSPort(t *testing.T) {
	_, err := NewSMTPEmailSender(SMTPConfig{
		Host:        "smtpdm.aliyun.com",
		Port:        587,
		Username:    "noreply@example.com",
		Password:    "unit-test-password",
		FromAddress: "noreply@example.com",
		FromName:    "C2CMarket",
	})
	if err == nil {
		t.Fatalf("expected non-465 smtp port to fail")
	}
}
