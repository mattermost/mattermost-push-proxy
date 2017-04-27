gcm
===

Documentation: http://godoc.org/github.com/alexjlockwood/gcm

Getting Started
---------------

To install gcm, use `go get`:

```bash
go get github.com/mattermost/gcm
```

Import gcm with the following:

```go
import "github.com/mattermost/gcm"
```

Sample Usage
------------

Here is a quick sample illustrating how to send a message to the GCM server:

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/mattermost/gcm"
)

func main() {
	// Create the message to be sent.
	data := map[string]interface{}{"score": "5x1", "time": "15:10"}
	regIDs := []string{"4", "8", "15", "16", "23", "42"}
	msg := gcm.NewMessage(data, regIDs...)

	// Create a Sender to send the message.
	sender := &gcm.Sender{ApiKey: "sample_api_key"}

	// Send the message and receive the response after at most two retries.
	response, err := sender.Send(msg, 2)
	if err != nil {
		fmt.Println("Failed to send message:", err)
		return
	}

	/* ... */
}
```

Note for Google AppEngine users
-------------------------------

If your application server runs on Google AppEngine, you must import the `appengine/urlfetch` package and create the `Sender` as follows:

```go
package sample

import (
	"appengine"
	"appengine/urlfetch"

	"github.com/alexjlockwood/gcm"
)

func handler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	client := urlfetch.Client(c)
	sender := &gcm.Sender{ApiKey: "sample_api_key", Http: client}

	/* ... */
}        
```
