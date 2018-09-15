package database

import (
	"testing"

	"strings"

	"encoding/json"
	"os"
	"time"

	"runtime/pprof"

	"github.com/iost-official/Go-IOS-Protocol/db"
)

func watchTimeout(t *testing.T, f func()) {
	ta := time.Now()
	f()
	tb := time.Now().Sub(ta)
	if tb > time.Millisecond {
		t.Error("time overflow")
	}
}

func TestJson(t *testing.T) {
	buf, err := json.Marshal([]string{"abc", "def"})
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != `["abc","def"]` {
		t.Fatal(string(buf))
	}
}

func TestString(t *testing.T) {
	ss := strings.Replace("@a@b@c", "@b", "", 1)
	if ss != `@a@c` {
		t.Fatal(ss)
	}

	sl := strings.Split("@a@b@c", "@")
	if sl[1] != "a" {
		t.Fatal(sl)
	}
}

func TestMap(t *testing.T) {
	t.Skip() // todo repair db put time or find out why
	d, err := db.NewMVCCDB("mvcc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		d.Close()
		os.RemoveAll("mvcc")
	}()

	f, err := os.Create("mput.prof")
	if err != nil {
		panic(err)
	}
	err = pprof.StartCPUProfile(f)
	if err != nil {
		panic(err)
	}
	defer pprof.StopCPUProfile()

	vi := NewVisitor(100, d)

	watchTimeout(t, func() {
		vi.MapHandler.MPut("a", "b", "c")
	})
	watchTimeout(t, func() {
		vi.MapHandler.MPut("b", "bb", "c")
	})
	watchTimeout(t, func() {
		vi.MapHandler.MPut("c", "b", "c")
	})

	watchTimeout(t, func() {
		vi.MapHandler.MHas("a", "b")
	})
}
