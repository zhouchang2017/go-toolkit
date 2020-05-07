package common

import (
	"errors"
	"testing"
	"time"
)

func TestWithAttempts(t *testing.T) {
	usedAttempts := 0
	err := WithAttempts(3, func() error { usedAttempts++; return nil })
	if err != nil {
		t.Error("Func Should have returned nil.")
	}
	if usedAttempts != 1 {
		t.Errorf("Func should have succeeded after 1 attempts. It took %d", usedAttempts)

	}

	usedAttempts = 0
	err = WithAttempts(3, func() error { usedAttempts++; return errors.New("someError") })
	if err == nil {
		t.Error("Func Should NOT have returned nil.")
	}
	if usedAttempts != 3 {
		t.Errorf("Func should have failed after 3 attempts. It took %d", usedAttempts)

	}

	usedAttempts = 0
	err = WithAttempts(3, func() error {
		usedAttempts++
		if usedAttempts != 3 {
			return errors.New("someError")
		} else {
			return nil
		}
	})
	if err != nil {
		t.Error("Func Should have returned nil.")
	}
	if usedAttempts != 3 {
		t.Errorf("Func should have succeeded after 3 attempts. It took %d", usedAttempts)
	}
}

func TestWithBackoff(t *testing.T) {
	err := errors.New("someError")
	funcWithBackOff := WithBackoff(1*time.Second, func() error { return err })
	before := time.Now()
	retErr := funcWithBackOff()
	if retErr == nil {
		t.Error("An error 'someError'. Got: ", retErr)
	}
	after := time.Now()
	diff := after.Sub(before)
	if diff < 1*time.Second || diff > 2*time.Second {
		t.Error("Time elapsed shuld have been MORE than 1 second and less than 2. Was: ", diff)
	}

	before = time.Now()
	retErr = funcWithBackOff()
	if retErr == nil {
		t.Error("An error 'someError'. Got: ", retErr)
	}
	after = time.Now()
	diff = after.Sub(before)
	if diff < 2*time.Second || diff > 3*time.Second {
		t.Error("Time elapsed shuld have been MORE than 2 second and less than 3. Was: ", diff)
	}

	err = nil
	before = time.Now()
	retErr = funcWithBackOff()
	if retErr != nil {
		t.Error("No error should have been returned. Got: ", retErr)
	}
	after = time.Now()
	diff = after.Sub(before)
	if diff > 1*time.Second {
		t.Error("Time elapsed shuld have been LESS than 1 second. Was: ", diff)
	}

}
