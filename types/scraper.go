package types

import "math"

type Scraper interface {
	Navigate()
	GetProductList()
	FormatProductList()
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
	s.FormatProductList()
	s.GetProductList()
}
