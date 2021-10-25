package main

import (
	"context"
	"github.com/D7682/scraper/types"
	"github.com/chromedp/chromedp"
	"github.com/manifoldco/promptui"
	"log"
	"sync"
	"time"
)

func Tasks() chromedp.Tasks {
	bestbuy := types.BestBuy {
		Table: make(types.List),
		Mutex: new(sync.RWMutex),
		Prompt: new(promptui.Select),

	}

	return chromedp.Tasks{
		bestbuy.Init(),
		chromedp.Sleep(time.Second * 5),
	}
}

func main() {
	AllocCtx, AllocCancel := chromedp.NewExecAllocator(context.Background(), []chromedp.ExecAllocatorOption{
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("blink-settings", "imagesEnabled=true"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 6.3; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/73.0.3683.103 Safari/537.36"),
	}...)
	taskCtx, taskCancel := chromedp.NewContext(AllocCtx)

	if err := chromedp.Run(taskCtx, Tasks()...); err != nil {
		log.Fatal(err)
	}

	defer func() {
		AllocCancel()
		taskCancel()
	}()
}
