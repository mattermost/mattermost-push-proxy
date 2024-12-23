package server

import (
	"encoding/json"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func NewLogger(cfg *ConfigPushProxy) (*mlog.Logger, error) {
	// Initialize the logger - begin
	logger, err := mlog.NewLogger()
	if err != nil {
		return nil, err
	}
	if cfg.LogFormat != "plain" && cfg.LogFormat != "json" {
		cfg.LogFormat = "plain"
	}
	err = logger.ConfigureTargets(buildLogConfig(cfg), nil)
	if err != nil {
		return logger, err
	}

	return logger, nil
}

func buildLogConfig(cfg *ConfigPushProxy) mlog.LoggerConfiguration {
	logConf := make(mlog.LoggerConfiguration)

	if cfg.EnableFileLog && cfg.LogFileLocation != "" {
		logConf["file"] = buildLogFileConfig(cfg.LogFileLocation, cfg.LogFormat)
	}

	if cfg.EnableConsoleLog || cfg.LogFileLocation == "" || !cfg.EnableFileLog {
		logConf["console"] = buildConsoleLogConfig(cfg.LogFormat)
	}

	return logConf
}

func buildConsoleLogConfig(format string) mlog.TargetCfg {
	return mlog.TargetCfg{
		Type:          "console",
		Levels:        mlog.StdAll,
		Format:        format,
		Options:       json.RawMessage(`{"out": "stdout"}`),
		FormatOptions: json.RawMessage(`{"enable_color": true, "enable_caller": true}`),
		MaxQueueSize:  1000,
	}
}

func buildLogFileConfig(filename string, format string) mlog.TargetCfg {
	opts := struct {
		Filename    string `json:"filename"`
		Max_size    int    `json:"max_size"`
		Max_age     int    `json:"max_age"`
		Max_backups int    `json:"max_backups"`
		Compress    bool   `json:"compress"`
	}{
		Filename:    filename,
		Max_size:    100,
		Max_age:     0,
		Max_backups: 0,
		Compress:    true,
	}
	var optsJsonString, _ = json.Marshal(opts)

	return mlog.TargetCfg{
		Type:          "file",
		Levels:        mlog.StdAll,
		Format:        format,
		Options:       optsJsonString,
		FormatOptions: json.RawMessage(`{"enable_color": true, "enable_caller": true}`),
		MaxQueueSize:  1000,
	}
}
