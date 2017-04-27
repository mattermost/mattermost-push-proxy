package gcm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPriorityNewMessage(t *testing.T) {
	m := NewMessage(nil, "1")
	jm, err := json.Marshal(m)
	if err != nil {
		t.Errorf("NewMessage failed to produce valid json")
	}
	jms := string(jm)
	if !strings.Contains(jms, "priority\":\"high\"") {
		t.Errorf("NewMessage should produce high priority value. Found: %s", jms)
	}
}
