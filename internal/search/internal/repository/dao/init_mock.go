//go:build e2e || mock

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
	"context"
	_ "embed"
	"time"

	"github.com/olivere/elastic/v7"
	"golang.org/x/sync/errgroup"
)

var (
	//go:embed case_test_index.json
	testCaseIndex string
	//go:embed question_test_index.json
	testQuestionIndex string
	//go:embed skill_test_index.json
	testSkillIndex string
	//go:embed questionset_test_index.json
	testQuestionSetIndex string
)

func InitES(client *elastic.Client) error {
	const timeout = time.Second * 10
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var eg errgroup.Group
	eg.Go(func() error {
		return tryCreateIndex(ctx, client, PubCaseIndexName, testCaseIndex)
	})
	eg.Go(func() error {
		return tryCreateIndex(ctx, client, CaseIndexName, testCaseIndex)
	})
	eg.Go(func() error {
		return tryCreateIndex(ctx, client, PubQuestionIndexName, testQuestionIndex)
	})

	eg.Go(func() error {
		return tryCreateIndex(ctx, client, QuestionIndexName, testQuestionIndex)
	})
	eg.Go(func() error {
		return tryCreateIndex(ctx, client, SkillIndexName, testSkillIndex)
	})
	eg.Go(func() error {
		return tryCreateIndex(ctx, client, QuestionSetIndexName, testQuestionSetIndex)
	})
	return eg.Wait()
}
