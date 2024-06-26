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

//go:build e2e

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ecodeclub/webook/internal/interactive"
	intrmocks "github.com/ecodeclub/webook/internal/interactive/mocks"

	"github.com/ecodeclub/webook/internal/question/internal/event"
	eveMocks "github.com/ecodeclub/webook/internal/question/internal/event/mocks"
	"go.uber.org/mock/gomock"

	"github.com/ecodeclub/webook/internal/question/internal/domain"

	"gorm.io/gorm"

	"github.com/ecodeclub/ekit/sqlx"
	"github.com/ecodeclub/webook/internal/pkg/middleware"

	"github.com/ecodeclub/ecache"
	"github.com/ecodeclub/ekit/iox"
	"github.com/ecodeclub/ginx/session"
	"github.com/ecodeclub/webook/internal/question/internal/integration/startup"
	"github.com/ecodeclub/webook/internal/question/internal/repository/dao"
	"github.com/ecodeclub/webook/internal/question/internal/web"
	"github.com/ecodeclub/webook/internal/test"
	testioc "github.com/ecodeclub/webook/internal/test/ioc"
	"github.com/ego-component/egorm"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/server/egin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const uid = 123

type HandlerTestSuite struct {
	suite.Suite
	server         *egin.Component
	db             *egorm.Component
	rdb            ecache.Cache
	dao            dao.QuestionDAO
	questionSetDAO dao.QuestionSetDAO
	ctrl           *gomock.Controller
	producer       *eveMocks.MockSyncEventProducer
}

func (s *HandlerTestSuite) TearDownSuite() {
	err := s.db.Exec("DROP TABLE `answer_elements`").Error
	require.NoError(s.T(), err)
	err = s.db.Exec("DROP TABLE `questions`").Error
	require.NoError(s.T(), err)

	err = s.db.Exec("DROP TABLE `publish_answer_elements`").Error
	require.NoError(s.T(), err)
	err = s.db.Exec("DROP TABLE `publish_questions`").Error
	require.NoError(s.T(), err)

	err = s.db.Exec("DROP TABLE `question_sets`").Error
	require.NoError(s.T(), err)

	err = s.db.Exec("DROP TABLE `question_set_questions`").Error
	require.NoError(s.T(), err)
}

func (s *HandlerTestSuite) TearDownTest() {
	err := s.db.Exec("TRUNCATE TABLE `answer_elements`").Error
	require.NoError(s.T(), err)
	err = s.db.Exec("TRUNCATE TABLE `questions`").Error
	require.NoError(s.T(), err)

	err = s.db.Exec("TRUNCATE TABLE `publish_answer_elements`").Error
	require.NoError(s.T(), err)
	err = s.db.Exec("TRUNCATE TABLE `publish_questions`").Error
	require.NoError(s.T(), err)

	err = s.db.Exec("TRUNCATE TABLE `question_sets`").Error
	require.NoError(s.T(), err)

	err = s.db.Exec("TRUNCATE TABLE `question_set_questions`").Error
	require.NoError(s.T(), err)
}

func (s *HandlerTestSuite) SetupSuite() {
	s.ctrl = gomock.NewController(s.T())
	s.producer = eveMocks.NewMockSyncEventProducer(s.ctrl)

	intrSvc := intrmocks.NewMockService(s.ctrl)
	intrModule := &interactive.Module{
		Svc: intrSvc,
	}

	// 模拟返回的数据
	// 使用如下规律:
	// 1. liked == id % 2 == 1 (奇数为 true)
	// 2. collected = id %2 == 0 (偶数为 true)
	// 3. viewCnt = id + 1
	// 4. likeCnt = id + 2
	// 5. collectCnt = id + 3
	intrSvc.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().DoAndReturn(func(ctx context.Context,
		biz string, id int64, uid int64) (interactive.Interactive, error) {
		intr := s.mockInteractive(biz, id)
		return intr, nil
	})
	intrSvc.EXPECT().GetByIds(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context,
		biz string, ids []int64) (map[int64]interactive.Interactive, error) {
		res := make(map[int64]interactive.Interactive, len(ids))
		for _, id := range ids {
			intr := s.mockInteractive(biz, id)
			res[id] = intr
		}
		return res, nil
	}).AnyTimes()

	handler, err := startup.InitHandler(s.producer, intrModule)
	require.NoError(s.T(), err)
	require.NoError(s.T(), err)
	questionSetHandler, err := startup.InitQuestionSetHandler(s.producer, intrModule)
	require.NoError(s.T(), err)
	econf.Set("server", map[string]any{"contextTimeout": "1s"})
	server := egin.Load("server").Build()

	handler.PublicRoutes(server.Engine)
	questionSetHandler.PublicRoutes(server.Engine)
	server.Use(func(ctx *gin.Context) {
		ctx.Set("_session", session.NewMemorySession(session.Claims{
			Uid: uid,
			Data: map[string]string{
				"creator":   "true",
				"memberDDL": strconv.FormatInt(time.Now().Add(time.Hour).UnixMilli(), 10),
			},
		}))
	})
	handler.PrivateRoutes(server.Engine)
	questionSetHandler.PrivateRoutes(server.Engine)
	server.Use(middleware.NewCheckMembershipMiddlewareBuilder(nil).Build())
	handler.MemberRoutes(server.Engine)

	s.server = server
	s.db = testioc.InitDB()
	err = dao.InitTables(s.db)
	require.NoError(s.T(), err)
	s.dao = dao.NewGORMQuestionDAO(s.db)
	s.questionSetDAO = dao.NewGORMQuestionSetDAO(s.db)
	s.rdb = testioc.InitCache()
}

func (s *HandlerTestSuite) TestSave() {
	testCases := []struct {
		name   string
		before func(t *testing.T)
		after  func(t *testing.T)
		req    web.SaveReq

		wantCode int
		wantResp test.Result[int64]
	}{
		{
			//
			name: "全部新建",
			before: func(t *testing.T) {
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				q, eles, err := s.dao.GetByID(ctx, 1)
				require.NoError(t, err)
				s.assertQuestion(t, dao.Question{
					Uid:     uid,
					Title:   "面试题1",
					Content: "面试题内容",
					Status:  domain.UnPublishedStatus.ToUint8(),
					Labels: sqlx.JsonColumn[[]string]{
						Valid: true,
						Val:   []string{"MySQL"},
					},
				}, q)
				assert.Equal(t, 4, len(eles))
				wantEles := []dao.AnswerElement{
					s.buildDAOAnswerEle(1, 0, dao.AnswerElementTypeAnalysis),
					s.buildDAOAnswerEle(1, 1, dao.AnswerElementTypeBasic),
					s.buildDAOAnswerEle(1, 2, dao.AnswerElementTypeIntermedia),
					s.buildDAOAnswerEle(1, 3, dao.AnswerElementTypeAdvanced),
				}
				for i := range eles {
					ele := &(eles[i])
					assert.True(t, ele.Id > 0)
					assert.True(t, ele.Ctime > 0)
					assert.True(t, ele.Utime > 0)
					ele.Id = 0
					ele.Ctime = 0
					ele.Utime = 0
				}
				assert.ElementsMatch(t, wantEles, eles)
			},
			req: web.SaveReq{
				Question: web.Question{
					Title:        "面试题1",
					Content:      "面试题内容",
					Labels:       []string{"MySQL"},
					Analysis:     s.buildAnswerEle(0),
					Basic:        s.buildAnswerEle(1),
					Intermediate: s.buildAnswerEle(2),
					Advanced:     s.buildAnswerEle(3),
				},
			},
			wantCode: 200,
			wantResp: test.Result[int64]{
				Data: 1,
			},
		},
		{
			//
			name: "部分更新",
			before: func(t *testing.T) {
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err := s.db.WithContext(ctx).Create(&dao.Question{
					Id:      2,
					Uid:     uid,
					Title:   "老的标题",
					Content: "老的内容",
					Status:  domain.UnPublishedStatus.ToUint8(),
					Ctime:   123,
					Utime:   234,
				}).Error
				require.NoError(t, err)
				err = s.db.Create(&dao.AnswerElement{
					Id:        1,
					Qid:       2,
					Type:      dao.AnswerElementTypeAnalysis,
					Content:   "老的分析",
					Keywords:  "老的 keyword",
					Shorthand: "老的速记",
					Highlight: "老的亮点",
					Guidance:  "老的引导点",
					Ctime:     123,
					Utime:     123,
				}).Error
				require.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				q, eles, err := s.dao.GetByID(ctx, 2)
				require.NoError(t, err)
				s.assertQuestion(t, dao.Question{
					Uid:     uid,
					Status:  domain.UnPublishedStatus.ToUint8(),
					Title:   "面试题1",
					Content: "新的内容",
				}, q)
				assert.Equal(t, 4, len(eles))
				analysis := eles[0]
				s.assertAnswerElement(t, dao.AnswerElement{
					Content:   "新的分析",
					Type:      dao.AnswerElementTypeAnalysis,
					Qid:       2,
					Keywords:  "新的 keyword",
					Shorthand: "新的速记",
					Highlight: "新的亮点",
					Guidance:  "新的引导点",
				}, analysis)
			},
			req: func() web.SaveReq {
				analysis := web.AnswerElement{
					Id:        1,
					Content:   "新的分析",
					Keywords:  "新的 keyword",
					Shorthand: "新的速记",
					Highlight: "新的亮点",
					Guidance:  "新的引导点",
				}
				return web.SaveReq{
					Question: web.Question{
						Id:           2,
						Title:        "面试题1",
						Content:      "新的内容",
						Analysis:     analysis,
						Basic:        s.buildAnswerEle(1),
						Intermediate: s.buildAnswerEle(2),
						Advanced:     s.buildAnswerEle(3),
					},
				}
			}(),
			wantCode: 200,
			wantResp: test.Result[int64]{
				Data: 2,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)
			req, err := http.NewRequest(http.MethodPost,
				"/question/save", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[int64]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
			tc.after(t)
			// 清理掉 123 的数据
			err = s.db.Exec("TRUNCATE table `questions`").Error
			require.NoError(t, err)
			err = s.db.Exec("TRUNCATE table `answer_elements`").Error
			require.NoError(t, err)
		})
	}
}

func (s *HandlerTestSuite) TestPubList() {
	// 插入一百条
	data := make([]dao.PublishQuestion, 0, 100)
	for idx := 0; idx < 100; idx++ {
		data = append(data, dao.PublishQuestion{
			Uid:     uid,
			Status:  domain.UnPublishedStatus.ToUint8(),
			Title:   fmt.Sprintf("这是标题 %d", idx),
			Content: fmt.Sprintf("这是解析 %d", idx),
			Utime:   123,
		})
	}
	err := s.db.Create(&data).Error
	require.NoError(s.T(), err)
	testCases := []struct {
		name string
		req  web.Page

		wantCode int
		wantResp test.Result[[]web.Question]
	}{
		{
			name: "获取成功",
			req: web.Page{
				Limit:  2,
				Offset: 0,
			},
			wantCode: 200,
			wantResp: test.Result[[]web.Question]{
				Data: []web.Question{
					{
						Id:      100,
						Title:   "这是标题 99",
						Content: "这是解析 99",
						Status:  domain.UnPublishedStatus.ToUint8(),
						Utime:   123,
						Interactive: web.Interactive{
							ViewCnt:    101,
							LikeCnt:    102,
							CollectCnt: 103,
							Liked:      false,
							Collected:  true,
						},
					},
					{
						Id:      99,
						Title:   "这是标题 98",
						Content: "这是解析 98",
						Status:  domain.UnPublishedStatus.ToUint8(),
						Utime:   123,
						Interactive: web.Interactive{
							ViewCnt:    100,
							LikeCnt:    101,
							CollectCnt: 102,
							Liked:      true,
							Collected:  false,
						},
					},
				},
			},
		},
		{
			name: "获取部分",
			req: web.Page{
				Limit:  2,
				Offset: 99,
			},
			wantCode: 200,
			wantResp: test.Result[[]web.Question]{
				Data: []web.Question{
					{
						Id:      1,
						Title:   "这是标题 0",
						Content: "这是解析 0",
						Status:  domain.UnPublishedStatus.ToUint8(),
						Utime:   123,
						Interactive: web.Interactive{
							ViewCnt:    2,
							LikeCnt:    3,
							CollectCnt: 4,
							Liked:      true,
							Collected:  false,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost,
				"/question/pub/list", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[[]web.Question]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = s.rdb.Delete(ctx, "question:total")
	require.NoError(s.T(), err)
}

func (s *HandlerTestSuite) TestSync() {
	testCases := []struct {
		name   string
		before func(t *testing.T)
		after  func(t *testing.T)
		req    web.SaveReq

		wantCode int
		wantResp test.Result[int64]
	}{
		{
			//
			name: "全部新建",
			before: func(t *testing.T) {
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				q, eles, err := s.dao.GetPubByID(ctx, 1)
				require.NoError(t, err)
				s.assertQuestion(t, dao.Question{
					Uid:     uid,
					Title:   "面试题1",
					Status:  domain.PublishedStatus.ToUint8(),
					Content: "面试题内容",
				}, dao.Question(q))
				assert.Equal(t, 4, len(eles))
			},
			req: web.SaveReq{
				Question: web.Question{
					Title:        "面试题1",
					Content:      "面试题内容",
					Analysis:     s.buildAnswerEle(0),
					Basic:        s.buildAnswerEle(1),
					Intermediate: s.buildAnswerEle(2),
					Advanced:     s.buildAnswerEle(3),
				},
			},
			wantCode: 200,
			wantResp: test.Result[int64]{
				Data: 1,
			},
		},
		{
			//
			name: "部分更新",
			before: func(t *testing.T) {
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err := s.db.WithContext(ctx).Create(&dao.Question{
					Id:      2,
					Uid:     uid,
					Title:   "老的标题",
					Content: "老的内容",

					Ctime: 123,
					Utime: 234,
				}).Error
				require.NoError(t, err)
				err = s.db.Create(&dao.AnswerElement{
					Id:        1,
					Qid:       2,
					Type:      dao.AnswerElementTypeAnalysis,
					Content:   "老的分析",
					Keywords:  "老的 keyword",
					Shorthand: "老的速记",
					Highlight: "老的亮点",
					Guidance:  "老的引导点",
					Ctime:     123,
					Utime:     123,
				}).Error
				require.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				q, eles, err := s.dao.GetByID(ctx, 2)
				require.NoError(t, err)
				s.assertQuestion(t, dao.Question{
					Uid:     uid,
					Status:  domain.PublishedStatus.ToUint8(),
					Title:   "面试题1",
					Content: "新的内容",
				}, q)
				assert.Equal(t, 4, len(eles))
				analysis := eles[0]
				s.assertAnswerElement(t, dao.AnswerElement{
					Content:   "新的分析",
					Type:      dao.AnswerElementTypeAnalysis,
					Qid:       2,
					Keywords:  "新的 keyword",
					Shorthand: "新的速记",
					Highlight: "新的亮点",
					Guidance:  "新的引导点",
				}, analysis)

				pq, pEles, err := s.dao.GetPubByID(ctx, 2)

				s.assertQuestion(t, dao.Question{
					Uid:     uid,
					Status:  domain.PublishedStatus.ToUint8(),
					Title:   "面试题1",
					Content: "新的内容",
				}, dao.Question(pq))
				assert.Equal(t, 4, len(pEles))
				pAnalysis := pEles[0]
				s.assertAnswerElement(t, dao.AnswerElement{
					Content:   "新的分析",
					Type:      dao.AnswerElementTypeAnalysis,
					Qid:       2,
					Keywords:  "新的 keyword",
					Shorthand: "新的速记",
					Highlight: "新的亮点",
					Guidance:  "新的引导点",
				}, dao.AnswerElement(pAnalysis))
			},
			req: func() web.SaveReq {
				analysis := web.AnswerElement{
					Id:        1,
					Content:   "新的分析",
					Keywords:  "新的 keyword",
					Shorthand: "新的速记",
					Highlight: "新的亮点",
					Guidance:  "新的引导点",
				}
				return web.SaveReq{
					Question: web.Question{
						Id:           2,
						Title:        "面试题1",
						Content:      "新的内容",
						Analysis:     analysis,
						Basic:        s.buildAnswerEle(1),
						Intermediate: s.buildAnswerEle(2),
						Advanced:     s.buildAnswerEle(3),
					},
				}
			}(),
			wantCode: 200,
			wantResp: test.Result[int64]{
				Data: 2,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)
			req, err := http.NewRequest(http.MethodPost,
				"/question/publish", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[int64]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
			tc.after(t)
			// 清理掉 123 的数据
			err = s.db.Exec("TRUNCATE table `questions`").Error
			require.NoError(t, err)
			err = s.db.Exec("TRUNCATE table `answer_elements`").Error
			require.NoError(t, err)
		})
	}
}

func (s *HandlerTestSuite) TestDelete() {
	testCases := []struct {
		name string

		qid    int64
		before func(t *testing.T)
		after  func(t *testing.T)

		wantCode int
		wantResp test.Result[any]
	}{
		{
			name: "删除成功",
			qid:  123,
			before: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				var qid int64 = 123
				// prepare data
				_, err := s.dao.Sync(ctx, dao.Question{
					Id: qid,
				}, []dao.AnswerElement{
					{
						Qid: qid,
					},
				})
				require.NoError(t, err)
				err = s.db.Create(&dao.QuestionSetQuestion{
					QID: qid,
				}).Error
				require.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				var qid int64 = 123
				_, _, err := s.dao.GetPubByID(ctx, qid)
				assert.Equal(t, err, gorm.ErrRecordNotFound)
				_, _, err = s.dao.GetByID(ctx, qid)
				assert.Equal(t, err, gorm.ErrRecordNotFound)
				var res []dao.QuestionSetQuestion
				err = s.db.Where("qid = ?").Find(&res).Error
				assert.NoError(t, err)
				assert.Equal(t, 0, len(res))
			},
			wantCode: 200,
			wantResp: test.Result[any]{},
		},
		{
			name: "删除不存在的 Question",
			qid:  124,
			before: func(t *testing.T) {
			},
			after: func(t *testing.T) {
			},
			wantCode: 200,
			wantResp: test.Result[any]{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost,
				"/question/delete", iox.NewJSONReader(web.Qid{Qid: tc.qid}))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[any]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
		})
	}

}

func (s *HandlerTestSuite) TestPubDetail() {
	// 插入一百条
	data := make([]dao.PublishQuestion, 0, 2)
	for idx := 0; idx < 2; idx++ {
		data = append(data, dao.PublishQuestion{
			Id:      int64(idx + 1),
			Uid:     uid,
			Status:  domain.PublishedStatus.ToUint8(),
			Title:   fmt.Sprintf("这是标题 %d", idx),
			Content: fmt.Sprintf("这是解析 %d", idx),
		})
	}
	err := s.db.Create(&data).Error
	require.NoError(s.T(), err)
	testCases := []struct {
		name string

		req      web.Qid
		wantCode int
		wantResp test.Result[web.Question]
	}{
		{
			name: "查询到了数据",
			req: web.Qid{
				Qid: 2,
			},
			wantCode: 200,
			wantResp: test.Result[web.Question]{
				Data: web.Question{
					Id:      2,
					Title:   "这是标题 1",
					Status:  domain.PublishedStatus.ToUint8(),
					Content: "这是解析 1",
					Utime:   0,
					Interactive: web.Interactive{
						ViewCnt:    3,
						LikeCnt:    4,
						CollectCnt: 5,
						Liked:      false,
						Collected:  true,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost,
				"/question/pub/detail", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[web.Question]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
		})
	}
}

func (s *HandlerTestSuite) buildDAOAnswerEle(
	qid int64,
	idx int,
	typ uint8) dao.AnswerElement {
	return dao.AnswerElement{
		Qid:       qid,
		Type:      typ,
		Content:   fmt.Sprintf("这是解析 %d", idx),
		Keywords:  fmt.Sprintf("关键字 %d", idx),
		Shorthand: fmt.Sprintf("快速记忆法 %d", idx),
		Highlight: fmt.Sprintf("亮点 %d", idx),
		Guidance:  fmt.Sprintf("引导点 %d", idx),
	}
}

func (s *HandlerTestSuite) buildAnswerEle(idx int64) web.AnswerElement {
	return web.AnswerElement{
		Content:   fmt.Sprintf("这是解析 %d", idx),
		Keywords:  fmt.Sprintf("关键字 %d", idx),
		Shorthand: fmt.Sprintf("快速记忆法 %d", idx),
		Highlight: fmt.Sprintf("亮点 %d", idx),
		Guidance:  fmt.Sprintf("引导点 %d", idx),
	}
}

// assertQuestion 不比较 id
func (s *HandlerTestSuite) assertQuestion(t *testing.T, expect dao.Question, q dao.Question) {
	assert.True(t, q.Id > 0)
	assert.True(t, q.Ctime > 0)
	assert.True(t, q.Utime > 0)
	q.Id = 0
	q.Ctime = 0
	q.Utime = 0
	assert.Equal(t, expect, q)
}

// assertAnswerElement 不包括 Id
func (s *HandlerTestSuite) assertAnswerElement(
	t *testing.T,
	expect dao.AnswerElement,
	ele dao.AnswerElement) {
	assert.True(t, ele.Id > 0)
	ele.Id = 0
	assert.True(t, ele.Ctime > 0)
	ele.Ctime = 0
	assert.True(t, ele.Utime > 0)
	ele.Utime = 0
	assert.Equal(t, expect, ele)
}

func (s *HandlerTestSuite) TestQuestionSet_Save() {
	var testCases = []struct {
		name   string
		before func(t *testing.T)
		after  func(t *testing.T)
		req    web.SaveQuestionSetReq

		wantCode int
		wantResp test.Result[int64]
	}{
		{
			name: "创建成功1",
			before: func(t *testing.T) {
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
			},
			after: func(t *testing.T) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				qs, err := s.questionSetDAO.GetByID(ctx, 1)
				assert.NoError(t, err)

				s.assertQuestionSetEqual(t, dao.QuestionSet{
					Uid:         uid,
					Title:       "mysql",
					Description: "mysql相关面试题",
				}, qs)
			},
			req: web.SaveQuestionSetReq{
				Title:       "mysql",
				Description: "mysql相关面试题",
			},
			wantCode: 200,
			wantResp: test.Result[int64]{
				Data: 1,
			},
		},
		{
			name: "创建成功2",
			before: func(t *testing.T) {
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err := s.db.WithContext(ctx).Create(dao.QuestionSet{
					Id:          2,
					Uid:         uid,
					Title:       "老的 MySQL",
					Description: "老的 Desc",
					Ctime:       123,
					Utime:       123,
				}).Error
				require.NoError(t, err)
			},
			after: func(t *testing.T) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				qs, err := s.questionSetDAO.GetByID(ctx, 2)
				assert.NoError(t, err)
				s.assertQuestionSetEqual(t, dao.QuestionSet{
					Uid:         uid,
					Title:       "mq",
					Description: "mq相关面试题",
				}, qs)
			},
			req: web.SaveQuestionSetReq{
				Id:          2,
				Title:       "mq",
				Description: "mq相关面试题",
			},
			wantCode: 200,
			wantResp: test.Result[int64]{
				Data: 2,
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)
			targeURL := "/question-sets/save"
			req, err := http.NewRequest(http.MethodPost, targeURL, iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)

			recorder := test.NewJSONResponseRecorder[int64]()

			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
			tc.after(t)
		})
	}
}

// assertQuestionSetEqual 不比较 id
func (s *HandlerTestSuite) assertQuestionSetEqual(t *testing.T, expect dao.QuestionSet, actual dao.QuestionSet) {
	assert.True(t, actual.Id > 0)
	assert.True(t, actual.Ctime > 0)
	assert.True(t, actual.Utime > 0)
	actual.Id = 0
	actual.Ctime = 0
	actual.Utime = 0
	assert.Equal(t, expect, actual)
}

func (s *HandlerTestSuite) TestQuestionSet_UpdateQuestions() {
	testCases := []struct {
		name   string
		before func(t *testing.T)
		after  func(t *testing.T)
		req    web.UpdateQuestionsOfQuestionSetReq

		wantCode int
		wantResp test.Result[int64]
	}{
		{
			name: "空题集_添加多个问题",
			before: func(t *testing.T) {
				t.Helper()
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				// 创建一个空题集
				id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
					Id:          5,
					Uid:         uid,
					Title:       "oss",
					Description: "oss题集",
				})
				require.NoError(t, err)
				require.Equal(t, int64(5), id)

				// 创建问题
				questions := []dao.Question{
					{
						Id:      4,
						Uid:     uid + 1,
						Title:   "oss问题1",
						Status:  domain.UnPublishedStatus.ToUint8(),
						Content: "oss问题1",
						Ctime:   123,
						Utime:   234,
					},
					{
						Id:      5,
						Uid:     uid + 2,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "oss问题2",
						Content: "oss问题2",
						Ctime:   1234,
						Utime:   2345,
					},
				}
				for _, q := range questions {
					require.NoError(t, s.db.WithContext(ctx).Create(&q).Error)
				}

				// 题集中题目为0
				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, 0, len(qs))
			},
			after: func(t *testing.T) {
				t.Helper()

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				expected := []dao.Question{
					{
						Uid:     uid + 1,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "oss问题1",
						Content: "oss问题1",
					},
					{
						Uid:     uid + 2,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "oss问题2",
						Content: "oss问题2",
					},
				}

				actual, err := s.questionSetDAO.GetQuestionsByID(ctx, 5)
				require.NoError(t, err)
				require.Equal(t, len(expected), len(actual))

				for i := 0; i < len(expected); i++ {
					s.assertQuestion(t, expected[i], actual[i])
				}

			},
			req: web.UpdateQuestionsOfQuestionSetReq{
				QSID: 5,
				QIDs: []int64{4, 5},
			},
			wantCode: 200,
			wantResp: test.Result[int64]{},
		},
		{
			name: "非空题集_添加多个问题",
			before: func(t *testing.T) {
				t.Helper()
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				// 创建一个空题集
				id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
					Id:          7,
					Uid:         uid,
					Title:       "Go",
					Description: "Go题集",
				})
				require.NoError(t, err)
				require.Equal(t, int64(7), id)

				// 创建问题
				questions := []dao.Question{
					{
						Id:      14,
						Uid:     uid + 1,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题1",
						Content: "Go问题1",
						Ctime:   123,
						Utime:   234,
					},
					{
						Id:      15,
						Uid:     uid + 2,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题2",
						Content: "Go问题2",
						Ctime:   1234,
						Utime:   2345,
					},
					{
						Id:      16,
						Uid:     uid + 3,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题3",
						Content: "Go问题3",
						Ctime:   1234,
						Utime:   2345,
					},
				}
				for _, q := range questions {
					require.NoError(t, s.db.WithContext(ctx).Create(&q).Error)
				}

				require.NoError(t, s.questionSetDAO.UpdateQuestionsByID(ctx, id, []int64{14}))

				// 题集中题目为1
				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, 1, len(qs))
			},
			after: func(t *testing.T) {
				t.Helper()

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				expected := []dao.Question{
					{
						Uid:     uid + 1,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题1",
						Content: "Go问题1",
					},
					{
						Uid:     uid + 2,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题2",
						Content: "Go问题2",
					},
					{
						Uid:     uid + 3,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题3",
						Content: "Go问题3",
					},
				}

				actual, err := s.questionSetDAO.GetQuestionsByID(ctx, 7)
				require.NoError(t, err)
				require.Equal(t, len(expected), len(actual))

				for i := 0; i < len(expected); i++ {
					s.assertQuestion(t, expected[i], actual[i])
				}

			},
			req: web.UpdateQuestionsOfQuestionSetReq{
				QSID: 7,
				QIDs: []int64{14, 15, 16},
			},
			wantCode: 200,
			wantResp: test.Result[int64]{},
		},
		{
			name: "非空题集_删除全部问题",
			before: func(t *testing.T) {
				t.Helper()
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				// 创建一个空题集
				id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
					Id:          217,
					Uid:         uid,
					Title:       "Go",
					Description: "Go题集",
				})
				require.Equal(t, int64(217), id)
				require.NoError(t, err)

				// 创建问题
				questions := []dao.Question{
					{
						Id:      214,
						Uid:     uid + 1,
						Title:   "Go问题1",
						Content: "Go问题1",
						Ctime:   123,
						Utime:   234,
					},
					{
						Id:      215,
						Uid:     uid + 2,
						Title:   "Go问题2",
						Content: "Go问题2",
						Ctime:   1234,
						Utime:   2345,
					},
					{
						Id:      216,
						Uid:     uid + 2,
						Title:   "Go问题3",
						Content: "Go问题3",
						Ctime:   1234,
						Utime:   2345,
					},
				}
				for _, q := range questions {
					require.NoError(t, s.db.WithContext(ctx).Create(&q).Error)
				}

				require.NoError(t, s.questionSetDAO.UpdateQuestionsByID(ctx, id, []int64{214, 215, 216}))

				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, len(questions), len(qs))

			},
			after: func(t *testing.T) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, 217)
				require.NoError(t, err)
				require.Equal(t, 0, len(qs))
			},
			req: web.UpdateQuestionsOfQuestionSetReq{
				QSID: 217,
				QIDs: []int64{},
			},
			wantCode: 200,
			wantResp: test.Result[int64]{},
		},
		{
			name: "非空题集_删除部分问题",
			before: func(t *testing.T) {
				t.Helper()
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				// 创建一个空题集
				id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
					Id:          218,
					Uid:         uid,
					Title:       "Go",
					Description: "Go题集",
				})
				require.Equal(t, int64(218), id)
				require.NoError(t, err)

				// 创建问题
				questions := []dao.Question{
					{
						Id:      314,
						Uid:     uid + 1,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题1",
						Content: "Go问题1",
						Ctime:   123,
						Utime:   234,
					},
					{
						Id:      315,
						Uid:     uid + 2,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题2",
						Content: "Go问题2",
						Ctime:   1234,
						Utime:   2345,
					},
					{
						Id:      316,
						Uid:     uid + 2,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题3",
						Content: "Go问题3",
						Ctime:   1234,
						Utime:   2345,
					},
				}
				for _, q := range questions {
					require.NoError(t, s.db.WithContext(ctx).Create(&q).Error)
				}

				require.NoError(t, s.questionSetDAO.UpdateQuestionsByID(ctx, id, []int64{314, 315, 316}))

				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, len(questions), len(qs))
			},
			after: func(t *testing.T) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, 218)
				require.NoError(t, err)
				require.Equal(t, 1, len(qs))
				s.assertQuestion(t, dao.Question{
					Uid:     uid + 2,
					Status:  domain.UnPublishedStatus.ToUint8(),
					Title:   "Go问题2",
					Content: "Go问题2",
				}, qs[0])

			},
			req: web.UpdateQuestionsOfQuestionSetReq{
				QSID: 218,
				QIDs: []int64{315},
			},
			wantCode: 200,
			wantResp: test.Result[int64]{},
		},
		{
			name: "同时添加/删除部分问题",
			before: func(t *testing.T) {
				t.Helper()
				s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				// 创建一个空题集
				id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
					Id:          219,
					Uid:         uid,
					Title:       "Go",
					Description: "Go题集",
				})
				require.Equal(t, int64(219), id)
				require.NoError(t, err)

				// 创建问题
				questions := []dao.Question{
					{
						Id:      414,
						Uid:     uid + 1,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题1",
						Content: "Go问题1",
						Ctime:   123,
						Utime:   234,
					},
					{
						Id:      415,
						Uid:     uid + 2,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题2",
						Content: "Go问题2",
						Ctime:   1234,
						Utime:   2345,
					},
					{
						Id:      416,
						Uid:     uid + 3,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题3",
						Content: "Go问题3",
						Ctime:   1234,
						Utime:   2345,
					},
					{
						Id:      417,
						Uid:     uid + 4,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题4",
						Content: "Go问题4",
						Ctime:   1234,
						Utime:   2345,
					},
				}
				for _, q := range questions {
					require.NoError(t, s.db.WithContext(ctx).Create(&q).Error)
				}

				qids := []int64{414, 415}
				require.NoError(t, s.questionSetDAO.UpdateQuestionsByID(ctx, id, qids))

				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, len(qids), len(qs))
			},
			after: func(t *testing.T) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				expected := []dao.Question{
					{
						Uid:     uid + 2,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题2",
						Content: "Go问题2",
					},
					{
						Uid:     uid + 3,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题3",
						Content: "Go问题3",
					},
					{
						Uid:     uid + 4,
						Status:  domain.UnPublishedStatus.ToUint8(),
						Title:   "Go问题4",
						Content: "Go问题4",
					},
				}

				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, 219)
				require.NoError(t, err)

				require.Equal(t, len(expected), len(qs))

				for i, e := range expected {
					s.assertQuestion(t, e, qs[i])
				}
			},
			req: web.UpdateQuestionsOfQuestionSetReq{
				QSID: 219,
				QIDs: []int64{415, 416, 417},
			},
			wantCode: 200,
			wantResp: test.Result[int64]{},
		},
		{
			name: "题集不存在",
			before: func(t *testing.T) {
				t.Helper()
			},
			after: func(t *testing.T) {
				t.Helper()
			},
			req: web.UpdateQuestionsOfQuestionSetReq{
				QSID: 10000,
				QIDs: []int64{},
			},
			wantCode: 500,
			wantResp: test.Result[int64]{Code: 502001, Msg: "系统错误"},
		},
		// {
		//	name: "当前用户并非题集的创建者",
		//	before: func(t *testing.T) {
		//		t.Helper()
		//
		//		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		//		defer cancel()
		//
		//		// 创建一个空题集
		//		id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
		//			Id:          220,
		//			Uid:         uid + 100,
		//			Title:       "Go",
		//			Description: "Go题集",
		//		})
		//		require.Equal(t, int64(220), id)
		//		require.NoError(t, err)
		//	},
		//	after: func(t *testing.T) {
		//		t.Helper()
		//	},
		//	req: web.UpdateQuestionsOfQuestionSetReq{
		//		QSID: 220,
		//		QIDs: []int64{},
		//	},
		//	wantCode: 500,
		//	wantResp: test.Result[int64]{Code: 502001, Msg: "系统错误"},
		// },
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)
			req, err := http.NewRequest(http.MethodPost,
				"/question-sets/questions/save", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[int64]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
			tc.after(t)
		})
	}
}

func (s *HandlerTestSuite) TestQuestionSet_RetrieveQuestionSetDetail() {

	now := time.Now().UnixMilli()

	testCases := []struct {
		name   string
		before func(t *testing.T)
		after  func(t *testing.T)
		req    web.QuestionSetID

		wantCode int
		wantResp test.Result[web.QuestionSet]
	}{
		{
			name: "空题集",
			before: func(t *testing.T) {
				t.Helper()

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				// 创建一个空题集
				id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
					Id:          321,
					Uid:         uid,
					Title:       "Go",
					Description: "Go题集",
					Utime:       now,
				})
				require.NoError(t, err)
				require.Equal(t, int64(321), id)
			},
			after: func(t *testing.T) {
				t.Helper()
			},
			req: web.QuestionSetID{
				QSID: 321,
			},
			wantCode: 200,
			wantResp: test.Result[web.QuestionSet]{
				Data: web.QuestionSet{
					Id:          321,
					Title:       "Go",
					Description: "Go题集",
					Utime:       now,
					Interactive: web.Interactive{
						ViewCnt:    322,
						LikeCnt:    323,
						CollectCnt: 324,
						Liked:      true,
						Collected:  false,
					},
				},
			},
		},
		{
			name: "非空题集",
			before: func(t *testing.T) {
				t.Helper()

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()

				// 创建一个空题集
				id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
					Id:          322,
					Uid:         uid,
					Title:       "Go",
					Description: "Go题集",
					Utime:       now,
				})
				require.NoError(t, err)
				require.Equal(t, int64(322), id)

				// 添加问题
				questions := []dao.Question{
					{
						Id:      614,
						Uid:     uid + 1,
						Title:   "Go问题1",
						Content: "Go问题1",
						Ctime:   now,
						Utime:   now,
					},
					{
						Id:      615,
						Uid:     uid + 2,
						Title:   "Go问题2",
						Content: "Go问题2",
						Ctime:   now,
						Utime:   now,
					},
					{
						Id:      616,
						Uid:     uid + 3,
						Title:   "Go问题3",
						Content: "Go问题3",
						Ctime:   now,
						Utime:   now,
					},
				}
				for _, q := range questions {
					require.NoError(t, s.db.WithContext(ctx).Create(&q).Error)
				}

				qids := []int64{614, 615, 616}
				require.NoError(t, s.questionSetDAO.UpdateQuestionsByID(ctx, id, qids))

				// 题集中题目为1
				qs, err := s.questionSetDAO.GetQuestionsByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, len(qids), len(qs))
			},
			after: func(t *testing.T) {
				t.Helper()
			},
			req: web.QuestionSetID{
				QSID: 322,
			},
			wantCode: 200,
			wantResp: test.Result[web.QuestionSet]{
				Data: web.QuestionSet{
					Id:          322,
					Title:       "Go",
					Description: "Go题集",
					Interactive: web.Interactive{
						ViewCnt:    323,
						LikeCnt:    324,
						CollectCnt: 325,
						Liked:      false,
						Collected:  true,
					},
					Questions: []web.Question{
						{
							Id:      614,
							Title:   "Go问题1",
							Content: "Go问题1",
						},
						{
							Id:      615,
							Title:   "Go问题2",
							Content: "Go问题2",
						},
						{
							Id:      616,
							Title:   "Go问题3",
							Content: "Go问题3",
						},
					},
					Utime: now,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)
			req, err := http.NewRequest(http.MethodPost,
				"/question-sets/detail", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[web.QuestionSet]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
			tc.after(t)
		})
	}
}

func (s *HandlerTestSuite) TestQuestionSet_RetrieveQuestionSetDetail_Failed() {
	testCases := []struct {
		name   string
		before func(t *testing.T)
		after  func(t *testing.T)
		req    web.QuestionSetID

		wantCode int
		wantResp test.Result[int64]
	}{
		{
			name: "题集ID非法_题集ID不存在",
			before: func(t *testing.T) {
				t.Helper()
			},
			after: func(t *testing.T) {
				t.Helper()
			},
			req: web.QuestionSetID{
				QSID: 10000,
			},
			wantCode: 500,
			wantResp: test.Result[int64]{Code: 502001, Msg: "系统错误"},
		},
		// {
		//	name: "题集ID非法_题集ID与UID不匹配",
		//	before: func(t *testing.T) {
		//		t.Helper()
		//
		//		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		//		defer cancel()
		//
		//		// 创建一个空题集
		//		id, err := s.questionSetDAO.Create(ctx, dao.QuestionSet{
		//			Id:          320,
		//			Uid:         uid + 100,
		//			Title:       "Go",
		//			Description: "Go题集",
		//		})
		//		require.Equal(t, int64(320), id)
		//		require.NoError(t, err)
		//	},
		//	after: func(t *testing.T) {
		//		t.Helper()
		//	},
		//	req: web.QuestionSetID{
		//		QSID: 320,
		//	},
		//	wantCode: 500,
		//	wantResp: test.Result[int64]{Code: 502001, Msg: "系统错误"},
		// },
	}
	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			tc.before(t)
			req, err := http.NewRequest(http.MethodPost,
				"/question-sets/detail", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[int64]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
			tc.after(t)
		})
	}
}

func (s *HandlerTestSuite) TestQuestionSet_ListPrivateQuestionSets() {
	// 插入一百条
	total := 100
	data := make([]dao.QuestionSet, 0, total)

	for idx := 0; idx < total; idx++ {
		// 空题集
		data = append(data, dao.QuestionSet{
			Uid:         uid,
			Title:       fmt.Sprintf("题集标题 %d", idx),
			Description: fmt.Sprintf("题集简介 %d", idx),
			Utime:       123,
		})
	}
	err := s.db.Create(&data).Error
	require.NoError(s.T(), err)

	testCases := []struct {
		name string
		req  web.Page

		wantCode int
		wantResp test.Result[web.QuestionSetList]
	}{
		{
			name: "获取成功",
			req: web.Page{
				Limit:  2,
				Offset: 0,
			},
			wantCode: 200,
			wantResp: test.Result[web.QuestionSetList]{
				Data: web.QuestionSetList{
					Total: int64(total),
					QuestionSets: []web.QuestionSet{
						{
							Id:          100,
							Title:       "题集标题 99",
							Description: "题集简介 99",
							Utime:       123,
							Interactive: web.Interactive{
								ViewCnt:    101,
								LikeCnt:    102,
								CollectCnt: 103,
								Liked:      false,
								Collected:  true,
							},
						},
						{
							Id:          99,
							Title:       "题集标题 98",
							Description: "题集简介 98",
							Utime:       123,
							Interactive: web.Interactive{
								ViewCnt:    100,
								LikeCnt:    101,
								CollectCnt: 102,
								Liked:      true,
								Collected:  false,
							},
						},
					},
				},
			},
		},
		{
			name: "获取部分",
			req: web.Page{
				Limit:  2,
				Offset: 99,
			},
			wantCode: 200,
			wantResp: test.Result[web.QuestionSetList]{
				Data: web.QuestionSetList{
					Total: int64(total),
					QuestionSets: []web.QuestionSet{
						{
							Id:          1,
							Title:       "题集标题 0",
							Description: "题集简介 0",
							Utime:       123,
							Interactive: web.Interactive{
								ViewCnt:    2,
								LikeCnt:    3,
								CollectCnt: 4,
								Liked:      true,
								Collected:  false,
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost,
				"/question-sets/list", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[web.QuestionSetList]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
		})
	}
}

func (s *HandlerTestSuite) TestQuestionSet_ListAllQuestionSets() {
	// 插入一百条
	total := 100
	data := make([]dao.QuestionSet, 0, total)

	for idx := 0; idx < total; idx++ {
		// 空题集
		data = append(data, dao.QuestionSet{
			Uid:         int64(uid + idx),
			Title:       fmt.Sprintf("题集标题 %d", idx),
			Description: fmt.Sprintf("题集简介 %d", idx),
			Utime:       123,
		})
	}
	err := s.db.Create(&data).Error
	require.NoError(s.T(), err)

	testCases := []struct {
		name string
		req  web.Page

		wantCode int
		wantResp test.Result[web.QuestionSetList]
	}{
		{
			name: "获取成功",
			req: web.Page{
				Limit:  2,
				Offset: 0,
			},
			wantCode: 200,
			wantResp: test.Result[web.QuestionSetList]{
				Data: web.QuestionSetList{
					Total: int64(total),
					QuestionSets: []web.QuestionSet{
						{
							Id:          100,
							Title:       "题集标题 99",
							Description: "题集简介 99",
							Utime:       123,
							Interactive: web.Interactive{
								ViewCnt:    101,
								LikeCnt:    102,
								CollectCnt: 103,
								Liked:      false,
								Collected:  true,
							},
						},
						{
							Id:          99,
							Title:       "题集标题 98",
							Description: "题集简介 98",
							Utime:       123,
							Interactive: web.Interactive{
								ViewCnt:    100,
								LikeCnt:    101,
								CollectCnt: 102,
								Liked:      true,
								Collected:  false,
							},
						},
					},
				},
			},
		},
		{
			name: "获取部分",
			req: web.Page{
				Limit:  2,
				Offset: 99,
			},
			wantCode: 200,
			wantResp: test.Result[web.QuestionSetList]{
				Data: web.QuestionSetList{
					Total: int64(total),
					QuestionSets: []web.QuestionSet{
						{
							Id:          1,
							Title:       "题集标题 0",
							Description: "题集简介 0",
							Utime:       123,
							Interactive: web.Interactive{
								ViewCnt:    2,
								LikeCnt:    3,
								CollectCnt: 4,
								Liked:      true,
								Collected:  false,
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost,
				"/question-sets/list", iox.NewJSONReader(tc.req))
			req.Header.Set("content-type", "application/json")
			require.NoError(t, err)
			recorder := test.NewJSONResponseRecorder[web.QuestionSetList]()
			s.server.ServeHTTP(recorder, req)
			require.Equal(t, tc.wantCode, recorder.Code)
			assert.Equal(t, tc.wantResp, recorder.MustScan())
		})
	}
}

func (s *HandlerTestSuite) TestQuestionEvent() {
	t := s.T()
	ans := make([]event.Question, 0, 16)
	mu := sync.RWMutex{}
	s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, questionEvent event.QuestionEvent) error {
		var eve event.Question
		err := json.Unmarshal([]byte(questionEvent.Data), &eve)
		if err != nil {
			return err
		}
		mu.Lock()
		ans = append(ans, eve)
		mu.Unlock()
		return nil
	}).Times(2)
	// 保存
	saveReq := web.SaveReq{
		Question: web.Question{
			Title:        "面试题1",
			Content:      "新的内容",
			Analysis:     s.buildAnswerEle(1),
			Basic:        s.buildAnswerEle(2),
			Intermediate: s.buildAnswerEle(3),
			Advanced:     s.buildAnswerEle(4),
		},
	}
	req, err := http.NewRequest(http.MethodPost,
		"/question/save", iox.NewJSONReader(saveReq))
	req.Header.Set("content-type", "application/json")
	require.NoError(t, err)
	recorder := test.NewJSONResponseRecorder[int64]()
	s.server.ServeHTTP(recorder, req)

	require.Equal(t, 200, recorder.Code)

	// 发布
	syncReq := &web.SaveReq{
		Question: web.Question{
			Title:        "面试题2",
			Content:      "面试题内容",
			Analysis:     s.buildAnswerEle(0),
			Basic:        s.buildAnswerEle(1),
			Intermediate: s.buildAnswerEle(2),
			Advanced:     s.buildAnswerEle(3),
		},
	}
	req2, err := http.NewRequest(http.MethodPost,
		"/question/publish", iox.NewJSONReader(syncReq))
	req2.Header.Set("content-type", "application/json")
	require.NoError(t, err)
	recorder = test.NewJSONResponseRecorder[int64]()
	s.server.ServeHTTP(recorder, req2)
	require.Equal(t, 200, recorder.Code)
	time.Sleep(1 * time.Second)
	for idx := range ans {
		ans[idx].ID = 0
		ans[idx].Utime = 0
		ans[idx].Answer = event.Answer{
			Analysis:     s.removeId(ans[idx].Answer.Analysis),
			Basic:        s.removeId(ans[idx].Answer.Basic),
			Intermediate: s.removeId(ans[idx].Answer.Intermediate),
			Advanced:     s.removeId(ans[idx].Answer.Advanced),
		}
	}
	assert.Equal(t, []event.Question{
		{
			Title:   "面试题1",
			Content: "新的内容",
			UID:     uid,
			Status:  1,
			Answer: event.Answer{
				Analysis:     s.buildEventEle(1),
				Basic:        s.buildEventEle(2),
				Intermediate: s.buildEventEle(3),
				Advanced:     s.buildEventEle(4),
			},
		},
		{
			Title:   "面试题2",
			UID:     uid,
			Content: "面试题内容",
			Status:  2,
			Answer: event.Answer{
				Analysis:     s.buildEventEle(0),
				Basic:        s.buildEventEle(1),
				Intermediate: s.buildEventEle(2),
				Advanced:     s.buildEventEle(3),
			},
		},
	}, ans)
}

func (s *HandlerTestSuite) TestQuestionSetEvent() {
	t := s.T()
	ans := make([]event.QuestionSet, 0, 16)
	mu := &sync.RWMutex{}
	s.producer.EXPECT().Produce(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, questionEvent event.QuestionEvent) error {
		var eve event.QuestionSet
		err := json.Unmarshal([]byte(questionEvent.Data), &eve)
		if err != nil {
			return err
		}
		mu.Lock()
		ans = append(ans, eve)
		mu.Unlock()
		return nil
	}).Times(2)

	_, err := s.dao.Create(context.Background(), dao.Question{
		Id: 1,
	}, []dao.AnswerElement{
		{
			Content: "ele",
		},
	})
	require.NoError(t, err)
	_, err = s.dao.Create(context.Background(), dao.Question{
		Id: 2,
	}, []dao.AnswerElement{
		{
			Content: "ele",
		},
	})
	require.NoError(t, err)
	// 保存
	saveReq := web.SaveQuestionSetReq{
		Title:       "questionSet1",
		Description: "question_description1",
	}
	req, err := http.NewRequest(http.MethodPost,
		"/question-sets/save", iox.NewJSONReader(saveReq))
	req.Header.Set("content-type", "application/json")
	require.NoError(t, err)
	recorder := test.NewJSONResponseRecorder[int64]()
	s.server.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
	// 更新
	syncReq := &web.UpdateQuestionsOfQuestionSetReq{
		QSID: 1,
		QIDs: []int64{1, 2},
	}
	req2, err := http.NewRequest(http.MethodPost,
		"/question-sets/questions/save", iox.NewJSONReader(syncReq))
	req2.Header.Set("content-type", "application/json")
	require.NoError(t, err)
	recorder = test.NewJSONResponseRecorder[int64]()
	s.server.ServeHTTP(recorder, req2)
	require.Equal(t, 200, recorder.Code)
	time.Sleep(1 * time.Second)
	for idx := range ans {
		ans[idx].Id = 0
		ans[idx].Utime = 0
	}
	assert.Equal(t, []event.QuestionSet{
		{
			Uid:         uid,
			Title:       "questionSet1",
			Description: "question_description1",
			Questions:   []int64{},
		},
		{
			Uid:         uid,
			Title:       "questionSet1",
			Description: "question_description1",
			Questions:   []int64{1, 2},
		},
	}, ans)
}

func (s *HandlerTestSuite) removeId(ele event.AnswerElement) event.AnswerElement {
	require.True(s.T(), ele.ID != 0)
	ele.ID = 0
	return ele
}

func (s *HandlerTestSuite) buildEventEle(idx int64) event.AnswerElement {
	return event.AnswerElement{
		Content:   fmt.Sprintf("这是解析 %d", idx),
		Keywords:  fmt.Sprintf("关键字 %d", idx),
		Shorthand: fmt.Sprintf("快速记忆法 %d", idx),
		Highlight: fmt.Sprintf("亮点 %d", idx),
		Guidance:  fmt.Sprintf("引导点 %d", idx),
	}
}

func (s *HandlerTestSuite) mockInteractive(biz string, id int64) interactive.Interactive {
	liked := id%2 == 1
	collected := id%2 == 0
	return interactive.Interactive{
		Biz:        biz,
		BizId:      id,
		ViewCnt:    int(id + 1),
		LikeCnt:    int(id + 2),
		CollectCnt: int(id + 3),
		Liked:      liked,
		Collected:  collected,
	}
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}
