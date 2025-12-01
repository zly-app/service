/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2020/11/30
   Description :
-------------------------------------------------
*/

package api

import (
	"fmt"
	"reflect"

	"github.com/zly-app/zapp/log"
	"go.uber.org/zap"

	"github.com/kataras/iris/v12"
	iris_context "github.com/kataras/iris/v12/context"

	app_config "github.com/zly-app/zapp/config"

	"github.com/zly-app/service/api/utils"
)

// 处理程序
//
// 如果返回bytes会直接返回给客户端
// 返回其它值会经过处理后再返回给客户端
type Handler = func(ctx *Context) interface{}

// 写入响应函数
type WriteResponseFunc func(ctx *Context, code int, message string, data interface{})

// 设置写入响应函数
func SetWriteResponseFunc(fn WriteResponseFunc) {
	if fn == nil {
		panic("WriteResponseFunc is nil")
	}
	defaultWriteResponseFunc = fn
}

// 默认写入响应函数
var defaultWriteResponseFunc WriteResponseFunc = func(ctx *Context, code int, message string, data interface{}) {
	switch v := data.(type) {
	case []byte: // 直接写入
		_, _ = ctx.Write(v)
	default:
		_ = ctx.JSON(Response{
			ErrCode: code,
			ErrMsg:  message,
			Data:    data,
		})
	}
}

type Response struct {
	ErrCode int         `json:"err_code"`
	ErrMsg  string      `json:"err_msg"`
	Data    interface{} `json:"data,omitempty"`
}

var typeOfContext = reflect.TypeOf((*Context)(nil))
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

// 包装处理程序
func wrap(handler interface{}, isMiddleware bool) iris.Handler {
	if handler == nil {
		log.Fatal("handler为nil", zap.String("handler", fmt.Sprintf("%T", handler)))
	}

	h := newHandler(handler)
	fn := h.MakeHandler()

	return func(irisCtx *iris_context.Context) {
		ctx := makeContext(irisCtx) // 构建上下文
		result := fn(ctx)           // 处理

		// 如果是中间件, 只有返回nil才能继续调用链, 非nil值表示拦截, 并将结果处理后返回给客户端
		if isMiddleware && result == nil { // 返回nil继续调用链
			ctx.Next()
			return
		}

		WriteToCtx(ctx, result) // 写入结果
		ctx.StopExecution()     // 停止调用链
	}
}

// 写入数据到ctx
//
// 如果返回bytes会直接返回给客户端
// 返回其它值会经过处理后再返回给客户端
func WriteToCtx(ctx *Context, result interface{}) {
	if err, ok := result.(error); ok {
		code, message := decodeErr(err)

		conf := utils.Context.MustGetConfFromIrisContext(ctx.IrisContext)
		if app_config.Conf.Config().Frame.Debug || conf.SendDetailedErrorInProduction {
			message = err.Error()
		}
		ctx.Values().Set("error", err)
		defaultWriteResponseFunc(ctx, code, message, nil)
		return
	}

	ctx.Values().Set("result", result)
	switch v := result.(type) {
	case []byte:
		ctx.ContentType(iris_context.ContentBinaryHeaderValue)
		defaultWriteResponseFunc(ctx, OK.Code, OK.Message, v)
	case *[]byte:
		ctx.ContentType(iris_context.ContentBinaryHeaderValue)
		defaultWriteResponseFunc(ctx, OK.Code, OK.Message, *v)
	default:
		ctx.ContentType(iris_context.ContentJSONHeaderValue)
		defaultWriteResponseFunc(ctx, OK.Code, OK.Message, result)
	}
}

// 包装处理程序
//
// handler 是一个 func
//
//	入参: 第一个入参必须是 *api.Context 类型, 如果有第二个入参必须是 struct, 第二个入参可以是指针, 第二个入参会自动bind
//	出参: 第一个出参可以是任何类型, 如果有第二个出参必须是error类型
//	示例:
//	    func (ctx *api.Context) interface{}
//	    func (ctx *api.Context) error
//	    func (ctx *api.Context, req *AnyReqStruct) interface{}
//	    func (ctx *api.Context, req *AnyReqStruct) error
//	    func (ctx *api.Context, req *AnyReqStruct) (interface{}, error)
//	    func (ctx *api.Context, req *AnyReqStruct) (*AnyOutStruct, error)
func Wrap(handler interface{}) iris.Handler {
	return wrap(handler, false)
}

// 包装中间件, 类似 Wrap, 只有返回nil才能继续调用链, 非nil值表示拦截, 并将结果处理后返回给客户端
func WrapMiddleware(handler interface{}) iris.Handler {
	return wrap(handler, true)
}
