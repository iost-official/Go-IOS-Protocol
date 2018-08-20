package blockcache

import (
	"testing"
	//	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/iost-official/Go-IOS-Protocol/core/mocks"
	"github.com/iost-official/Go-IOS-Protocol/db/mocks"

	"github.com/iost-official/Go-IOS-Protocol/core/new_block"
	. "github.com/smartystreets/goconvey/convey"
)

func genBlock(fa *block.Block, wit string, num uint64) *block.Block {
	ret := &block.Block{
		Head: block.BlockHead{
			Witness: wit,
			Number:  int64(num),
		},
	}
	if fa == nil {
		ret.Head.ParentHash = []byte("Im a single block")
	} else {
		ret.Head.ParentHash = fa.HeadHash()
	}
	ret.CalculateHeadHash()
	return ret
}
func TestBlockCache(t *testing.T) {
	ctl := gomock.NewController(t)
	b0 := &block.Block{
		Head: block.BlockHead{
			Version:    0,
			ParentHash: []byte("nothing"),
			Witness:    "w0",
			Number:     0,
		},
	}

	b1 := genBlock(b0, "w1", 1)
	b2 := genBlock(b1, "w2", 2)
	b2a := genBlock(b1, "w3", 3)
	b3 := genBlock(b2, "w4", 4)
	b4 := genBlock(b2a, "w5", 5)
	b3a := genBlock(b2, "w6", 6)
	b5 := genBlock(b3a, "w7", 7)

	s1 := genBlock(nil, "w1", 1)
	s2 := genBlock(s1, "w2", 2)
	s2a := genBlock(s1, "w3", 3)
	s3 := genBlock(s2, "w4", 4)

	txdb := core_mock.NewMockTxDB(ctl)
	txdb.EXPECT().Push(gomock.Any()).AnyTimes().Return(nil)
	statedb := db_mock.NewMockMVCCDB(ctl)
	statedb.EXPECT().Flush(gomock.Any()).AnyTimes().Return(nil)
	base := core_mock.NewMockChain(ctl)
	base.EXPECT().Top().AnyTimes().Return(b0, nil)
	base.EXPECT().Push(gomock.Any()).AnyTimes().Return(nil)
	global := core_mock.NewMockBaseVariable(ctl)
	global.EXPECT().BlockChain().AnyTimes().Return(base)
	global.EXPECT().TxDB().AnyTimes().Return(txdb)
	global.EXPECT().StateDB().AnyTimes().Return(statedb)
	Convey("Test of Block Cache", t, func() {
		Convey("Add:", func() {
			bc, _ := NewBlockCache(global)
			//fmt.Printf("Leaf:%+v\n",bc.Leaf)
			_ = bc.Add(b1)
			//fmt.Printf("Leaf:%+v\n",bc.Leaf)
			//bc.Draw()
			bc.Add(b2)
		})

		Convey("Flush", func() {
			bc, _ := NewBlockCache(global)
			bc.Add(b1)
			//bc.Draw()
			bc.Add(b2)
			//bc.Draw()
			bc.Add(b2a)
			//bc.Draw()
			bc.Add(b3)
			//bc.Draw()
			//b4node, _ := bc.Add(b4)
			//bc.Draw()
			bc.Add(b3a)
			//bc.Draw()
			bc.Add(b5)
			//bc.Draw()

			bc.Add(s1)
			bc.Add(s2)
			bc.Add(s2a)
			bc.Add(s3)
			//bc.Draw()
			//bc.Flush(b4node)
			//bc.Draw()

		})

		Convey("GetBlockbyNumber", func() {
			bc, _ := NewBlockCache(global)
			b1node := bc.Add(b1)
			//bc.Draw()
			b2node := bc.Add(b2)
			//bc.Draw()
			bc.Add(b2a)
			//bc.Draw()
			bc.Add(b3)
			//bc.Draw()
			b4node := bc.Add(b4)
			//bc.Draw()
			b3anode := bc.Add(b3a)
			//bc.Draw()
			b5node := bc.Add(b5)
			//bc.Draw()
			So(bc.head, ShouldEqual, b5node)
			blk, _ := bc.GetBlockByNumber(7)
			So(blk, ShouldEqual, b5node.Block)
			blk, _ = bc.GetBlockByNumber(6)
			So(blk, ShouldEqual, b3anode.Block)
			blk, _ = bc.GetBlockByNumber(2)
			So(blk, ShouldEqual, b2node.Block)
			blk, _ = bc.GetBlockByNumber(1)
			So(blk, ShouldEqual, b1node.Block)
			blk, _ = bc.GetBlockByNumber(4)
			So(blk, ShouldEqual, nil)

			bc.Flush(b4node)
			//bc.Draw()

		})

	})
}
