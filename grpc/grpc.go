/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/23
   Description :
-------------------------------------------------
*/

package grpc

import (
	"context"
	"net"
	"reflect"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/zly-app/zapp/core"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type GrpcServer = grpc.Server

var typeOfComponentConnInterface = reflect.TypeOf((*core.IComponent)(nil)).Elem()
var typeOfGrpcServer = reflect.TypeOf((*GrpcServer)(nil))

type RegistryGrpcServiceFunc = func(c core.IComponent, server *GrpcServer)

type GrpcService struct {
	app    core.IApp
	server *grpc.Server
	conf   *Config
}

func NewGrpcService(app core.IApp) core.IService {
	var conf Config
	err := app.GetConfig().ParseServiceConfig(nowServiceType, &conf)
	if err != nil {
		app.Fatal("创建服务失败", zap.String("serviceType", string(nowServiceType)), zap.Error(err))
	}

	if conf.HeartbeatTime <= 0 {
		conf.HeartbeatTime = defaultHeartbeatTime
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			UnaryServerLogInterceptor(app),         // 日志
			grpc_ctxtags.UnaryServerInterceptor(),  // 设置标记
			grpc_recovery.UnaryServerInterceptor(), // panic恢复
		)),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time: time.Duration(conf.HeartbeatTime) * time.Millisecond, // 心跳
		}),
	)

	return &GrpcService{
		app:    app,
		server: server,
		conf:   &conf,
	}
}

func (g *GrpcService) Inject(a ...interface{}) {
	cValue := reflect.ValueOf(g.app.GetComponent())
	serverValue := reflect.ValueOf(g.server)
	for _, v := range a {
		vType := reflect.TypeOf(v)
		if vType.Kind() != reflect.Func {
			g.app.Fatal("grpc服务端注入参数必须是func")
			return
		}
		if vType.NumIn() != 2 {
			g.app.Fatal("grpc服务端注入func入参为2个")
			return
		}
		arg0 := vType.In(0)
		if !arg0.AssignableTo(typeOfComponentConnInterface) {
			g.app.Fatal("注入的func第1个入参必须能转为 core.IComponent")
			return
		}
		arg1 := vType.In(1)
		if !arg1.AssignableTo(typeOfGrpcServer) {
			g.app.Fatal("注入的func第2个入参必须能转为 *grpc.GrpcServer")
			return
		}

		vValue := reflect.ValueOf(v)
		vValue.Call([]reflect.Value{cValue, serverValue})
	}
}

func (g *GrpcService) Start() error {
	listener, err := net.Listen("tcp", g.conf.Bind)
	if err != nil {
		return err
	}

	g.app.Debug("正在启动grpc服务", zap.String("bind", g.conf.Bind))
	return g.server.Serve(listener)
}

func (g *GrpcService) Close() error {
	g.server.GracefulStop()
	g.app.Debug("grpc服务已关闭")
	return nil
}

// 日志拦截器
func UnaryServerLogInterceptor(app core.IApp) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		log := app.NewSessionLogger(zap.String("full_method", info.FullMethod))
		ctx = SaveLoggerToContext(ctx, log)

		startTime := time.Now()
		log.Debug("grpc.request", zap.Any("req", req))

		resp, err := handler(ctx, req)
		if err != nil {
			log.Error("grpc.response", zap.String("latency", time.Since(startTime).String()), zap.Error(err))
		} else {
			log.Debug("grpc.response", zap.String("latency", time.Since(startTime).String()), zap.Any("resp", resp))
		}

		return resp, err
	}
}
