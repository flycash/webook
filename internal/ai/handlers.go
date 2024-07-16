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

package ai

import (
	"github.com/ecodeclub/webook/internal/ai/internal/service/gpt/handler"
	"github.com/ecodeclub/webook/internal/ai/internal/service/gpt/handler/biz"
	"github.com/ecodeclub/webook/internal/ai/internal/service/gpt/handler/config"
	"github.com/ecodeclub/webook/internal/ai/internal/service/gpt/handler/credit"
	"github.com/ecodeclub/webook/internal/ai/internal/service/gpt/handler/gpt/zhipu"
	"github.com/ecodeclub/webook/internal/ai/internal/service/gpt/handler/log"
	"github.com/ecodeclub/webook/internal/ai/internal/service/gpt/handler/record"
	"github.com/gotomicro/ego/core/econf"
)

func InitHandlerFacade(common []handler.Builder, zhipu *zhipu.Handler) *biz.FacadeHandler {
	que := InitQuestionExamineHandler(common, zhipu)
	return biz.NewHandler(map[string]handler.Handler{
		que.Biz(): que,
	})
}

func InitZhipu() *zhipu.Handler {
	type Config struct {
		APIKey string  `yaml:"apikey"`
		Price  float64 `yaml:"price"`
	}
	var cfg Config
	err := econf.UnmarshalKey("zhipu", &cfg)
	if err != nil {
		panic(err)
	}
	h, err := zhipu.NewHandler(cfg.APIKey, cfg.Price)
	if err != nil {
		panic(err)
	}
	return h
}

func InitQuestionExamineHandler(
	common []handler.Builder,
	// gpt 就是真正的出口
	gpt handler.Handler) *biz.CompositionHandler {
	// log -> cfg -> credit -> record -> question_examine -> gpt
	builder := biz.NewQuestionExamineBizHandlerBuilder()
	common = append(common, builder)
	res := biz.NewCombinedBizHandler("question_examine", common, gpt)
	return res
}

func InitCommonHandlers(log *log.HandlerBuilder,
	cfg *config.HandlerBuilder,
	credit *credit.HandlerBuilder,
	record *record.HandlerBuilder) []handler.Builder {
	return []handler.Builder{log, cfg, credit, record}
}
