// Copyright 2021 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
)

func TestBlock(t *testing.T) {
	mu := &sync.RWMutex{}
	ts1 := NowMicro()
	time.Sleep(time.Microsecond)
	info1 := MockInfo(mu, 2, 2)
	info2 := MockInfo(mu, 1000, 4)
	schema1 := MockSchema(2)
	schema2 := MockVarCharSchema(2)

	tbl1 := NewTable(NextGlobalSeqNum(), info1, schema1, 1)
	seg1 := NewSegment(tbl1, info1.Sequence.GetSegmentID())
	blk1 := NewBlock(info1.Sequence.GetBlockID(), seg1)
	time.Sleep(time.Duration(1) * time.Microsecond)
	ts2 := NowMicro()

	tbl2 := NewTable(NextGlobalSeqNum(), info2, schema2, 2)
	seg2 := NewSegment(tbl2, info2.Sequence.GetSegmentID())
	blk2 := NewBlock(info2.Sequence.GetBlockID(), seg2)
	time.Sleep(time.Duration(1) * time.Microsecond)
	ts3 := NowMicro()
	assert.Equal(t, "Blk(1-1-1)(DataState=0)", blk1.String())

	assert.False(t, blk1.Select(ts1))
	assert.True(t, blk1.Select(ts2))
	assert.Nil(t, blk1.Delete(ts3))

	time.Sleep(time.Duration(1) * time.Microsecond)
	ts4 := NowMicro()
	assert.False(t, blk1.IsFull())
	assert.Nil(t, blk1.SetCount(1))
	assert.False(t, blk1.TryUpgrade())
	_, err := blk1.AddCount(1)
	assert.Nil(t, err)
	_, err = blk1.AddCount(1)
	assert.NotNil(t, err)
	assert.True(t, blk1.TryUpgrade())
	assert.True(t, blk1.IsFull())
	assert.Equal(t, "Blk(1-1-1)(DataState=2)[D][F]", blk1.String())
	assert.True(t, blk1.IsDeleted(ts4))
	assert.False(t, blk1.Select(ts4))
	assert.True(t, blk1.Select(ts2))

	assert.Equal(t, uint64(1), blk2.GetID())
	assert.Equal(t, uint64(1), blk2.GetSegmentID())
	assert.Equal(t, Standalone, blk2.GetBoundState())

	assert.Equal(t, blk1.MaxRowCount*4*2, EstimateBlockSize(blk1))

	assert.Nil(t, blk2.GetReplayIndex())
	_, has := blk2.GetAppliedIndex()
	assert.False(t, has)

	blk3 := blk2.Copy()
	assert.NotNil(t, blk1.Update(blk3))
	blk3.Segment.Table.ID = blk1.Segment.Table.ID
	assert.NotNil(t, blk1.Update(blk3))
	blk3.MaxRowCount = blk1.MaxRowCount
	assert.NotNil(t, blk1.Update(blk3))
	blk3.DataState = blk1.DataState
	assert.NotNil(t, blk1.Update(blk3))
	assert.Nil(t, blk3.SetCount(blk1.GetCount()))
	assert.Nil(t, blk1.Update(blk3))

	assert.Nil(t, blk2.SetCount(1))
	assert.Equal(t, blk2.DataState, PARTIAL)

	assert.Equal(t, blk2.ID, blk2.copyNoLock(nil).ID)
	assert.NotNil(t, blk2.SetCount(0))
	assert.NotNil(t, blk2.SetCount(88888))
	assert.Nil(t, blk2.SetCount(1000))
	assert.Equal(t, blk2.DataState, FULL)

	assert.Equal(t, blk2.ID, blk2.AsCommonID().BlockID)
	n, _ := blk1.Marshal()
	t.Log(string(n))
	assert.Equal(t, 149, len(n))

	assert.Equal(t, blk2.MaxRowCount*8*2, EstimateBlockSize(blk2))

	blk4 := NewBlock(info2.Sequence.GetBlockID(), seg2)
	pool, _ := ants.NewPool(20)
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		pool.Submit(func() {
			blk4.AddCount(1)
			wg.Done()
		})
	}
	wg.Wait()
	assert.Equal(t, uint64(1000), blk4.GetCount())
}
