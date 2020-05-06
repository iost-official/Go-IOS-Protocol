package snapshot

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"github.com/iost-official/go-iost/common/config"
	"io"
	"os"
	"path/filepath"
	"strings"

	"compress/gzip"

	"github.com/iost-official/go-iost/db/kv/leveldb"
)

/*
// Save the function for saving block's head from snapshot.
func Save(db db.MVCCDB, blk *block.Block) error {
	bhJSON, err := json.Marshal(blk.Head)
	if err != nil {
		return fmt.Errorf("json fail: %v", err)
	}
	err = db.Put("snapshot", "blockHead", string(bhJSON))
	if err != nil {
		return fmt.Errorf("state db put fail: %v", err)
	}
	return nil
}

// Load the function for loading block's head from snapshot.
func Load(db db.MVCCDB) (*block.Block, error) {
	bhJSON, err := db.Get("snapshot", "blockHead")
	if err != nil {
		return nil, fmt.Errorf("get current block head from state db failed. err: %v", err)
	}
	bh := &block.BlockHead{}
	err = json.Unmarshal([]byte(bhJSON), bh)
	if err != nil {
		return nil, fmt.Errorf("block head decode failed. err: %v", err)
	}

	blk := &block.Block{Head: bh}
	return blk, blk.CalculateHeadHash()
}
*/

// ToSnapshot the function for saving db to snapshot.
func ToSnapshot(conf *config.Config) error {
	src := filepath.Join(conf.DB.LdbPath, "StateDB")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	file, err := os.Create(filepath.Join(conf.DB.LdbPath, "Snapshot.tar.gz"))
	if err != nil {
		return err
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(file, conf.DB.LdbPath)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
		f.Close()
		return nil
	})
}

// FromSnapshot the function for loading db from snapshot.
func FromSnapshot(conf *config.Config) error {
	src := filepath.Join(conf.DB.LdbPath, "/StateDB")

	s, err := os.Stat(src)
	if err == nil && s.IsDir() {
		return errors.New("state db already has")
	}
	err = os.MkdirAll(src, os.ModePerm)
	if err != nil {
		return err
	}
	fr, err := os.Open(conf.Snapshot.FilePath)
	if err != nil {
		return err
	}
	defer fr.Close()

	gr, err := gzip.NewReader(fr)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if h.Typeflag == tar.TypeDir {
			continue
		}

		fw, err := os.OpenFile(filepath.Join(conf.DB.LdbPath, h.Name), os.O_CREATE|os.O_WRONLY, os.FileMode(h.Mode))
		if err != nil {
			return err
		}

		_, err = io.Copy(fw, tr)
		if err != nil {
			return err
		}

		fw.Close()
	}
	return nil
}

// ToFile the function for saving db to File.
func ToFile(conf *config.Config) error {
	src := filepath.Join(conf.DB.LdbPath, "StateDB")

	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}
	db, err := leveldb.NewDB(src)
	if err != nil {
		return err
	}
	defer db.Close()

	file, err := os.OpenFile(filepath.Join(conf.DB.LdbPath, "Snapshot.iost"), os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		return err
	}
	defer file.Close()
	iter := db.NewIteratorByPrefix([]byte("")).(*leveldb.Iter)
	writer := bufio.NewWriterSize(file, 100000)
	for iter.Next() {
		k := string(iter.Key())
		v := string(iter.Value())
		writer.WriteString(k + "\n")
		writer.WriteString(v + "\n")
	}
	writer.Flush()
	return nil
}

// FromFile the function for loading db from File.
func FromFile() error {
	return nil
}
