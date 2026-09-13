package main

// runServe is invoked by the "awo serve" command.
// It prints a startup banner, reads configuration from flags or environment
// variables, bootstraps the framework, and starts the Fiber HTTP server.
//
// Usage:
//
//	awo serve [--port 8080] [--db DATABASE_URL] [--redis REDIS_URL] [--log-level info] [--open]
func runServe(args []string) error {
	cfg := parseServeFlags(args)
	return startServer(cfg)
}
