package types

import (
	"github.com/chromedp/chromedp"
	"math"
)

type Scraper interface {
	Navigate() chromedp.ActionFunc
	GetProductList() chromedp.ActionFunc
	PromptSelection() chromedp.ActionFunc
	Purchase() chromedp.ActionFunc
}

type Item struct {
	Name  string
	Price string
	Link  string
}

type List map[string]*Item

func productNameLength(x string) int {
	return int(math.Round(float64(len(x)) - float64(len(x)/2)))
}

func Init(s Scraper) {
	s.Navigate()
	s.GetProductList()
	s.PromptSelection()
	s.Purchase()
}
