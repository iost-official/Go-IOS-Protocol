package snapshot

import (
	"fmt"
	"github.com/iost-official/go-iost/common/config"
	"math/rand"
	"os"
	"testing"

	"github.com/iost-official/go-iost/db"
	. "github.com/smartystreets/goconvey/convey"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

func TestSnapshot(t *testing.T) {
	Convey("Test of Snapshot", t, func() {
		os.RemoveAll("DB")
		defer os.RemoveAll("DB")
		stateDB, err := db.NewMVCCDB("DB/StateDB")
		So(err, ShouldBeNil)

		for i := 0; i < 100; i++ {
			err = stateDB.Put("state", randString(64), randString(32))
			if err != nil {
				fmt.Println(err)
			}
		}
		stateDB.Commit("abc")
		stateDB.Flush("abc")
		stateDB.Close()
		config := &config.Config{
			DB: &config.DBConfig{
				LdbPath: "DB/",
			},
			Snapshot: &config.SnapshotConfig{
				Enable:   true,
				FilePath: "DB/Snapshot.tar.gz",
			},
		}
		err = ToSnapshot(config)
		So(err, ShouldBeNil)
		err = ToFile(config)
		So(err, ShouldBeNil)
		os.RemoveAll("DB/StateDB/")
		err = FromSnapshot(config)
		So(err, ShouldBeNil)
	})
}

func BenchmarkSnapshot(b *testing.B) {
	os.RemoveAll("DB")
	defer os.RemoveAll("DB")
	stateDB, err := db.NewMVCCDB("DB/StateDB")

	if err != nil {
		fmt.Println(err)
	}
	for i := 0; i < 1000000; i++ {
		stateDB.Put("state", randString(64), randString(32))
	}
	stateDB.Commit("abc")
	stateDB.Flush("abc")
	stateDB.Close()
	config := &config.Config{
		DB: &config.DBConfig{
			LdbPath: "DB/",
		},
	}
	b.Run("ToSnapshot", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			err = ToSnapshot(config)
		}
	})

	b.Run("ToFile", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			err = ToFile(config)
		}
	})

}
