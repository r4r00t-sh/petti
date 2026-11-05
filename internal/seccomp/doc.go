// Package seccomp applies Petti's default syscall filter (denylist) via seccomp(2).
//
// We hand-roll a classic BPF filter with golang.org/x/sys/unix rather than
// pulling in libseccomp-golang: the profile is small, the dependency surface
// stays "stdlib + x/sys first", and the filter is easy to read for learning.
package seccomp
