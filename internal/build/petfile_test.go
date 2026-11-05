package build

import (
	"strings"
	"testing"
)

func TestParsePetfile(t *testing.T) {
	src := `
# comment
FROM alpine:3.20
WORKDIR /app
ENV FOO=bar
ENV BAZ qux
COPY . /app/
RUN echo hi && \
    echo there
CMD ["sleep", "1"]
ENTRYPOINT ["/bin/sh", "-c"]
`
	insts, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(insts) != 8 {
		t.Fatalf("len=%d %+v", len(insts), insts)
	}
	if insts[0].Kind != From || insts[0].Args[0] != "alpine:3.20" {
		t.Fatalf("%+v", insts[0])
	}
	if insts[3].Kind != Env || insts[3].Args[0] != "BAZ=qux" {
		t.Fatalf("%+v", insts[3])
	}
	if insts[5].Kind != Run || !strings.Contains(insts[5].Args[0], "echo there") {
		t.Fatalf("%+v", insts[5])
	}
	if len(insts[6].Args) != 2 || insts[6].Args[0] != "sleep" {
		t.Fatalf("CMD %+v", insts[6])
	}
}

func TestParseRequiresFROM(t *testing.T) {
	_, err := Parse(strings.NewReader("RUN true\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}
