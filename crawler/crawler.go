package crawler

import (
	"fmt"

	zapp_core "github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/logger"
	"go.uber.org/zap"

	"github.com/zly-app/service/crawler/config"
	"github.com/zly-app/service/crawler/core"
	"github.com/zly-app/service/crawler/downloader"
	"github.com/zly-app/service/crawler/middleware"
	"github.com/zly-app/service/crawler/queue"
)

type Crawler struct {
	app           zapp_core.IApp
	conf          *config.ServiceConfig
	parserMethods map[string]core.ParserMethod

	spider     core.ISpider
	queue      core.IQueue
	downloader core.IDownloader
	middleware core.IMiddleware
}

func (c *Crawler) Inject(a ...interface{}) {
	if c.spider != nil {
		c.app.Fatal("crawler服务重复注入")
	}

	if len(a) != 1 {
		c.app.Fatal("crawler服务注入数量必须为1个")
	}

	var ok bool
	c.spider, ok = a[0].(core.ISpider)
	if !ok {
		c.app.Fatal("crawler服务注入类型错误, 它必须能转为 crawler/core.ISpider")
	}

	c.CheckSpiderParserMethod()
}

func (c *Crawler) Start() error {
	err := c.spider.Init(c)
	if err != nil {
		return fmt.Errorf("spider初始化失败: %v", err)
	}

	go c.Run()
	return nil
}

func (c *Crawler) Close() error {
	err := c.spider.Stop()
	if err != nil {
		c.app.Error("spider停止时出错", zap.Error(err))
	}

	if err = c.downloader.Close(); err != nil {
		c.app.Error("关闭下载器时出错误", zap.Error(err))
	}
	if err = c.middleware.Close(); err != nil {
		c.app.Error("关闭中间件时出错误", zap.Error(err))
	}
	if err = c.queue.Close(); err != nil {
		c.app.Error("关闭队列时出错误", zap.Error(err))
	}
	return nil
}

func NewCrawler(app zapp_core.IApp) zapp_core.IService {
	conf := config.NewConfig(app)
	confKey := "services." + string(config.NowServiceType)
	if app.GetConfig().GetViper().IsSet(confKey) {
		err := app.GetConfig().ParseServiceConfig(config.NowServiceType, conf)
		if err != nil {
			logger.Log.Panic("服务配置错误", zap.String("serviceType", string(config.NowServiceType)), zap.Error(err))
		}
	}
	err := conf.Check()
	if err != nil {
		logger.Log.Panic("服务配置错误", zap.String("serviceType", string(config.NowServiceType)), zap.Error(err))
	}

	return &Crawler{
		app:        app,
		conf:       conf,
		queue:      queue.NewQueue(app),
		downloader: downloader.NewDownloader(app),
		middleware: middleware.NewMiddleware(app),
	}
}
