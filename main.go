// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattermost/mattermost-push-proxy/server"
)

var flagConfigFile string

func main() {
	flag.StringVar(&flagConfigFile, "config", "mattermost-push-proxy.json", "")
	flag.Parse()

	fileName := server.FindConfigFile(flagConfigFile)
	cfg, err := server.LoadConfig(fileName)
	if err != nil {
		// We just do a hard exit, because the app won't be able to start without a config.
		log.Fatal(err)
	}

	logger := server.NewLogger(cfg)
	logger.Info("Loading " + fileName)

	srv := server.New(cfg, logger)
	srv.Start()

	// wait for kill signal before attempting to gracefully shutdown
	// the running service
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-stopChan

	srv.Stop()
}
