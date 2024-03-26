// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package order

import (
	"sync"
	"time"

	"github.com/ecodeclub/ecache"
	"github.com/ecodeclub/mq-api"
	"github.com/ecodeclub/webook/internal/credit"
	"github.com/ecodeclub/webook/internal/order/internal/consumer"
	"github.com/ecodeclub/webook/internal/order/internal/job"
	"github.com/ecodeclub/webook/internal/order/internal/repository"
	"github.com/ecodeclub/webook/internal/order/internal/repository/dao"
	service4 "github.com/ecodeclub/webook/internal/order/internal/service"
	"github.com/ecodeclub/webook/internal/order/internal/web"
	"github.com/ecodeclub/webook/internal/payment"
	"github.com/ecodeclub/webook/internal/pkg/sequencenumber"
	"github.com/ecodeclub/webook/internal/product"
	"github.com/ego-component/egorm"
	"github.com/google/wire"
	"gorm.io/gorm"
)

// Injectors from wire.go:

func InitHandler(db *gorm.DB, paymentSvc payment.Service, productSvc product.Service, creditSvc credit.Service, cache ecache.Cache) *web.Handler {
	serviceService := initService(db)
	generator := sequencenumber.NewGenerator()
	handler := web.NewHandler(serviceService, paymentSvc, productSvc, creditSvc, generator, cache)
	return handler
}

func InitCompleteOrderConsumer(db *gorm.DB, q mq.MQ) *consumer.CompleteOrderConsumer {
	serviceService := initService(db)
	v := InitMQConsumer(q)
	completeOrderConsumer := consumer.NewCompleteOrderConsumer(serviceService, v)
	return completeOrderConsumer
}

// wire.go:

type Handler = web.Handler

type CompleteOrderConsumer = consumer.CompleteOrderConsumer

type CloseExpiredOrdersJob = job.CloseExpiredOrdersJob

var HandlerSet = wire.NewSet(
	initService, sequencenumber.NewGenerator, web.NewHandler,
)

var (
	once = &sync.Once{}
	svc  service4.Service
)

func initService(db *gorm.DB) service4.Service {
	once.Do(func() {
		_ = dao.InitTables(db)
		orderDAO := dao.NewOrderGORMDAO(db)
		orderRepository := repository.NewRepository(orderDAO)
		svc = service4.NewService(orderRepository)
	})
	return svc
}

func InitMQConsumer(q mq.MQ) []mq.Consumer {
	topic := "payment_successful"
	groupID := "OrderConsumerGroup"
	c, err := q.Consumer(topic, groupID)
	if err != nil {
		panic(err)
	}
	return []mq.Consumer{c}
}

func InitCloseExpiredOrdersJob(db *egorm.Component) *CloseExpiredOrdersJob {
	return job.NewCloseExpiredOrdersJob(initService(db), 10, 31, time.Hour)
}