package cli

const usageText = `Petti — a Docker-like container runtime (educational).

Usage:
  petti [global flags] <command> [arguments]

Global flags:
  -h, --help       Show this help
  -v, --v, --version
                   Print version and exit
  --debug          Structured debug logs on stderr

Commands:
  pull             Download an image from a registry
  images           List local images
  rmi              Remove a local image tag
  build            Build an image from a Petfile
  save             Save images to a tarball
  load             Load images from a tarball
  compose          Multi-container runner (petti.yaml)
  system           Maintenance (prune)
  run              Run a container (-d, -it, --rm, -v, …)
  ps               List containers
  stop             Stop a running container
  kill             Force-kill a container
  rm               Remove a container
  logs             Fetch container logs
  inspect          Show container JSON
  exec             Run a command in a running container
  version          Print version information
  help             Show this help

Note: top-level -v means VERSION. After 'run', -v/--volume means VOLUME.

Petti is a learning runtime. It is not safe for untrusted workloads.
`

// Usage returns the help text shown for no args, -h, --help, and help.
func Usage() string {
	return usageText
}
