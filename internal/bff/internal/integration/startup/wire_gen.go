// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package bff

import (
	"github.com/ecodeclub/webook/internal/bff/internal/web"
	"github.com/ecodeclub/webook/internal/cases"
	"github.com/ecodeclub/webook/internal/interactive"
	baguwen "github.com/ecodeclub/webook/internal/question"
)

// Injectors from wire.go:

func InitHandler(intrModule *interactive.Module, caseModule *cases.Module, queSvc *baguwen.Module) (*web.Handler, error) {
	service := intrModule.Svc
	serviceService := caseModule.Svc
	caseSetService := caseModule.SetSvc
	service2 := queSvc.Svc
	questionSetService := queSvc.SetSvc
	examineService := queSvc.ExamSvc
	handler := web.NewHandler(service, serviceService, caseSetService, service2, questionSetService, examineService)
	return handler, nil
}

// wire.go:

type Hdl web.Handler
