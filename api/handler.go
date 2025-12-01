package api

import (
	"fmt"
	"reflect"

	"github.com/zly-app/zapp/log"
	"go.uber.org/zap"

	"github.com/zly-app/service/api/utils"
)

type handlerUtil struct {
	handler interface{}
	name    string
	hType   reflect.Type
}

func newHandler(handler interface{}) *handlerUtil {
	if handler == nil {
		log.Fatal("handler为nil", zap.String("handler", fmt.Sprintf("%T", handler)))
	}

	hType := reflect.TypeOf(handler)
	if hType.Kind() != reflect.Func {
		log.Fatal("handler必须是函数", zap.String("handler", fmt.Sprintf("%T", handler)))
	}

	hName := utils.GetFuncName(handler)

	return &handlerUtil{
		handler: handler,
		name:    hName,
		hType:   hType,
	}
}

// 检查 handler 指纹
func (h *handlerUtil) checkHandlerFingerprint() {
	// 检查入参
	if h.hType.NumIn() < 1 || h.hType.NumIn() > 2 {
		log.Fatal("handler的入参数量为1个或2个", zap.String("handlerName", h.name), zap.String("fingerprint", fmt.Sprintf("%T", h.handler)))
	}

	// 检查第一个参数
	arg0 := h.hType.In(0)
	if !arg0.AssignableTo(typeOfContext) {
		log.Fatal("handler的第一个入参必须是 *api.Context", zap.String("handlerName", h.name), zap.String("fingerprint", fmt.Sprintf("%T", h.handler)))
	}

	// 检查出参
	if h.hType.NumOut() < 1 || h.hType.NumOut() > 2 {
		log.Fatal("handler的出参数量为1个或2个", zap.String("handlerName", h.name), zap.String("fingerprint", fmt.Sprintf("%T", h.handler)))
	}

	// 如果出参数为2个, 最后一个出参必须是error
	if h.hType.NumOut() == 2 {
		out1 := h.hType.Out(1)
		if !out1.AssignableTo(typeOfError) {
			log.Fatal("handler的第二个出参必须是 error", zap.String("handlerName", h.name), zap.String("fingerprint", fmt.Sprintf("%T", h.handler)))
		}
	}
}

// 根据 handler 构建req建造者
//
// req 是 handler 的第二个入参, 如果入参数量小于 2 返回 nil
// req 必须是 struct 或 *struct
func (h *handlerUtil) mustMakeReqCreator() func(ctx *Context) (reflect.Value, error) {
	if h.hType.NumIn() < 2 {
		return nil
	}

	// 检查 req 是 struct 或 *struct
	arg1 := h.hType.In(1)                  // 获取req的类型
	reqIsPtr := arg1.Kind() == reflect.Ptr // req参数是否为指针
	if reqIsPtr {
		arg1 = arg1.Elem() // 获取req的真实类型
	}
	if arg1.Kind() != reflect.Struct {
		log.Fatal("handler的第二个入参必须是 struct 或 *struct", zap.String("fingerprint", fmt.Sprintf("%T", h.handler)))
	}

	// 返回建造者
	return func(ctx *Context) (reflect.Value, error) {
		req := reflect.New(arg1)                          // 创建req实例
		if err := ctx.Bind(req.Interface()); err != nil { // bind参数
			return req, err
		}

		if reqIsPtr { // 如果req是指针, 直接返回
			return req, nil
		}
		return req.Elem(), nil // 非指针要返回指向对象
	}
}

// 构建handler
func (h *handlerUtil) makeHandler() Handler {
	hValue := reflect.ValueOf(h.handler)
	reqCreator := h.mustMakeReqCreator()
	return func(ctx *Context) interface{} {
		var outValues []reflect.Value

		// 调用handler
		if reqCreator == nil { // 如果没有req建造者, 表示不需要req参数
			outValues = hValue.Call([]reflect.Value{reflect.ValueOf(ctx)})
		} else {
			reqValue, err := reqCreator(ctx)
			if err != nil {
				return err
			}
			outValues = hValue.Call([]reflect.Value{reflect.ValueOf(ctx), reqValue})
		}

		// 检查结果
		if len(outValues) == 1 { // 如果只有一个结果直接返回
			return outValues[0].Interface()
		}

		err := outValues[1].Interface()
		if err != nil {
			return err.(error)
		}
		return outValues[0].Interface()
	}
}

func (h *handlerUtil) MakeHandler() Handler {
	fn, ok := h.handler.(Handler)
	if !ok {
		h.checkHandlerFingerprint() // 检查handler指纹
		fn = h.makeHandler()        // 构建handler
	}
	return func(ctx *Context) interface{} {
		ctx.Values().Set("_handler_name", h.name)
		return fn(ctx)
	}
}
