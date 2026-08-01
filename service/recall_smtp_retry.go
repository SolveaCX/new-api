package service

import (
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	RecallActivitySMTPSendFailedCode    = "activity_smtp_send_failed"
	RecallActivitySMTPSendFailedMessage = "Activity SMTP delivery failed. Check the host, port, credentials, TLS mode, and sender authorization, then retry."
)

var recallSMTPRetryDelays = [...]time.Duration{
	30 * time.Second,
	time.Minute,
	2 * time.Minute,
	4 * time.Minute,
}

type recallSMTPAttemptOutcome string

const (
	recallSMTPAttemptAccepted  recallSMTPAttemptOutcome = "accepted"
	recallSMTPAttemptRetryable recallSMTPAttemptOutcome = "retryable"
	recallSMTPAttemptPermanent recallSMTPAttemptOutcome = "permanent"
	recallSMTPAttemptUncertain recallSMTPAttemptOutcome = "uncertain"
)

type recallSMTPAttemptResult struct {
	Outcome recallSMTPAttemptOutcome
}

type recallSMTPUncertainError struct {
	err error
}

func (e recallSMTPUncertainError) Error() string {
	if e.err == nil {
		return "smtp outcome uncertain"
	}
	return e.err.Error()
}

func (e recallSMTPUncertainError) Unwrap() error {
	return e.err
}

func classifyRecallSMTPAttempt(err error) recallSMTPAttemptResult {
	if err == nil {
		return recallSMTPAttemptResult{Outcome: recallSMTPAttemptAccepted}
	}
	var uncertain recallSMTPUncertainError
	if errors.As(err, &uncertain) || common.IsEmailSendUncertain(err) {
		return recallSMTPAttemptResult{Outcome: recallSMTPAttemptUncertain}
	}
	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) && smtpErr.Code > 0 {
		if smtpErr.Code >= 500 && smtpErr.Code <= 599 {
			return recallSMTPAttemptResult{Outcome: recallSMTPAttemptPermanent}
		}
		if smtpErr.Code >= 400 && smtpErr.Code <= 499 {
			return recallSMTPAttemptResult{Outcome: recallSMTPAttemptRetryable}
		}
	}
	if isRecallSMTPDialFailure(err) {
		return recallSMTPAttemptResult{Outcome: recallSMTPAttemptRetryable}
	}
	if isRecallSMTPDeterministicRejection(err) {
		return recallSMTPAttemptResult{Outcome: recallSMTPAttemptPermanent}
	}
	return recallSMTPAttemptResult{Outcome: recallSMTPAttemptUncertain}
}

func isRecallSMTPDialFailure(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && strings.EqualFold(opErr.Op, "dial")
}

func isRecallSMTPDeterministicRejection(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid smtp sender") ||
		strings.Contains(message, "invalid email receiver") ||
		strings.Contains(message, "invalid email message-id") ||
		strings.Contains(message, "email headers must not contain cr or lf") ||
		strings.Contains(message, "email receiver must not contain cr or lf") ||
		strings.Contains(message, "email receiver is required")
}

var recallSMTPOutcomeSysLog = common.SysLog

func observeRecallSMTPAttemptOutcome(outcome recallSMTPAttemptOutcome) {
	recallSMTPOutcomeSysLog(fmt.Sprintf("recall smtp attempt outcome outcome=%s scope=process payload=none", outcome))
}
