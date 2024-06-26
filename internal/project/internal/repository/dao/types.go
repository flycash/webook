// Copyright 2023 ecodeclub
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dao

import (
	"database/sql"

	"github.com/ecodeclub/ekit/sqlx"
)

type Project struct {
	Id           int64  `gorm:"primaryKey,autoIncrement"`
	Title        string `gorm:"type:varchar(256)"`
	Overview     string
	SystemDesign string
	GithubRepo   string `gorm:"type:varchar(512)"`
	GiteeRepo    string `gorm:"type:varchar(512)"`
	// 目前来说项目关联的八股文题集，并不需要额外的信息
	RefQuestionSet int64
	Status         uint8
	SN             string                    `gorm:"column:sn;type:varchar(255)"`
	Labels         sqlx.JsonColumn[[]string] `gorm:"type:varchar(512)"`
	ProductSPU     sql.NullString            `gorm:"type:varchar(255);comment:商品的SPU SN"`
	CodeSPU        sql.NullString            `gorm:"type:varchar(255);comment:作为兑换码的SPU SN"`
	Desc           string
	Utime          int64
	Ctime          int64
}

type PubProject Project

type ProjectDifficulty struct {
	Id    int64  `gorm:"primaryKey,autoIncrement"`
	Pid   int64  `gorm:"index"`
	Title string `gorm:"type:varchar(256)"`
	// 这是面试时候的介绍这个项目难点
	Content  string `json:"content,omitempty"`
	Analysis string
	Status   uint8
	Utime    int64
	Ctime    int64
}

type PubProjectDifficulty ProjectDifficulty

type ProjectResume struct {
	Id       int64 `gorm:"primaryKey,autoIncrement"`
	Pid      int64 `gorm:"index"`
	Role     uint8
	Content  string
	Analysis string
	Status   uint8
	Utime    int64
	Ctime    int64
}

type PubProjectResume ProjectResume

type ProjectIntroduction struct {
	Id       int64 `gorm:"primaryKey,autoIncrement"`
	Pid      int64 `gorm:"index"`
	Role     uint8
	Content  string
	Analysis string
	Status   uint8
	Utime    int64
	Ctime    int64
}

type PubProjectIntroduction ProjectIntroduction

type PubProjectQuestion ProjectQuestion

type ProjectQuestion struct {
	Id       int64 `gorm:"primaryKey,autoIncrement"`
	Pid      int64 `gorm:"index"`
	Title    string
	Analysis string
	Answer   string
	Status   uint8
	Utime    int64
	Ctime    int64
}

type ProjectCombo struct {
	Id      int64  `gorm:"primaryKey,autoIncrement"`
	Pid     int64  `gorm:"index"`
	Title   string `gorm:"type:varchar(256)"`
	Content string
	Status  uint8
	Utime   int64
	Ctime   int64
}

type PubProjectCombo ProjectCombo
