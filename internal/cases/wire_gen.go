// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package cases

import (
	"sync"

	"github.com/ecodeclub/mq-api"
	"github.com/ecodeclub/webook/internal/cases/internal/domain"
	"github.com/ecodeclub/webook/internal/cases/internal/event"
	"github.com/ecodeclub/webook/internal/cases/internal/repository"
	"github.com/ecodeclub/webook/internal/cases/internal/repository/dao"
	"github.com/ecodeclub/webook/internal/cases/internal/service"
	"github.com/ecodeclub/webook/internal/cases/internal/web"
	"github.com/ecodeclub/webook/internal/interactive"
	"github.com/ego-component/egorm"
	"gorm.io/gorm"
)

// Injectors from wire.go:

func InitModule(db *gorm.DB, intrModule *interactive.Module, q mq.MQ) (*Module, error) {
	caseDAO := InitCaseDAO(db)
	caseRepo := repository.NewCaseRepo(caseDAO)
	interactiveEventProducer, err := event.NewInteractiveEventProducer(q)
	if err != nil {
		return nil, err
	}
	syncEventProducer, err := event.NewSyncEventProducer(q)
	if err != nil {
		return nil, err
	}
	serviceService := service.NewService(caseRepo, interactiveEventProducer, syncEventProducer)
	service2 := intrModule.Svc
	handler := web.NewHandler(serviceService, service2)
	module := &Module{
		Svc: serviceService,
		Hdl: handler,
	}
	return module, nil
}

// wire.go:

var daoOnce = sync.Once{}

func InitTableOnce(db *gorm.DB) {
	daoOnce.Do(func() {
		err := dao.InitTables(db)
		if err != nil {
			panic(err)
		}
	})
}

func InitCaseDAO(db *egorm.Component) dao.CaseDAO {
	InitTableOnce(db)
	return dao.NewCaseDao(db)
}

type Handler = web.Handler

type Service = service.Service

type Case = domain.Case
