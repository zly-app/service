package cron

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/zly-app/zapp"
)

func TestTask(t *testing.T) {
	task := NewTask("test", "@every 1s", true, func(ctx IContext) (err error) {
		fmt.Println("触发")
		return nil
	})
	err := task.Trigger(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestCronTask(t *testing.T) {
	app := zapp.NewApp("test",
		WithService(),
	)
	defer app.Exit()

	RegistryHandler("test", "@every 1s", true, func(ctx IContext) (err error) {
		fmt.Println("触发")
		return nil
	})

	go func() {
		time.Sleep(time.Second * 3)
		app.Exit()
	}()
	app.Run()
}
