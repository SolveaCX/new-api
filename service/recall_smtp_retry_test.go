package service

import (
	"errors"
	"net"
	"net/textproto"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecallSMTPClassifiesAttemptOutcomes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want recallSMTPAttemptOutcome
	}{
		{name: "accepted", want: recallSMTPAttemptAccepted},
		{name: "smtp_4xx_retryable", err: &textproto.Error{Code: 421, Msg: "service unavailable"}, want: recallSMTPAttemptRetryable},
		{name: "smtp_5xx_permanent", err: &textproto.Error{Code: 550, Msg: "mailbox unavailable"}, want: recallSMTPAttemptPermanent},
		{name: "dial_retryable", err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, want: recallSMTPAttemptRetryable},
		{name: "post_data_uncertain", err: recallSMTPUncertainError{err: errors.New("connection reset after DATA")}, want: recallSMTPAttemptUncertain},
		{name: "deterministic_content_rejection", err: errors.New("email headers must not contain CR or LF"), want: recallSMTPAttemptPermanent},
		{name: "raw_provider_error_uncertain", err: errors.New("temporary provider failure"), want: recallSMTPAttemptUncertain},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, classifyRecallSMTPAttempt(testCase.err).Outcome)
		})
	}
}

func TestRecallSMTPRetryDelaysAreExactFourSlots(t *testing.T) {
	require.Equal(t, [...]time.Duration{
		30 * time.Second,
		time.Minute,
		2 * time.Minute,
		4 * time.Minute,
	}, recallSMTPRetryDelays)
}

func TestRecallSMTPOutcomeObservationLogsSanitizedPerAttempt(t *testing.T) {
	originalSysLog := recallSMTPOutcomeSysLog
	var logs []string
	recallSMTPOutcomeSysLog = func(message string) {
		logs = append(logs, message)
	}
	t.Cleanup(func() {
		recallSMTPOutcomeSysLog = originalSysLog
	})

	observeRecallSMTPAttemptOutcome(recallSMTPAttemptUncertain)

	require.Equal(t, []string{
		"recall smtp attempt outcome outcome=uncertain scope=process payload=none",
	}, logs)
}
