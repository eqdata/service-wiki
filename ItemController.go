package main

import (
	"net/http"
	"fmt"
	"encoding/json"
	"strings"
	"strconv"
)

type ItemController struct {
	Controller
}

// Stores auction data to the Amazon RDS storage once it has been parsed
func (c *ItemController) store(w http.ResponseWriter, r *http.Request) {
	var items []string
	if r.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return
	}
	err := json.NewDecoder(r.Body).Decode(&items)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	fmt.Println("items are: ", items)
	if len(items) == 0 {
		http.Error(w, "No lines were present in the auctions array", 400)
		return
	}

	go c.parse(&items)
}

//
func (c *ItemController) parse(rawItems *[]string) {

	for _, itemName := range *rawItems {
		// Ensure string is properly formatted
		itemName = strings.TrimSpace(itemName)
		LogInDebugMode("Item is: " + itemName + ", length is: " + strconv.Itoa(len(itemName)))
		item := Item {
			name: itemName,
			displayName: TitleCase(itemName, true),
		}
		go item.FetchData()
	}
}
