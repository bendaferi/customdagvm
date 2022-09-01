// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

type StaticService struct{}

func CreateStaticService() *StaticService {
	return &StaticService{}
}