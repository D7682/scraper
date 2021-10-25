package types

import (
	"bufio"
	"context"
	"encoding/json"
	_ "encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
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
	Selector
}

type Selector struct {
	AddCartBtn string
	CartIcoSel   string
	PriceSel  string
	CheckoutBtn string
}

func (b *BestBuy) Purchase() chromedp.ActionFunc {
	// Add a checker that removes any items that are not the item that we chose.
	// and change the item option to 1 so only one item is bought.
	return func(ctx context.Context) error {
		fmt.Printf("%v\n", b.CheckoutBtn)
		err := chromedp.WaitReady(b.CheckoutBtn, chromedp.ByQueryAll).Do(ctx)
		if err != nil {
			return err
		}

		err = chromedp.ScrollIntoView(b.CheckoutBtn, chromedp.ByQueryAll).Do(ctx)
		if err != nil {
			return err
		}

		err = chromedp.Evaluate(fmt.Sprintf(`
		let checkoutbtn = document.querySelector(%v)
		checkoutbtn[0].click()
		`, b.CheckoutBtn), nil, chromedp.EvalWithCommandLineAPI).Do(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}


func (b *BestBuy) GoToCart() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		err := chromedp.Navigate("https://www.bestbuy.com/cart").Do(ctx)
		if err != nil {
			return err
		}
		return nil
	}
}

func (b *BestBuy) AddToCart() chromedp.ActionFunc {
	return func(ctx context.Context) error {
	outerloop:
		for {
			var (
				nodes      []*cdp.Node
				buttonNode *cdp.Node
			)

			err := chromedp.WaitReady("body").Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.WaitReady(b.AddCartBtn).Do(ctx)
			if err != nil {
				return err
			}


			// This first section checks that the button is ready to be checked
			// For each loop we first check that the button is always ready.
			attr := func() string {
				err = chromedp.Nodes(b.AddCartBtn, &nodes, chromedp.ByQuery).Do(ctx)
				if err != nil {
					log.Fatal(err)
				}
				buttonNode = nodes[0]

				err = chromedp.ScrollIntoView(b.AddCartBtn, chromedp.ByQueryAll).Do(ctx)
				if err != nil {
					log.Fatal(err)
				}
				// c-button-disabled is a class property that we are checking for in the button
				// if c-button-disabled does exist as a property of the button we refresh until the product is available.
				attr, _ := buttonNode.Attribute("class")
				return attr
			}()

			// If the button has the class property of c-button-disabled then we want to refresh the page endlessly.
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

			// This loop checks that items were added to the cart.
		clickLoop:
			err = func() error {
				err := chromedp.WaitVisible("body", chromedp.ByQueryAll).Do(ctx)
				if err != nil {
					return err
				}

				err = chromedp.WaitVisible(b.AddCartBtn, chromedp.ByQueryAll).Do(ctx)
				if err != nil {
					return err
				}

				err = chromedp.ScrollIntoView(b.AddCartBtn, chromedp.ByQueryAll).Do(ctx)
				if err != nil {
					return err
				}

				// if things don't get added to the cart for some reason
				// uncomment the line below. I'm not sure yet, but the bestbuy site may require the mouse to be on the page
				// the line below takes care of making sure that this is simulated.
				// input.DispatchMouseEvent(input.MouseMoved, 0, 0)
				fmt.Println(buttonNode)
				err = chromedp.MouseClickNode(buttonNode, chromedp.ButtonLeft).Do(ctx)
				if err != nil {
					return err
				}

				err = chromedp.WaitVisible(b.CartIcoSel, chromedp.ByQueryAll).Do(ctx)
				if err != nil {
					return err
				}

				var items *runtime.RemoteObject
				fmt.Printf("%#v\n", items)
				err = chromedp.Evaluate(`
					if (typeof(newCart) === "undefined") {
						let newCart = document.querySelectorAll("div[data-testid=cart-count]")[0]
						newCart.innerText
					}`, &items, chromedp.EvalWithCommandLineAPI).Do(ctx)
				if err != nil {
					return err
				}
				type Data struct {
					Value int `json:",string"`
				}

				data, err := items.MarshalJSON()
				if err != nil {
					log.Fatal(err)
				}

				var itemsInCart Data
				err = json.Unmarshal(data, &itemsInCart)
				if err != nil {
					log.Fatal(err)
				}

				fmt.Println(itemsInCart.Value)

				return nil
			}()
			if err != nil {
				err := chromedp.Reload().Do(ctx)
				if err != nil {
					return err
				}
				goto clickLoop
			}
			break outerloop
		}
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

func (b *BestBuy) CreatePrompt() chromedp.ActionFunc {
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

		err = chromedp.Nodes(b.PriceSel, &pricing, chromedp.ByQueryAll).Do(ctx)
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
			}
		}
		b.Prompt.Label = "Pick an Item"
		b.Prompt.Items = listing

		// This part writes the prompt to the terminal.
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

func (b *BestBuy) SetSel() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		b.Selector = Selector{
			AddCartBtn: `div[class="fulfillment-add-to-cart-button"] > div:nth-child(1) > div[style="position:relative"] > .add-to-cart-button.c-button-lg`,
			CartIcoSel:   `div[data-testid="cart-count"]`,
			PriceSel: `div[class="pricing-price"] > div > div > div > div > div > div > div > span[aria-hidden="true"]:nth-child(1)`,
			CheckoutBtn: "`button[class=\"btn btn-lg btn-block btn-primary\"]`",
		}
		return nil
	}
}

func (b *BestBuy) Init() chromedp.ActionFunc {
	var (
		scheduler *gocron.Scheduler
		message   = func() {
			_, err := Notification.Println("You can cancel at any time with ctrl-c.")
			if err != nil {
				log.Fatal(err)
			}
		}
	)
	// Scheduled Notification
	scheduler = gocron.NewScheduler(time.UTC)
	_, err := scheduler.Every(10).Seconds().Do(message)
	if err != nil {
		log.Fatal(err)
	}
	scheduler.StartAsync()

	return func(ctx context.Context) error {
		// Set Selectors
		err := b.SetSel().Do(ctx)
		if err != nil {
			return err
		}

		err = b.Navigate().Do(ctx)
		if err != nil {
			return err
		}

		err = b.CreatePrompt().Do(ctx)
		if err != nil {
			return err
		}

		err = b.AddToCart().Do(ctx)
		if err != nil {
			return err
		}

		err = b.GoToCart().Do(ctx)
		if err != nil {
			return err
		}

		err = b.Purchase().Do(ctx)
		if err != nil {
			return err
		}

		err = chromedp.Sleep(time.Second * 100).Do(ctx)
		if err != nil {
			return err
		}

		scheduler.Stop()
		return nil
	}
}
