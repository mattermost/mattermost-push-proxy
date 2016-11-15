// Google Cloud Messaging for application servers implemented using the
// Go programming language.
package gcm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"
)

const (
	// GcmSendEndpoint is the endpoint for sending messages to the GCM server.
	GcmSendEndpoint = "https://android.googleapis.com/gcm/send"
	// Initial delay before first retry, without jitter.
	backoffInitialDelay = 1000
	// Maximum delay before a retry.
	maxBackoffDelay = 1024000
)

// Declared as a mutable variable for testing purposes.
var gcmSendEndpoint = GcmSendEndpoint

// Sender abstracts the interaction between the application server and the
// GCM server. The developer must obtain an API key from the Google APIs
// Console page and pass it to the Sender so that it can perform authorized
// requests on the application server's behalf. To send a message to one or
// more devices use the Sender's Send or SendNoRetry methods.
//
// If the Http field is nil, a zeroed http.Client will be allocated and used
// to send messages. If your application server runs on Google AppEngine,
// you must use the "appengine/urlfetch" package to create the *http.Client
// as follows:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//		c := appengine.NewContext(r)
//		client := urlfetch.Client(c)
//		sender := &gcm.Sender{ApiKey: key, Http: client}
//
//		/* ... */
//	}
type Sender struct {
	ApiKey string
	Http   *http.Client
}

// SendNoRetry sends a message to the GCM server without retrying in case of
// service unavailability. A non-nil error is returned if a non-recoverable
// error occurs (i.e. if the response status is not "200 OK").
func (s *Sender) SendNoRetry(msg *Message) (*Response, error) {
	if err := checkSender(s); err != nil {
		return nil, err
	} else if err := checkMessage(msg); err != nil {
		return nil, err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", gcmSendEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("key=%s", s.ApiKey))
	req.Header.Add("Content-Type", "application/json")

	resp, err := s.Http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d error: %s", resp.StatusCode, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	response := new(Response)
	err = json.Unmarshal(body, response)
	return response, err
}

// Send sends a message to the GCM server, retrying in case of service
// unavailability. A non-nil error is returned if a non-recoverable
// error occurs (i.e. if the response status is not "200 OK").
//
// Note that messages are retried using exponential backoff, and as a
// result, this method may block for several seconds.
func (s *Sender) Send(msg *Message, retries int) (*Response, error) {
	if err := checkSender(s); err != nil {
		return nil, err
	} else if err := checkMessage(msg); err != nil {
		return nil, err
	} else if retries < 0 {
		return nil, errors.New("'retries' must not be negative.")
	}

	// Send the message for the first time.
	resp, err := s.SendNoRetry(msg)
	if err != nil {
		return nil, err
	} else if resp.Failure == 0 || retries == 0 {
		return resp, nil
	}

	// One or more messages failed to send.
	regIDs := msg.RegistrationIDs
	allResults := make(map[string]Result, len(regIDs))
	backoff := backoffInitialDelay
	for i := 0; updateStatus(msg, resp, allResults) > 0 && i < retries; i++ {
		sleepTime := backoff/2 + rand.Intn(backoff)
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		backoff = min(2*backoff, maxBackoffDelay)
		if resp, err = s.SendNoRetry(msg); err != nil {
			msg.RegistrationIDs = regIDs
			return nil, err
		}
	}

	// Bring the message back to its original state.
	msg.RegistrationIDs = regIDs

	// Create a Response containing the overall results.
	finalResults := make([]Result, len(regIDs))
	var success, failure, canonicalIDs int
	for i := 0; i < len(regIDs); i++ {
		result, _ := allResults[regIDs[i]]
		finalResults[i] = result
		if result.MessageID != "" {
			if result.RegistrationID != "" {
				canonicalIDs++
			}
			success++
		} else {
			failure++
		}
	}

	return &Response{
		// Return the most recent multicast id.
		MulticastID:  resp.MulticastID,
		Success:      success,
		Failure:      failure,
		CanonicalIDs: canonicalIDs,
		Results:      finalResults,
	}, nil
}

// updateStatus updates the status of the messages sent to devices and
// returns the number of recoverable errors that could be retried.
func updateStatus(msg *Message, resp *Response, allResults map[string]Result) int {
	unsentRegIDs := make([]string, 0, resp.Failure)
	for i := 0; i < len(resp.Results); i++ {
		regID := msg.RegistrationIDs[i]
		allResults[regID] = resp.Results[i]
		if resp.Results[i].Error == "Unavailable" {
			unsentRegIDs = append(unsentRegIDs, regID)
		}
	}
	msg.RegistrationIDs = unsentRegIDs
	return len(unsentRegIDs)
}

// min returns the smaller of two integers. For exciting religious wars
// about why this wasn't included in the "math" package, see this thread:
// https://groups.google.com/d/topic/golang-nuts/dbyqx_LGUxM/discussion
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// checkSender returns an error if the sender is not well-formed and
// initializes a zeroed http.Client if one has not been provided.
func checkSender(sender *Sender) error {
	if sender.ApiKey == "" {
		return errors.New("the sender's API key must not be empty")
	}
	if sender.Http == nil {
		sender.Http = new(http.Client)
	}
	return nil
}

// checkMessage returns an error if the message is not well-formed.
func checkMessage(msg *Message) error {
	if msg == nil {
		return errors.New("the message must not be nil")
	} else if msg.RegistrationIDs == nil {
		return errors.New("the message's RegistrationIDs field must not be nil")
	} else if len(msg.RegistrationIDs) == 0 {
		return errors.New("the message must specify at least one registration ID")
	} else if len(msg.RegistrationIDs) > 1000 {
		return errors.New("the message may specify at most 1000 registration IDs")
	} else if msg.TimeToLive < 0 || 2419200 < msg.TimeToLive {
		return errors.New("the message's TimeToLive field must be an integer " +
			"between 0 and 2419200 (4 weeks)")
	}
	return nil
}
