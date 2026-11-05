package cli

// Exit codes used by petti. Keep these stable; scripts and docs rely on them.
const (
	ExitOK       = 0   // success
	ExitRuntime  = 1   // runtime / operational error
	ExitUsage    = 2   // usage / unknown command / bad flags
	ExitInternal = 125 // petti internal failure (unexpected)
	// Container exit codes from `petti run` are passed through as-is (later milestones).
)
