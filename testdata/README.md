# Integration tests build a busybox rootfs in a temp dir at runtime.
# Requires a *statically* linked busybox (`busybox-static` on Debian/Ubuntu);
# a dynamic host busybox cannot exec after pivot_root (no ld.so in the fixture).
