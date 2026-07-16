package common

import (
	"errors"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailMessageIncludesProvidedStableMessageID(t *testing.T) {
	originalFrom := SMTPFrom
	originalAccount := SMTPAccount
	originalName := SystemName
	t.Cleanup(func() {
		SMTPFrom = originalFrom
		SMTPAccount = originalAccount
		SystemName = originalName
	})
	SMTPFrom = "sender@mail.example.com"
	SMTPAccount = ""
	SystemName = "Flatkey"

	message, err := buildEmailMessage("Welcome back", "user@example.com", "<p>Hello</p>", "<recall-42-1@mail.example.com>")
	require.NoError(t, err)
	require.Contains(t, string(message), "Message-ID: <recall-42-1@mail.example.com>\r\n")
	require.Contains(t, string(message), "To: user@example.com\r\n")
}

func TestEmailMessageRejectsHeaderInjection(t *testing.T) {
	originalFrom := SMTPFrom
	originalAccount := SMTPAccount
	t.Cleanup(func() {
		SMTPFrom = originalFrom
		SMTPAccount = originalAccount
	})
	SMTPFrom = "sender@mail.example.com"
	SMTPAccount = ""

	tests := []struct {
		name      string
		subject   string
		receiver  string
		messageID string
	}{
		{name: "subject", subject: "Welcome\r\nBcc: victim@example.com", receiver: "user@example.com", messageID: "<recall-42-1@mail.example.com>"},
		{name: "receiver", subject: "Welcome", receiver: "user@example.com\nBcc: victim@example.com", messageID: "<recall-42-1@mail.example.com>"},
		{name: "message id", subject: "Welcome", receiver: "user@example.com", messageID: "<recall-42-1@mail.example.com>\r\nBcc: victim@example.com"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := buildEmailMessage(testCase.subject, testCase.receiver, "body", testCase.messageID)
			require.Error(t, err)
		})
	}
}

func TestEmailMessageDomainUsesEffectiveSenderMailbox(t *testing.T) {
	originalFrom := SMTPFrom
	originalAccount := SMTPAccount
	t.Cleanup(func() {
		SMTPFrom = originalFrom
		SMTPAccount = originalAccount
	})

	SMTPFrom = ""
	SMTPAccount = "sender@Fallback.Example.COM"
	domain, err := EmailMessageIDDomain()
	require.NoError(t, err)
	require.Equal(t, "fallback.example.com", domain)

	for _, sender := range []string{
		"sender@example.com,other@example.com",
		"sender@example.com\r\nBcc: victim@example.com",
		"sender@invalid_domain",
	} {
		SMTPFrom = sender
		_, err := EmailMessageIDDomain()
		require.Error(t, err, sender)
	}
}

func TestEmailMessageCanonicalizesAddressHeadersAndRejectsInvalidMessageIDDomain(t *testing.T) {
	originalFrom := SMTPFrom
	originalAccount := SMTPAccount
	originalName := SystemName
	t.Cleanup(func() {
		SMTPFrom = originalFrom
		SMTPAccount = originalAccount
		SystemName = originalName
	})
	SMTPFrom = "sender@notify.example.com"
	SMTPAccount = "sender@notify.example.com"
	SystemName = `Flatkey "Ops" \ 通知`

	message, err := buildEmailMessage("subject", "one@example.com; two@example.com", "body", "<recall-1-1@notify.example.com>")
	require.NoError(t, err)
	require.Contains(t, string(message), "To: one@example.com, two@example.com\r\n")
	require.Contains(t, string(message), "From: "+(&mail.Address{Name: SystemName, Address: SMTPFrom}).String()+"\r\n")

	for _, messageID := range []string{
		"<recall-1-1@a..example.com>",
		"<recall-1-1@.example.com>",
		"<recall-1-1@example.com.>",
		"<recall-1-1@invalid_domain>",
		"<recall-1-1@Notify.Example.com>",
	} {
		_, err := buildEmailMessage("subject", "one@example.com", "body", messageID)
		require.Error(t, err, messageID)
	}
}

func TestEmailMessageFallbackDoesNotMutateSMTPFrom(t *testing.T) {
	originalFrom := SMTPFrom
	originalAccount := SMTPAccount
	originalServer := SMTPServer
	originalPort := SMTPPort
	originalSSL := SMTPSSLEnabled
	t.Cleanup(func() {
		SMTPFrom = originalFrom
		SMTPAccount = originalAccount
		SMTPServer = originalServer
		SMTPPort = originalPort
		SMTPSSLEnabled = originalSSL
	})
	SMTPFrom = ""
	SMTPAccount = "sender@fallback.example.com"
	SMTPServer = "127.0.0.1"
	SMTPPort = 1
	SMTPSSLEnabled = false

	err := SendEmailWithMessageID("subject", "one@example.com", "body", "<recall-1-1@fallback.example.com>")
	require.Error(t, err)
	require.Empty(t, SMTPFrom)
}

func TestEmailMessageUncertainClassificationPreservesErrorChain(t *testing.T) {
	cause := errors.New("connection reset after DATA")
	err := &emailSendError{Uncertain: true, Err: cause}
	require.True(t, IsEmailSendUncertain(err))
	require.ErrorIs(t, err, cause)
	require.Equal(t, cause.Error(), err.Error())

	definite := &emailSendError{Uncertain: false, Err: cause}
	require.False(t, IsEmailSendUncertain(definite))
	require.True(t, strings.Contains(definite.Error(), "connection reset"))
}
