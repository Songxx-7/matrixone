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

package mergegroup

import (
	"github.com/matrixorigin/matrixone/pkg/common/hashmap"
	"github.com/matrixorigin/matrixone/pkg/common/mpool"
	"github.com/matrixorigin/matrixone/pkg/common/reuse"
	"github.com/matrixorigin/matrixone/pkg/container/batch"
	"github.com/matrixorigin/matrixone/pkg/container/types"
	"github.com/matrixorigin/matrixone/pkg/sql/colexec"
	"github.com/matrixorigin/matrixone/pkg/vm"
	"github.com/matrixorigin/matrixone/pkg/vm/process"
)

var _ vm.Operator = new(MergeGroup)

const (
	Build = iota
	Eval
	End
)

const (
	H0 = iota
	H8
	HStr
)

type container struct {
	state int
	itr   hashmap.Iterator
	// should use hash map or not and the hash map type.
	typ int

	// hash map related.
	hashKeyWidth   int
	groupByCol     int
	keyNullability bool
	intHashMap     *hashmap.IntHashMap
	strHashMap     *hashmap.StrHashMap
	inserted       []uint8
	zInserted      []uint8

	bat *batch.Batch
}

type MergeGroup struct {
	NeedEval bool // need to projection the aggregate column
	ctr      container

	PartialResults     []any
	PartialResultTypes []types.T

	vm.OperatorBase
	colexec.Projection
}

func (mergeGroup *MergeGroup) GetOperatorBase() *vm.OperatorBase {
	return &mergeGroup.OperatorBase
}

func init() {
	reuse.CreatePool[MergeGroup](
		func() *MergeGroup {
			return &MergeGroup{}
		},
		func(a *MergeGroup) {
			*a = MergeGroup{}
		},
		reuse.DefaultOptions[MergeGroup]().
			WithEnableChecker(),
	)
}

func (mergeGroup MergeGroup) TypeName() string {
	return opName
}

func (mergeGroup *MergeGroup) ExecProjection(proc *process.Process, input *batch.Batch) (*batch.Batch, error) {
	var err error
	batch := input
	if mergeGroup.ProjectList != nil {
		batch, err = mergeGroup.EvalProjection(input, proc)
	}
	return batch, err
}

func (mergeGroup *MergeGroup) WithNeedEval(needEval bool) *MergeGroup {
	mergeGroup.NeedEval = needEval
	return mergeGroup
}

func (ctr *container) cleanBatch(mp *mpool.MPool) {
	if ctr.bat != nil {
		ctr.bat.Clean(mp)
		ctr.bat = nil
	}
}

func (ctr *container) cleanHashMap() {
	if ctr.intHashMap != nil {
		ctr.intHashMap.Free()
		ctr.intHashMap = nil
	}
	if ctr.strHashMap != nil {
		ctr.strHashMap.Free()
		ctr.strHashMap = nil
	}
}
