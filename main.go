// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattermost/push-proxy/server"
)

var flagConfigFile string

var stopChan chan os.Signal = make(chan os.Signal)

func main() {

	flag.StringVar(&flagConfigFile, "config", "mattermost-push-proxy.json", "")
	flag.Parse()
	server.LoadConfig(flagConfigFile)

	server.Start()

	// wait for kill signal before attempting to gracefully shutdown
	// the running service
	signal.Notify(stopChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-stopChan

	server.Stop()
}
