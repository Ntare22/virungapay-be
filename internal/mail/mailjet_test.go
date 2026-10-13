package mail

import "testing"

func TestReadOutcomeReportsMailjetMessageError(t *testing.T) {
	body := []byte(`{"Messages":[{"Status":"error","Errors":[{"ErrorMessage":"sender not allowed"}]}]}`)
	outcome := readOutcome(body)
	if !outcome.failed || outcome.errText != "sender not allowed" {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestReadOutcomeAcceptsSuccess(t *testing.T) {
	body := []byte(`{"Messages":[{"Status":"success"}]}`)
	outcome := readOutcome(body)
	if outcome.failed || outcome.status != "success" {
		t.Fatalf("outcome = %+v", outcome)
	}
}
