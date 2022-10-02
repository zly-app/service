/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2020/11/29
   Description :
-------------------------------------------------
*/

package middleware

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/kataras/iris/v12"
	iris_context "github.com/kataras/iris/v12/context"
	open_log "github.com/opentracing/opentracing-go/log"
	app_config "github.com/zly-app/zapp/config"
	app_utils "github.com/zly-app/zapp/pkg/utils"
	"go.uber.org/zap"

	"github.com/zly-app/zapp/core"
	zapp_utils "github.com/zly-app/zapp/pkg/utils"

	"github.com/zly-app/service/api/config"
	"github.com/zly-app/service/api/utils"
)

func valuesToTexts(values map[string][]string, sep string) []string {
	var texts []string
	for k, vs := range values {
		for _, v := range vs {
			texts = append(texts, k+sep+v)
		}
	}
	sort.Strings(texts)
	return texts
}

func LoggerMiddleware(app core.IApp, conf *config.Config) iris.Handler {
	if app_config.Conf.Config().Frame.Log.Json {
		return loggerMiddlewareWithJson(app, conf)
	}
	return loggerMiddleware(app, conf)
}

// 以文本方式输出
func loggerMiddleware(app core.IApp, conf *config.Config) iris.Handler {
	isDebug := app_config.Conf.Config().Frame.Debug
	return func(irisCtx iris.Context) {
		startTime := time.Now()

		// log
		log := utils.Context.MustGetLoggerFromIrisContext(irisCtx)

		// 链路追踪
		ctx := utils.Context.MustGetContextFromIrisContext(irisCtx)
		span := zapp_utils.Trace.GetSpan(ctx)

		// request
		ip := utils.Context.GetRemoteIP(irisCtx)
		params := valuesToTexts(irisCtx.Request().URL.Query(), "=")
		span.SetTag("method", irisCtx.Method())
		span.SetTag("path", irisCtx.Path())
		span.LogFields(open_log.String("params", strings.Join(params, "\n")))
		span.LogFields(open_log.String("ip", irisCtx.RemoteAddr()))
		var msgBuff bytes.Buffer
		msgBuff.WriteString("api.request path: ")
		msgBuff.WriteString(irisCtx.Method())
		msgBuff.WriteByte(' ')
		msgBuff.WriteString(irisCtx.Path())
		msgBuff.WriteString("\nparams:\n")
		for _, s := range params {
			msgBuff.WriteString("  ")
			msgBuff.WriteString(s)
			msgBuff.WriteByte('\n')
		}
		msgBuff.WriteByte('\n')
		if conf.ReqLogLevelIsInfo {
			log.Info(msgBuff.String(), zap.String("ip", ip))
		} else {
			log.Debug(msgBuff.String(), zap.String("ip", ip))
		}

		// handler
		irisCtx.Next()

		// response
		msgBuff.Reset()
		msgBuff.WriteString("api.response path: ")
		msgBuff.WriteString(irisCtx.Method())
		msgBuff.WriteByte(' ')
		msgBuff.WriteString(irisCtx.Path())
		msgBuff.WriteString("\nparams:\n")
		for _, s := range params {
			msgBuff.WriteString("  ")
			msgBuff.WriteString(s)
			msgBuff.WriteByte('\n')
		}
		msgBuff.WriteByte('\n')

		latency := time.Since(startTime)
		fields := []interface{}{
			zap.String("ip", ip),
			zap.String("latency_text", latency.String()),
			zap.Duration("latency", latency),
		}

		// error
		err, hasErr := irisCtx.Values().Get("error").(error)
		hasPanic, _ := irisCtx.Values().Get("panic").(bool)
		if hasErr {
			if err == nil {
				err = fmt.Errorf("err{nil}")
			}
		}

		// headers
		if hasErr || conf.AlwaysLogHeaders {
			headers := valuesToTexts(irisCtx.Request().Header, ": ")
			span.LogFields(open_log.String("headers", strings.Join(headers, "\n")))
			msgBuff.WriteString("headers:\n")
			for _, s := range headers {
				msgBuff.WriteString("  ")
				msgBuff.WriteString(s)
				msgBuff.WriteByte('\n')
			}
			msgBuff.WriteByte('\n')
		}

		// body
		if hasErr || conf.AlwaysLogBody {
			body, _ := irisCtx.GetBody()
			bodyText := string(body)
			if irisCtx.GetContentTypeRequested() == iris_context.ContentBinaryHeaderValue { // 流
				bodyText = fmt.Sprintf("<bytesLen=%d>", irisCtx.GetContentLength())
			} else if len(bodyText) > int(conf.LogBodyMaxSize) { // 超长
				bodyText = fmt.Sprintf("<len=%d>{%s...}", irisCtx.GetContentLength(), bodyText[:conf.LogBodyMaxSize])
			}
			span.LogFields(open_log.String("body", bodyText))
			msgBuff.WriteString("body:")
			msgBuff.WriteString(bodyText)
			msgBuff.WriteString("\n\n")
		}

		// result
		if !hasErr {
			var result string
			contentType := iris_context.TrimHeaderValue(irisCtx.ResponseWriter().Header().Get(iris_context.ContentTypeHeaderKey))
			if contentType == iris_context.ContentBinaryHeaderValue { // 流
				result = fmt.Sprintf("result<bytesLen=%d>", irisCtx.ResponseWriter().Written())
			} else if irisCtx.ResponseWriter().Written() > conf.LogApiResultMaxSize { // 超长
				result = fmt.Sprintf("result<len=%d>", irisCtx.ResponseWriter().Written())
			} else {
				switch v := irisCtx.Values().Get("result").(type) {
				case nil:
					result = "result<nil>"
				case string:
					result = v
				default:
					result, _ = jsoniter.ConfigCompatibleWithStandardLibrary.MarshalToString(v)
				}
			}
			span.LogFields(open_log.String("result", result))
			if (isDebug && conf.LogApiResultInDevelop) || (!isDebug && conf.LogApiResultInProd) {
				msgBuff.WriteString("result: ")
				msgBuff.WriteString(result)
				msgBuff.WriteString("\n\n")
			}
			if conf.RspLogLevelIsInfo {
				log.Info(append([]interface{}{msgBuff.String()}, fields...)...)
			} else {
				log.Debug(append([]interface{}{msgBuff.String()}, fields...)...)
			}
			return
		}

		// error
		if !hasPanic {
			span.SetTag("error", true)
			span.LogFields(open_log.Error(err))
			msgBuff.WriteString("err: ")
			msgBuff.WriteString(err.Error())
			msgBuff.WriteString("\n\n")
			log.Error(append([]interface{}{msgBuff.String()}, fields...)...)
			return
		}

		handlerName := irisCtx.Values().GetStringDefault("_handler_name", irisCtx.HandlerName())
		// panic
		panicErrDetail := app_utils.Recover.GetRecoverErrorDetail(err)
		panicErrInfos := strings.Split(panicErrDetail, "\n")
		span.SetTag("error", true)
		span.SetTag("panic", true)
		span.SetTag("handler_name", handlerName)
		span.LogFields(open_log.String("err", err.Error()))
		span.LogFields(open_log.String("detail", panicErrDetail))

		msgBuff.WriteString("panic:\n")
		msgBuff.WriteString("  Recovered from a route's Handler: ")
		msgBuff.WriteString(handlerName)
		msgBuff.WriteString("  ")
		msgBuff.WriteString(strings.Join(panicErrInfos, "\n  "))
		msgBuff.WriteString("\n\n")
		log.Error(append([]interface{}{msgBuff.String()}, fields...)...)

		// send_error_result
		result := map[string]interface{}{
			"err_code": 1,
			"err_msg":  "service internal error",
		}
		if isDebug || conf.SendDetailedErrorInProduction {
			result["err_msg"] = append(
				[]string{fmt.Sprintf("Recovered from a route's Handler: %s", handlerName)},
				panicErrInfos...,
			)
		}
		_, _ = irisCtx.JSON(result)
		irisCtx.StopExecution()
	}
}

// 以json方式输出
func loggerMiddlewareWithJson(app core.IApp, conf *config.Config) iris.Handler {
	isDebug := app_config.Conf.Config().Frame.Debug
	return func(irisCtx *iris_context.Context) {
		startTime := time.Now()

		// log
		log := utils.Context.MustGetLoggerFromIrisContext(irisCtx)

		// 链路追踪
		ctx := utils.Context.MustGetContextFromIrisContext(irisCtx)
		span := zapp_utils.Trace.GetSpan(ctx)

		// request
		ip := utils.Context.GetRemoteIP(irisCtx)
		params := valuesToTexts(irisCtx.Request().URL.Query(), "=")
		span.SetTag("method", irisCtx.Method())
		span.SetTag("path", irisCtx.Path())
		span.LogFields(open_log.String("params", strings.Join(params, "\n")))
		span.LogFields(open_log.String("ip", irisCtx.RemoteAddr()))

		fields := []interface{}{
			"api.request",
			zap.String("method", irisCtx.Method()),
			zap.String("path", irisCtx.Path()),
			zap.Strings("params", params),
			zap.String("ip", ip),
		}
		if conf.ReqLogLevelIsInfo {
			log.Info(fields...)
		} else {
			log.Debug(fields...)
		}

		// handler
		irisCtx.Next()

		// response
		latency := time.Since(startTime)
		fields = []interface{}{
			"api.response",
			zap.String("method", irisCtx.Method()),
			zap.String("path", irisCtx.Path()),
			zap.Strings("params", params),
			zap.String("ip", ip),
			zap.String("latency_text", latency.String()),
			zap.Duration("latency", latency),
		}

		// error
		err, hasErr := irisCtx.Values().Get("error").(error)
		hasPanic, _ := irisCtx.Values().Get("panic").(bool)
		if hasErr {
			if err == nil {
				err = fmt.Errorf("err{nil}")
			}
		}

		// headers
		if hasErr || conf.AlwaysLogHeaders {
			headers := valuesToTexts(irisCtx.Request().Header, ": ")
			span.LogFields(open_log.String("headers", strings.Join(headers, "\n")))
			fields = append(fields, zap.Strings("headers", headers))
		}

		// body
		if hasErr || conf.AlwaysLogBody {
			body, _ := irisCtx.GetBody()
			bodyText := string(body)
			if irisCtx.GetContentTypeRequested() == iris_context.ContentBinaryHeaderValue { // 流
				bodyText = fmt.Sprintf("<bytesLen=%d>", irisCtx.GetContentLength())
			} else if len(bodyText) > int(conf.LogBodyMaxSize) { // 超长
				bodyText = fmt.Sprintf("<len=%d>{%s...}", irisCtx.GetContentLength(), bodyText[:conf.LogBodyMaxSize])
			}
			span.LogFields(open_log.String("body", bodyText))
			fields = append(fields, zap.String("body", bodyText))
		}

		// result
		if !hasErr {
			var result string
			contentType := iris_context.TrimHeaderValue(irisCtx.ResponseWriter().Header().Get(iris_context.ContentTypeHeaderKey))
			if contentType == iris_context.ContentBinaryHeaderValue { // 流
				result = fmt.Sprintf("result<bytesLen=%d>", irisCtx.ResponseWriter().Written())
			} else if irisCtx.ResponseWriter().Written() > conf.LogApiResultMaxSize { // 超长
				result = fmt.Sprintf("result<len=%d>", irisCtx.ResponseWriter().Written())
			} else {
				switch v := irisCtx.Values().Get("result").(type) {
				case nil:
					result = "result<nil>"
				case string:
					result = v
				default:
					result, _ = jsoniter.ConfigCompatibleWithStandardLibrary.MarshalToString(v)
				}
			}
			span.LogFields(open_log.String("result", result))
			if (isDebug && conf.LogApiResultInDevelop) || (!isDebug && conf.LogApiResultInProd) {
				fields = append(fields, zap.String("result", result))
			}
			if conf.RspLogLevelIsInfo {
				log.Info(fields...)
			} else {
				log.Debug(fields...)
			}
			return
		}

		// error
		if !hasPanic {
			span.SetTag("error", true)
			span.LogFields(open_log.String("err", err.Error()))
			fields = append(fields, zap.String("err", err.Error()))
			log.Error(fields...)
			return
		}

		handlerName := irisCtx.Values().GetStringDefault("_handler_name", irisCtx.HandlerName())
		// panic
		panicErrDetail := app_utils.Recover.GetRecoverErrorDetail(err)
		panicErrInfos := strings.Split(panicErrDetail, "\n")
		span.SetTag("error", true)
		span.SetTag("panic", true)
		span.SetTag("handler_name", handlerName)
		span.LogFields(open_log.String("err", err.Error()))
		span.LogFields(open_log.String("detail", panicErrDetail))

		fields = append(fields,
			zap.Bool("panic", true),
			zap.String("handler_name", handlerName),
			zap.String("err", err.Error()),
			zap.Strings("detail", panicErrInfos),
		)
		log.Error(fields...)

		// send_error_result
		result := map[string]interface{}{
			"err_code": 1,
			"err_msg":  "service internal error",
		}
		if isDebug || conf.SendDetailedErrorInProduction {
			result["err_msg"] = append(
				[]string{fmt.Sprintf("Recovered from a route's Handler: %s", handlerName)},
				panicErrInfos...,
			)
		}
		_, _ = irisCtx.JSON(result)
		irisCtx.StopExecution()
	}
}
