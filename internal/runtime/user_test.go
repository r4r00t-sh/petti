package runtime_test

import (
	"reflect"
	"testing"

	"github.com/r4r00t-sh/petti/internal/runtime"
)

func TestParseUser(t *testing.T) {
	u, err := runtime.ParseUser("1000")
	if err != nil || !u.Set || u.UID != 1000 || u.GID != 1000 {
		t.Fatalf("%+v %v", u, err)
	}
	u, err = runtime.ParseUser("1000:100")
	if err != nil || u.UID != 1000 || u.GID != 100 || !u.HasGID {
		t.Fatalf("%+v %v", u, err)
	}
	if _, err := runtime.ParseUser("root"); err == nil {
		t.Fatal("expected error for non-numeric")
	}
}

func TestParseEnv(t *testing.T) {
	k, v, err := runtime.ParseEnv("FOO=bar=baz")
	if err != nil || k != "FOO" || v != "bar=baz" {
		t.Fatalf("%q %q %v", k, v, err)
	}
	if _, _, err := runtime.ParseEnv("NOEQUALS"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMergeEnv(t *testing.T) {
	out, err := runtime.MergeEnv(
		[]string{"PATH=/bin", "HOME=/root", "A=1"},
		[]string{"A=2", "B=3"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"PATH=/bin", "HOME=/root", "A=2", "B=3"}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("got %#v want %#v", out, want)
	}
}

func TestMergeEntrypoint(t *testing.T) {
	got := runtime.MergeEntrypoint([]string{"/bin/echo"}, []string{"hi", "there"})
	want := []string{"/bin/echo", "hi", "there"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%#v", got)
	}
	got = runtime.MergeEntrypoint(nil, []string{"ps"})
	if !reflect.DeepEqual(got, []string{"ps"}) {
		t.Fatalf("%#v", got)
	}
}

func TestDefaultCapNames(t *testing.T) {
	names := runtime.DefaultCapNames()
	if len(names) < 10 {
		t.Fatalf("too few caps: %v", names)
	}
	found := false
	for _, n := range names {
		if n == "SYS_CHROOT" {
			found = true
		}
		if n == "SYS_ADMIN" {
			t.Fatal("SYS_ADMIN must not be in default set")
		}
	}
	if !found {
		t.Fatal("missing SYS_CHROOT")
	}
}
