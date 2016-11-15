package gcm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testResponse struct {
	StatusCode int
	Response   *Response
}

func startTestServer(t *testing.T, responses ...*testResponse) *httptest.Server {
	i := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		if i >= len(responses) {
			t.Fatalf("server received %d requests, expected %d", i+1, len(responses))
		}
		resp := responses[i]
		status := resp.StatusCode
		if status == 0 || status == http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			respBytes, _ := json.Marshal(resp.Response)
			fmt.Fprint(w, string(respBytes))
		} else {
			w.WriteHeader(status)
		}
		i++
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	gcmSendEndpoint = server.URL
	return server
}

func TestSendNoRetryInvalidApiKey(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()
	sender := &Sender{ApiKey: ""}
	if _, err := sender.SendNoRetry(&Message{RegistrationIDs: []string{"1"}}); err == nil {
		t.Fatal("test should fail when sender's ApiKey is \"\"")
	}
}

func TestSendInvalidApiKey(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()
	sender := &Sender{ApiKey: ""}
	if _, err := sender.Send(&Message{RegistrationIDs: []string{"1"}}, 0); err == nil {
		t.Fatal("test should fail when sender's ApiKey is \"\"")
	}
}

func TestSendNoRetryInvalidMessage(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()
	sender := &Sender{ApiKey: "test"}
	if _, err := sender.SendNoRetry(nil); err == nil {
		t.Fatal("test should fail when message is nil")
	}
	if _, err := sender.SendNoRetry(&Message{}); err == nil {
		t.Fatal("test should fail when message RegistrationIDs field is nil")
	}
	if _, err := sender.SendNoRetry(&Message{RegistrationIDs: []string{}}); err == nil {
		t.Fatal("test should fail when message RegistrationIDs field is an empty slice")
	}
	if _, err := sender.SendNoRetry(&Message{RegistrationIDs: make([]string, 1001)}); err == nil {
		t.Fatal("test should fail when more than 1000 RegistrationIDs are specified")
	}
	if _, err := sender.SendNoRetry(&Message{RegistrationIDs: []string{"1"}, TimeToLive: -1}); err == nil {
		t.Fatal("test should fail when message TimeToLive field is negative")
	}
	if _, err := sender.SendNoRetry(&Message{RegistrationIDs: []string{"1"}, TimeToLive: 2419201}); err == nil {
		t.Fatal("test should fail when message TimeToLive field is greater than 2419200")
	}
}

func TestSendInvalidMessage(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()
	sender := &Sender{ApiKey: "test"}
	if _, err := sender.Send(nil, 0); err == nil {
		t.Fatal("test should fail when message is nil")
	}
	if _, err := sender.Send(&Message{}, 0); err == nil {
		t.Fatal("test should fail when message RegistrationIDs field is nil")
	}
	if _, err := sender.Send(&Message{RegistrationIDs: []string{}}, 0); err == nil {
		t.Fatal("test should fail when message RegistrationIDs field is an empty slice")
	}
	if _, err := sender.Send(&Message{RegistrationIDs: make([]string, 1001)}, 0); err == nil {
		t.Fatal("test should fail when more than 1000 RegistrationIDs are specified")
	}
	if _, err := sender.Send(&Message{RegistrationIDs: []string{"1"}, TimeToLive: -1}, 0); err == nil {
		t.Fatal("test should fail when message TimeToLive field is negative")
	}
	if _, err := sender.Send(&Message{RegistrationIDs: []string{"1"}, TimeToLive: 2419201}, 0); err == nil {
		t.Fatal("test should fail when message TimeToLive field is greater than 2419200")
	}
}

func TestSendNoRetrySuccess(t *testing.T) {
	server := startTestServer(t, &testResponse{Response: &Response{}})
	defer server.Close()
	sender := &Sender{ApiKey: "test"}
	msg := NewMessage(map[string]interface{}{"key": "value"}, "1")
	if _, err := sender.SendNoRetry(msg); err != nil {
		t.Fatalf("test failed with error: %s", err)
	}
}

func TestSendNoRetryNonrecoverableFailure(t *testing.T) {
	server := startTestServer(t, &testResponse{StatusCode: http.StatusBadRequest})
	defer server.Close()
	sender := &Sender{ApiKey: "test"}
	msg := NewMessage(map[string]interface{}{"key": "value"}, "1")
	if _, err := sender.SendNoRetry(msg); err == nil {
		t.Fatal("test expected non-recoverable error")
	}
}

func TestSendOneRetrySuccess(t *testing.T) {
	server := startTestServer(t,
		&testResponse{Response: &Response{Failure: 1, Results: []Result{{Error: "Unavailable"}}}},
		&testResponse{Response: &Response{Success: 1, Results: []Result{{MessageID: "id"}}}},
	)
	defer server.Close()
	sender := &Sender{ApiKey: "test"}
	msg := NewMessage(map[string]interface{}{"key": "value"}, "1")
	if _, err := sender.Send(msg, 1); err != nil {
		t.Fatal("send should succeed after one retry")
	}
}

func TestSendOneRetryFailure(t *testing.T) {
	server := startTestServer(t,
		&testResponse{Response: &Response{Failure: 1, Results: []Result{{Error: "Unavailable"}}}},
		&testResponse{Response: &Response{Failure: 1, Results: []Result{{Error: "Unavailable"}}}},
	)
	defer server.Close()
	sender := &Sender{ApiKey: "test"}
	msg := NewMessage(map[string]interface{}{"key": "value"}, "1")
	resp, err := sender.Send(msg, 1)
	if err != nil || resp.Failure != 1 {
		t.Fatal("send should return response with one failure")
	}
}

func TestSendOneRetryNonrecoverableFailure(t *testing.T) {
	server := startTestServer(t,
		&testResponse{Response: &Response{Failure: 1, Results: []Result{{Error: "Unavailable"}}}},
		&testResponse{StatusCode: http.StatusBadRequest},
	)
	defer server.Close()
	sender := &Sender{ApiKey: "test"}
	msg := NewMessage(map[string]interface{}{"key": "value"}, "1")
	if _, err := sender.Send(msg, 1); err == nil {
		t.Fatal("send should fail after one retry")
	}
}
