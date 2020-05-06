package kv

import (
	"crypto/rand"
	"os/exec"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	DBPATH = "db"
)

type StorageTestSuite struct {
	suite.Suite
	storage *Storage
	t       StorageType
}

func (suite *StorageTestSuite) SetupTest() {
	storage, err := NewStorage(DBPATH, suite.t)
	suite.Require().Nil(err)
	suite.storage = storage
	suite.storage.Put([]byte("key01"), []byte("value01"))
	suite.storage.Put([]byte("key02"), []byte("value02"))
	suite.storage.Put([]byte("key03"), []byte("value03"))
	suite.storage.Put([]byte("key04"), []byte("value04"))
	suite.storage.Put([]byte("key05"), []byte("value05"))
	suite.storage.Put([]byte("iost01"), []byte("value06"))
	suite.storage.Put([]byte("iost02"), []byte("value07"))
	suite.storage.Put([]byte("iost03"), []byte("value08"))
	suite.storage.Put([]byte("iost04"), []byte("value09"))
	suite.storage.Put([]byte("iost05"), []byte("value10"))
}

func (suite *StorageTestSuite) TestGet() {
	var value []byte
	var err error
	value, err = suite.storage.Get([]byte("key00"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)
	value, err = suite.storage.Get([]byte("key01"))
	suite.Nil(err)
	suite.Equal([]byte("value01"), value)
	value, err = suite.storage.Get([]byte("key05"))
	suite.Nil(err)
	suite.Equal([]byte("value05"), value)
}

func (suite *StorageTestSuite) TestPut() {
	var value []byte
	var err error
	value, err = suite.storage.Get([]byte("key06"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)
	value, err = suite.storage.Get([]byte("key07"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)

	err = suite.storage.Put([]byte("key07"), []byte("value07"))
	suite.Nil(err)

	value, err = suite.storage.Get([]byte("key06"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)
	value, err = suite.storage.Get([]byte("key07"))
	suite.Nil(err)
	suite.Equal([]byte("value07"), value)
}

func (suite *StorageTestSuite) TestDelete() {
	var value []byte
	var err error
	value, err = suite.storage.Get([]byte("key04"))
	suite.Nil(err)
	suite.Equal([]byte("value04"), value)
	value, err = suite.storage.Get([]byte("key05"))
	suite.Nil(err)
	suite.Equal([]byte("value05"), value)

	err = suite.storage.Delete([]byte("key04"))
	suite.Nil(err)

	value, err = suite.storage.Get([]byte("key04"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)
	value, err = suite.storage.Get([]byte("key05"))
	suite.Nil(err)
	suite.Equal([]byte("value05"), value)
}

func (suite *StorageTestSuite) TestKeys() {
	var keys [][]byte
	var err error

	keys, err = suite.storage.Keys([]byte("key"))
	suite.Nil(err)
	suite.ElementsMatch(
		[][]byte{
			[]byte("key01"),
			[]byte("key02"),
			[]byte("key03"),
			[]byte("key04"),
			[]byte("key05"),
		},
		keys,
	)
	keys, err = suite.storage.Keys([]byte(global.Token))
	suite.Nil(err)
	suite.ElementsMatch(
		[][]byte{
			[]byte("iost01"),
			[]byte("iost02"),
			[]byte("iost03"),
			[]byte("iost04"),
			[]byte("iost05"),
		},
		keys,
	)
}

func (suite *StorageTestSuite) TestBatch() {
	var value []byte
	var err error

	value, err = suite.storage.Get([]byte("key04"))
	suite.Nil(err)
	suite.Equal([]byte("value04"), value)
	value, err = suite.storage.Get([]byte("key05"))
	suite.Nil(err)
	suite.Equal([]byte("value05"), value)
	value, err = suite.storage.Get([]byte("key06"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)

	err = suite.storage.BeginBatch()
	suite.Nil(err)
	err = suite.storage.BeginBatch()
	suite.NotNil(err)

	err = suite.storage.Delete([]byte("key04"))
	suite.Nil(err)
	err = suite.storage.Put([]byte("key06"), []byte("value06"))
	suite.Nil(err)

	err = suite.storage.CommitBatch()
	suite.Nil(err)
	err = suite.storage.CommitBatch()
	suite.NotNil(err)

	value, err = suite.storage.Get([]byte("key04"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)
	value, err = suite.storage.Get([]byte("key05"))
	suite.Nil(err)
	suite.Equal([]byte("value05"), value)
	value, err = suite.storage.Get([]byte("key06"))
	suite.Nil(err)
	suite.Equal([]byte("value06"), value)
}

func (suite *StorageTestSuite) TestRecover() {
	var value []byte
	var err error

	value, err = suite.storage.Get([]byte("key04"))
	suite.Nil(err)
	suite.Equal([]byte("value04"), value)
	value, err = suite.storage.Get([]byte("key05"))
	suite.Nil(err)
	suite.Equal([]byte("value05"), value)
	value, err = suite.storage.Get([]byte("key06"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)

	err = suite.storage.BeginBatch()
	suite.Nil(err)

	err = suite.storage.Delete([]byte("key04"))
	suite.Nil(err)
	err = suite.storage.Put([]byte("key06"), []byte("value06"))
	suite.Nil(err)

	err = suite.storage.Close()
	suite.Nil(err)
	storage, err := NewStorage(DBPATH, suite.t)
	suite.Require().Nil(err)
	suite.storage = storage

	err = suite.storage.CommitBatch()
	suite.NotNil(err)

	value, err = suite.storage.Get([]byte("key04"))
	suite.Nil(err)
	suite.Equal([]byte("value04"), value)
	value, err = suite.storage.Get([]byte("key05"))
	suite.Nil(err)
	suite.Equal([]byte("value05"), value)
	value, err = suite.storage.Get([]byte("key06"))
	suite.Nil(err)
	suite.Equal([]byte{}, value)
}

func (suite *StorageTestSuite) TearDownTest() {
	err := suite.storage.Close()
	suite.Nil(err)
	cmd := exec.Command("rm", "-r", DBPATH)
	err = cmd.Run()
	suite.Require().Nil(err)
}

func TestStorageTestSuite(t *testing.T) {
	suite.Run(t, &StorageTestSuite{t: LevelDBStorage})
}

func BenchmarkStorage(b *testing.B) {
	for _, t := range []StorageType{LevelDBStorage} {
		storage, err := NewStorage(DBPATH, t)
		if err != nil {
			b.Fatalf("Failed to new storage: %v", err)
		}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		for i := 0; i < 400000; i++ {
			key := make([]byte, 32)
			value := make([]byte, 32)
			rand.Read(key)
			rand.Read(value)
			keys = append(keys, key)
			values = append(values, value)
		}

		b.Run(reflect.TypeOf(storage.StorageBackend).String()+"Put", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := storage.Put(keys[i], values[i])
				assert.Nil(b, err)
			}
		})

		for i := 0; i < 400000; i++ {
			err := storage.Put(keys[i], values[i])
			assert.Nil(b, err)
		}

		b.Run(reflect.TypeOf(storage.StorageBackend).String()+"Get", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				value, err := storage.Get(keys[i])
				assert.Nil(b, err)
				if !assert.Equal(b, values[i], value) {
					b.Fatalf("Num: %v", i)
				}
			}
		})
		b.Run(reflect.TypeOf(storage.StorageBackend).String()+"Delete", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := storage.Delete(keys[i])
				assert.Nil(b, err)
			}
		})

		storage.Close()
		cmd := exec.Command("rm", "-r", DBPATH)
		cmd.Run()
	}
}

func BenchmarkKeys(b *testing.B) {
	for _, t := range []StorageType{LevelDBStorage} {
		storage, err := NewStorage(DBPATH, t)
		if err != nil {
			b.Fatalf("Failed to new storage: %v", err)
		}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		headkeys := make([][]byte, 0)
		headkey := make([]byte, 32)
		for i := 0; i < 10000; i++ {
			if i%2500 == 0 {
				headkey = make([]byte, 32)
				rand.Read(headkey)
				headkeys = append(headkeys, headkey)
			}
			key := make([]byte, 32)
			value := make([]byte, 128)
			rand.Read(key)
			rand.Read(value)
			keys = append(keys, append(headkey, key...))
			values = append(values, value)
			storage.Put(append(headkey, key...), value)
		}
		b.Run(reflect.TypeOf(storage.StorageBackend).String()+"Keys", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for j := 0; j < len(headkeys); j++ {
					storage.Keys(headkeys[j])
				}
			}
		})

		b.Run(reflect.TypeOf(storage.StorageBackend).String()+"Get", func(b *testing.B) {
			vals := make([][]byte, 0)
			for i := 0; i < b.N; i++ {
				for j := 0; j < len(keys); j++ {
					val, _ := storage.Get(keys[j])
					vals = append(vals, val)
				}
			}
		})
		storage.Close()
		cmd := exec.Command("rm", "-r", DBPATH)
		cmd.Run()
	}
}

func BenchmarkIterator(b *testing.B) {
	storage, err := NewStorage(DBPATH, LevelDBStorage)
	if err != nil {
		b.Fatalf("Failed to new storage: %v", err)
	}

	keys := make([][]byte, 0)
	values := make([][]byte, 0)
	headkeys := make([][]byte, 0)
	headkey := make([]byte, 32)
	bnum := 100
	txnum := 2000

	for i := 0; i < txnum*bnum; i++ {
		if i%txnum == 0 {
			headkey = make([]byte, 32)
			rand.Read(headkey)
			headkeys = append(headkeys, headkey)
		}
		key := make([]byte, 32)
		value := make([]byte, 128)
		rand.Read(key)
		rand.Read(value)
		keys = append(keys, append(headkey, key...))
		values = append(values, value)
		storage.Put(append(headkey, key...), value)
	}
	b.Run(reflect.TypeOf(storage.StorageBackend).String()+"Iterator", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			iter := storage.NewIteratorByPrefix(headkeys[i%bnum])
			iter.Release()
			err := iter.Error()
			if !assert.Nil(b, err) {
				b.Fatalf("Fail to New the Iterator: %v", err)
			}
		}
	})
	b.Run(reflect.TypeOf(storage.StorageBackend).String()+"IteratorAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			iter := storage.NewIteratorByPrefix(headkeys[i%bnum])
			for iter.Next() {
			}
			iter.Release()
			err := iter.Error()
			if !assert.Nil(b, err) {
				b.Fatalf("Fail to iterate the Iterator: %v", err)
			}
		}
	})
	b.Run(reflect.TypeOf(storage.StorageBackend).String()+"Get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := 0; j < txnum; j++ {
				storage.Get(keys[i%bnum*txnum+j])
			}
		}
	})
	storage.Close()
	cmd := exec.Command("rm", "-r", DBPATH)
	cmd.Run()
}
