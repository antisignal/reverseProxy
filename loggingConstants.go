package main

const (
	EVENT_SERVER_STARTED    = "server_started"
	EVENT_REQUEST_RECEIVED  = "[STRING NOT ASSIGNED]"
	EVENT_BACKEND_SELECTING = "backend_selecting"
	EVENT_BACKEND_SELECTED  = "backend_selected"
	EVENT_BACKEND_SKIPPING  = "backend_skipping"

	EVENT_REQUEST_COMPLETED      = "request_completed"
	EVENT_BACKEND_HEALTH_CHANGED = "backend_health_changed"
	EVENT_BACKEND_ADDED          = "[STRING NOT ASSIGNED]"
	EVENT_BACKEND_REMOVED
	EVENT_PROXY_ERROR
	EVENT_PROXY_ERROR_STARTUP   = "proxy_error_startup"
	EVENT_PROXY_STARTING        = "proxy_starting"
	EVENT_PROXY_STOPPING        = "proxy_stopping"
	EVENT_PROXY_RETURNING_503   = "proxy_returning_503"
	EVENT_SERVER_ERROR          = "server_error"
	EVENT_SERVER_ERROR_STARTUP  = "server_error_startup"
	EVENT_CHAOS_EXITING         = "chaos_exiting"
	EVENT_CHAOS_KILLING_BACKEND = "chaos_killing_backend"
	EVENT_CHAOS_KILLED_BACKEND  = "chaos_killed_backend"
	EVENT_CHAOS_FAILED_TO_KILL  = "chaos_listener_failed_to_kill"
	EVENT_HEALTHCHECKER_ERROR   = "healthchecker_error"
	EVENT_PROGRAM_EXITING       = "program_exiting"
)

const (
	REASON_LISTEN_FAILED                 = "listen_failed"
	REASON_PORTS_EXHAUSTED               = "ports_exhausted"
	REASON_BACKEND_DEAD                  = "backend_dead"
	REASON_DUMMY_URL_INVALID             = "dummy_url_invalid"
	REASON_ALL_BACKENDS_DEAD             = "all_backends_dead"
	REASON_TOO_MANY_BACKENDS_DEAD        = "too_many_backends_dead"
	REASON_LISTENER_ALREADY_CLOSED       = "listener_already_closed"
	REASON_FAILED_TO_MARSHAL_TO_JSON     = "failed_to_marshal_to_json"
	REASON_FAILED_TO_WRITE               = "failed_to_write"
	REASON_STOPPING_NORMALLY             = "stopping_normally"
	REASON_STOPPING_DUE_TO_ERROR         = "stopping_due_to_error"
	REASON_REQUEST_FAILED                = "request_failed"
	REASON_FAILED_TO_CLOSE_RESPONSE_BODY = "failed_to_close_response_body"
	REASON_TERMINATE_ON_CHAOS_EXITING    = "terminate_on_chaos_exiting"
)
