// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package user

import (
	"github.com/ecodeclub/ecache"
	"github.com/ecodeclub/mq-api"
	"github.com/ecodeclub/webook/internal/member"
	"github.com/ecodeclub/webook/internal/permission"
	"github.com/ecodeclub/webook/internal/user/internal/repository"
	"github.com/ecodeclub/webook/internal/user/internal/repository/cache"
	"github.com/ecodeclub/webook/internal/user/internal/service"
	"github.com/ecodeclub/webook/internal/user/internal/web"
	"github.com/google/wire"
	"gorm.io/gorm"
)

// Injectors from wire.go:

func InitHandler(db *gorm.DB, cache2 ecache.Cache, q mq.MQ, creators []string, memberSvc *member.Module, permissionSvc *permission.Module) *web.Handler {
	wechatWebOAuth2Service := initWechatWebOAuthService(cache2)
	wechatMiniOAuth2Service := initWechatMiniOAuthService()
	userDAO := initDAO(db)
	userCache := cache.NewUserECache(cache2)
	userRepository := repository.NewCachedUserRepository(userDAO, userCache)
	registrationEventProducer := initRegistrationEventProducer(q)
	userService := service.NewUserService(userRepository, registrationEventProducer)
	serviceService := memberSvc.Svc
	service2 := permissionSvc.Svc
	handler := iniHandler(wechatWebOAuth2Service, wechatMiniOAuth2Service, userService, serviceService, service2, creators)
	return handler
}

// wire.go:

var ProviderSet = wire.NewSet(
	iniHandler, cache.NewUserECache, initDAO,
	initWechatWebOAuthService,
	initWechatMiniOAuthService,
	initRegistrationEventProducer, service.NewUserService, repository.NewCachedUserRepository,
)
