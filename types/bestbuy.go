package types

import (
	"bufio"
	"context"
	_ "encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/fatih/color"
	"github.com/go-co-op/gocron"
	"github.com/manifoldco/promptui"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	Notification = color.New(color.FgHiYellow)
)

type BestBuy struct {
	Table  List
	Mutex  *sync.RWMutex
	Prompt *promptui.Select
}

func (b *BestBuy) Purchase() chromedp.ActionFunc {
	message := func() {
		_, err := Notification.Println("You can cancel at any time with ctrl-c.")
		if err != nil {
			log.Fatal(err)
		}
	}

	return func(ctx context.Context) error {

		var (
			scheduler *gocron.Scheduler
		)
		// Scheduled Notification
		scheduler = gocron.NewScheduler(time.UTC)
		_, err := scheduler.Every(10).Seconds().Do(message)
		if err != nil {
			log.Fatal(err)
		}
		scheduler.StartAsync()

		// The gocron operation is blocking wg.Done() from being able to end the program.
		outerloop:
		for {
			// This first section checks that the button is ready to be checked
			// For each loop we first check that the button is always ready.
			var nodes []*cdp.Node

			err := chromedp.WaitReady(`div[class="fulfillment-add-to-cart-button"] > div:nth-child(1) > div[style="position:relative"] > .add-to-cart-button.c-button-lg`, chromedp.ByQueryAll).Do(ctx)
			if err != nil {
				log.Fatal(err)
			}

			err = chromedp.Nodes(`div[class="fulfillment-add-to-cart-button"] > div:nth-child(1) > div[style="position:relative"] > .add-to-cart-button.c-button-lg`, &nodes, chromedp.ByQuery).Do(ctx)
			if err != nil {
				log.Fatal(err)
			}


			// I did not execute anything on its own thread here, because we have to wait for the buttons to load as well as refresh at the correct time.
			for _, val := range nodes {
				// c-button-disabled is a class property that we are checking for in the button
				// if c-button-disabled does exist as a property of the button we refresh until the product is available.
				attr, _ := val.Attribute("class")
				if strings.Contains(attr, "c-button-disabled") {
					start := time.Now()

					err := chromedp.Reload().Do(ctx)
					if err != nil {
						log.Fatal(err)
					}
					elapsed := time.Since(start)
					fmt.Printf("Refreshed Item. Time Elapsed: %v\n", elapsed)
					continue
				}

				err := chromedp.ScrollIntoView(`div[class="fulfillment-add-to-cart-button"] > div:nth-child(1) > div[style="position:relative"] > .add-to-cart-button.c-button-lg`, chromedp.ByQueryAll).Do(ctx)
				if err != nil {
					log.Fatal(err)
				}

				// This Loop takes care of timing all the click events ( bestbuy website has many nuances to it ).
				clickloop:
					for {
						err = chromedp.WaitReady("body", chromedp.ByQueryAll).Do(ctx)
						if err != nil {
							fmt.Println(err)
						}

						// if things don't get added to the cart for some reason
						// uncomment the line below. I'm not sure yet, but the bestbuy site may require the mouse to be on the page
						// the line below takes care of making sure that this is simulated.
						// input.DispatchMouseEvent(input.MouseMoved, 0, 0)
						err = chromedp.MouseClickNode(val, chromedp.ButtonLeft).Do(ctx)
						if err != nil {
							return err
						}

						// Add a cart checker to check if the value of the cart is 1 after "click"
						// if there are 0 items in the cart make sure to reload the page and rerun this loop.
											/*err = chromedp.Reload().Do(ctx)
											if err != nil {
												fmt.Println(err)
												return err
											}*/


						break clickloop
					}
				break outerloop
			}
		}
		scheduler.Stop()
		chromedp.Sleep(time.Second * 60)
		return nil
	}
}

// Navigate will navigate to bestbuy and the link that we store.
func (b *BestBuy) Navigate() chromedp.ActionFunc {
	return func(ctx context.Context) error {
			// Search Prompt.
			fmt.Println("What do you want to search?")
			scanner := bufio.NewReader(os.Stdin)
			inputString, err := scanner.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			inputString = strings.TrimSuffix(inputString, "\n")

			val := fmt.Sprintf("https://www.bestbuy.com/site/searchpage.jsp?st=%v", url.QueryEscape(inputString))
			err = chromedp.Navigate(val).Do(ctx)
			err = chromedp.WaitReady("body", chromedp.ByQueryAll).Do(ctx)
			err = chromedp.WaitReady("ol.sku-item-list", chromedp.ByQueryAll).Do(ctx)
			err = chromedp.WaitReady("a.icon-navigation-link", chromedp.ByQueryAll).Do(ctx)
			err = chromedp.ScrollIntoView("a.icon-navigation-link", chromedp.ByQueryAll).Do(ctx)
		return nil
	}
}

func (b *BestBuy) GetProductList() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		// This Part adds the elements to the Hash Table
		var (
			nodes   []*cdp.Node
			pricing []*cdp.Node
			listing []string
		)

		err := chromedp.Nodes(".sku-header > a", &nodes, chromedp.ByQueryAll).Do(ctx)
		if err != nil {
			return err
		}

		err = chromedp.Nodes(`div[class="pricing-price"] > div > div > div > div > div > div > div > span[aria-hidden="true"]:nth-child(1)`, &pricing, chromedp.ByQueryAll).Do(ctx)
		if err != nil {
			return err
		}

		ch := make(chan string)
		for i, list := range nodes {
			attr, ok := list.Attribute("href")
			go func(i int, list *cdp.Node) {
				if ok {
					str := list.Children[0].NodeValue
					b.Mutex.Lock()
					b.Table[fmt.Sprintf("%v - %v", str[:productNameLength(str)], pricing[i].Children[0].NodeValue)] = &Item{
						Name:  str,
						Price: pricing[i].Children[0].NodeValue,
						Link:  attr,
					}
					b.Mutex.Unlock()
					ch <- fmt.Sprintf("%v - %v", str[:productNameLength(str)], pricing[i].Children[0].NodeValue)
				}
			}(i, list)



			// this adds the name of each item in the table to the listing array
			// which we will then assign as the list of prompt items.
			select {
			case listingName := <-ch:
				listing = append(listing, listingName)
				fmt.Println("appended")
			}
		}
		b.Prompt.Label = "Pick an Item"
		fmt.Println(listing)
		b.Prompt.Items = listing
		return nil
	}
}

func (b *BestBuy) PromptSelection() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		_, choice, err := b.Prompt.Run()
		if err != nil {
			log.Fatal(err)
		}

		val := fmt.Sprintf("https://www.bestbuy.com%v", b.Table[choice].Link)
		err = chromedp.Navigate(val).Do(ctx)
		if err != nil {
			return err
		}
		return nil
	}
}
