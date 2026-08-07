package config

import (
	"log/slog"
	"net"
	"os"
	"reverseProxy/internal/logging"
	"strconv"
	"time"
)

type LogLevel int

const (
	CONFIG_LOG_LEVEL_DEBUG LogLevel = iota
	CONFIG_LOG_LEVEL_INFO
	CONFIG_LOG_LEVEL_WARN
	CONFIG_LOG_LEVEL_ERROR
	CONFIG_LOG_LEVEL_FATAL
)

type Config struct {
	OriginIP                net.IP
	ListenPort              int
	LogLevel                LogLevel
	OriginPortsStart        int
	NumBackends             int
	ChaosKillInterval       time.Duration
	Test502BadGateway       bool
	TestDeadBackends        bool
	TerminateOnChaosExiting bool
}

func GetConfig() *Config {
	var config = &Config{}

	var originIPString = os.Getenv("REVERSE_PROXY_ORIGIN_IP")
	if originIPString == "" {
		originIPString = "127.0.0.1"
	}
	var originIP = net.ParseIP(originIPString)
	if originIP == nil {
		slog.Error("[config] invalid origin IP provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_ORIGIN_IP,
			"service", "config",
			"originIPString", originIPString,
			"timestamp", time.Now().String())
		return nil
	}
	config.OriginIP = originIP

	var listenPortString = os.Getenv("REVERSE_PROXY_LISTEN_PORT")
	if listenPortString == "" {
		listenPortString = "8080"
	}
	listenPort, err := strconv.Atoi(listenPortString)
	if err != nil {
		slog.Error("[config] invalid listen port provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_LISTEN_PORT,
			"service", "config",
			"listenPortString", listenPortString,
			"timestamp", time.Now().String())
		return nil
	}
	config.ListenPort = listenPort

	var logLevelString = os.Getenv("REVERSE_PROXY_LOG_LEVEL")
	if logLevelString == "" {
		logLevelString = "INFO"
	}
	switch logLevelString {
	case "DEBUG": // XXX: this could be a place where entropy gets introduced,
		// if more cases are added
		config.LogLevel = CONFIG_LOG_LEVEL_DEBUG
	case "INFO":
		config.LogLevel = CONFIG_LOG_LEVEL_INFO
	case "WARN":
		config.LogLevel = CONFIG_LOG_LEVEL_WARN
	case "ERROR":
		config.LogLevel = CONFIG_LOG_LEVEL_ERROR
	case "FATAL":
		config.LogLevel = CONFIG_LOG_LEVEL_FATAL
	default:
		slog.Error("[config] invalid log level provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_LOG_LEVEL,
			"service", "config",
			"logLevelString", logLevelString,
			"timestamp", time.Now().String(),
		)
		return nil
	}
	var originPortsStart = os.Getenv("REVERSE_PROXY_ORIGIN_PORTS_START")
	if originPortsStart == "" {
		config.OriginPortsStart = 9090
	}
	originPortsStartInt, err := strconv.Atoi(originPortsStart)
	if err != nil {
		slog.Error("[config] invalid origin ports start provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_ORIGIN_PORTS_START,
			"service", "config",
			"originPortsStart", originPortsStart,
			"timestamp", time.Now().String(),
		)
		return nil
	}
	config.OriginPortsStart = originPortsStartInt

	var numBackendsString = os.Getenv("REVERSE_PROXY_NUMBER_BACKENDS")
	if numBackendsString == "" {
		numBackendsString = "10"
	}
	numBackends, err := strconv.Atoi(numBackendsString)
	if err != nil {
		slog.Error("[config] invalid number of backends provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_NUM_BACKENDS,
			"service", "config",
			"numBackendsString", numBackendsString,
			"timestamp", time.Now().String(),
		)
		return nil
	}
	config.NumBackends = numBackends

	var chaosKillIntervalString = os.Getenv("REVERSE_PROXY_CHAOS_KILL_INTERVAL_SEC")
	if chaosKillIntervalString == "" {
		config.ChaosKillInterval = time.Second * 5
	}
	_, err = strconv.Atoi(chaosKillIntervalString) // making sure it parses to an interval makes the ParseDuration safer
	if err != nil {
		slog.Error("[config] invalid chaos kill interval provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_CHAOS_KILL_INTERVAL_SEC,
			"service", "config",
			"chaosKillIntervalString", chaosKillIntervalString,
			"timestamp", time.Now().String(),
		)
		return nil
	}
	config.ChaosKillInterval, _ = time.ParseDuration(chaosKillIntervalString + "s") // error should be impossible

	var test502BadGatewayString = os.Getenv("REVERSE_PROXY_TEST_502_BAD_GATEWAY")
	if test502BadGatewayString == "" {
		test502BadGatewayString = "FALSE"
	}
	switch test502BadGatewayString {
	case "FALSE":
		config.Test502BadGateway = false
	case "TRUE":
		config.Test502BadGateway = true
	default:
		slog.Error("[config] invalid test 502 bad gateway bool provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_TEST_502_BAD_GATEWAY_OPTION,
			"service", "config",
			"chaosKillIntervalString", chaosKillIntervalString,
			"timestamp", time.Now().String(),
		)
		return nil
	}

	var testDeadBackendsString = os.Getenv("REVERSE_PROXY_TEST_DEAD_BACKENDS")

	if testDeadBackendsString == "" {
		testDeadBackendsString = "FALSE"
	}
	switch testDeadBackendsString {
	case "FALSE":
		config.TestDeadBackends = false
	case "TRUE":
		config.TestDeadBackends = true
	default:
		slog.Error("[config] invalid test dead backends bool provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_TEST_DEAD_BACKENDS_OPTION,
			"service", "config",
			"deadBackendsString", testDeadBackendsString,
			"timestamp", time.Now().String())
		return nil
	}

	var terminateOnChaosExitingString = os.Getenv("REVERSE_PROXY_TERMINATE_ON_CHAOS_EXITING")
	if terminateOnChaosExitingString == "" {
		terminateOnChaosExitingString = "FALSE"
	}
	switch terminateOnChaosExitingString {
	case "FALSE":
		config.TerminateOnChaosExiting = false
	case "TRUE":
		config.TerminateOnChaosExiting = true
	default:
		slog.Error("[config] invalid test dead backends bool provided",
			"event", logging.EVENT_INVALID_CONFIG,
			"reason", logging.REASON_INVALID_TERMINATE_ON_CHAOS_EXITING_OPTION,
			"service", "config",
			"terminateOnChaosExitingString", terminateOnChaosExitingString,
			"timestamp", time.Now().String())
		return nil
	}
	return config
}
