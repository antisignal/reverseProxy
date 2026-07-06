package config

type DebugInfo struct {
	Test502BadGateway       bool
	TestDeadBackends        bool
	TerminateOnChaosExiting bool
	verbose                 bool
}

var debugInfo = DebugInfo{
	Test502BadGateway:       false,
	TestDeadBackends:        true,
	TerminateOnChaosExiting: true,
	verbose:                 true,
}

func GetDebugInfo() DebugInfo {
	return debugInfo
}
