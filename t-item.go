package main

import (
	"fmt"
	"strings"
	"net/http"
	"io/ioutil"
	"github.com/alexmk92/stringutil"
	"regexp"
	"strconv"
	"database/sql"
)

/*
 |------------------------------------------------------------------
 | Type: Item
 |------------------------------------------------------------------
 |
 | Represents an item, when we fetch its data we first attempt to
 | hit our file cache, if the item doesn't exist there we fetch
 | it from the Wiki and then store it to our Mongo store
 |
 | @member name (string): Name of the item (url encoded)
 | @member displayName (string): Name of the item (browser friendly)
 | @member imageSrc (string): URL for the image stored on wiki
 | @member price (float32): The advertised price
 | @member statistics ([]Statistic): An array of all stats for this item
 |
 */

type Item struct {
	name string
	displayName string
	imageSrc string
	price float32
	statistics []Statistic
	effects []Effect
}

// Public method to fetch data for this item, in Go public method are
// capitalised by convention (doesn't actually enforce Public/Private methods in go)
// this method will call fetchDataFromWiki and fetchDataFromCache where appropriate
func (i *Item) FetchData() {
	fmt.Println("Fetching data for item: ", i.name)
	i.displayName = TitleCase(i.name, true)

	if(i.fetchDataFromSQL()) {
		fmt.Println("Exists in SQL")
	} else {
		i.fetchDataFromWiki()
		fmt.Println("Fetched from Wiki")
	}
}

// Data didn't exist on our server, so we hit the wiki here
func (i *Item) fetchDataFromWiki() {

	uriString := TitleCase(i.name, true)

	fmt.Println("Requesting data from: ", WIKI_BASE_URL + "/" + uriString)

	resp, err := http.Get(WIKI_BASE_URL + "/" + uriString)
	if(err != nil) {
		fmt.Println("ERROR GETTING DATA FROM WIKI: ", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if(err != nil) {
		fmt.Println("ERROR EXTRACTING BODY FROM RESPONSE: ", err)
	}
	i.extractItemDataFromHttpResponse(string(body))
}

// Check our cache first to see if the item exists - this will eventually return something
// other than a bool, it will return a parsed Item struct from a deserialised JSON object
// sent back from the mongo store
func (i *Item) fetchDataFromSQL() bool {
	var (
		name string
		displayName string
		statCode interface{}
		statValue interface{}
	)

	query := "SELECT name, displayName, code AS statCode, value AS statValue " +
		"FROM items " +
		"LEFT JOIN statistics " +
		"ON items.id = statistics.item_id " +
		"WHERE name = ? " +
		"OR displayName = ?"

	rows, _ := DB.Query(query, i.name, i.name)
	if rows != nil {
		defer rows.Close()

		hasStat := false
		for rows.Next() {
			err := rows.Scan(&name, &displayName, &statCode, &statValue)
			if err != nil {
				fmt.Println("Scan error: ", err)
			}
			if statCode == nil && statValue == nil {
				fmt.Println("No stat exists for: ", displayName)
			} else {
				hasStat = true
			}
			LogInDebugMode("Row is: ", name, displayName, fmt.Sprint(statCode), fmt.Sprint(statValue))
		}
		return hasStat
	}

	fmt.Println("No record found for item: ", i.name)
	return false
}

// Extracts data from body
func (i *Item) extractItemDataFromHttpResponse(body string) {
	itemDataIndex := stringutil.CaseInsensitiveIndexOf(body, "itemData")
	endOfItemDataIndex := stringutil.CaseInsensitiveIndexOf(body, "itembotbg")

	if(itemDataIndex > -1 && endOfItemDataIndex > -1) {

		body = body[itemDataIndex:endOfItemDataIndex]

		// Extract the item image - this assumes that the format is consistent (tested with 30 items thus far)
		imageSrc := body[stringutil.CaseInsensitiveIndexOf(body, "/images"):stringutil.CaseInsensitiveIndexOf(body, "width")-2]
		i.imageSrc = imageSrc

		// Extract the item information snippet
		openInfoParagraphIndex := stringutil.CaseInsensitiveIndexOf(body, "<p>") + 3 // +3 to ignore the <p> chars
		closeInfoParagraphIndex := stringutil.CaseInsensitiveIndexOf(body, "</p>")
		body = body[openInfoParagraphIndex:closeInfoParagraphIndex]

		upperParts := strings.Split(strings.TrimSpace(body), "<br />")

		for _, part := range upperParts {
			part = strings.TrimSpace(part)

			lowerParts := strings.Split(part, "  ")
			if(len(lowerParts) > 1) {
				for k :=0; k < len(lowerParts); k++ {
					i.assignStatistic(strings.TrimSpace(lowerParts[k]))
				}
			} else {
				i.assignStatistic(strings.TrimSpace(part))
			}
		}

		LogInDebugMode("Item is: ", i)
		go i.Save()
	} else {
		fmt.Println("No item data for this page")
	}
}

func (i *Item) assignStatistic(part string) {
	if strings.TrimSpace(part) == "" {
		return
	}

	var stat Statistic

	LogInDebugMode("Assigning part: ", part)
	if stringutil.CaseInsenstiveContains(part, "lore item", "magic item", "temporary") {
		stat.code = "AFFINITY"
		stat.effect = strings.ToUpper(part)
		stat.value = sql.NullFloat64{Float64: 0, Valid: false}
	} else if stringutil.CaseInsenstiveContains(part, "slot:", "class:", "race:", "size:", "skill:") {
		parts := strings.Split(part, ":")
		stat.code = strings.ToUpper(strings.TrimSpace(parts[0]))
		stat.effect = strings.ToUpper(strings.TrimSpace(parts[1]))
		stat.value = sql.NullFloat64{Float64: 0, Valid: false}
	} else if stringutil.CaseInsenstiveContains(part, "sv fire:", "sv cold:", "sv poison:", "sv magic:", "sv disease:", "dmg:", "ac:", "hp:", "dex:", "agi:", "sta:", "str:", "mana:", "cha:", "atk:", "wis:", "int:", "endr:", "wt:", "atk delay:") {
		parts := strings.Split(part, ":")

		isPositiveNumber := true
		if stringutil.CaseInsensitiveIndexOf(parts[1], "+") > -1 {
			parts[1] = strings.TrimSpace(strings.Replace(parts[1], "+", "", -1))
			isPositiveNumber = true
		} else if stringutil.CaseInsensitiveIndexOf(parts[1], "-") > -1 {
			parts[1] = strings.TrimSpace(strings.Replace(parts[1], "-", "", -1))
			isPositiveNumber = false
		}

		stat.code = strings.ToUpper(strings.TrimSpace(parts[0]))
		val, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

		if err != nil {
			fmt.Println("Stat error: ", err)
		} else {
			stat.value = sql.NullFloat64{Float64:val, Valid: true}
			if !isPositiveNumber {
				stat.value = sql.NullFloat64{Float64:(val * -1.0), Valid: true}
			}
		}
	} else if stringutil.CaseInsenstiveContains(part, "<a href=", "effect:") {
		var e Effect

		// Remove the effect: tag if it exists
		part = strings.TrimSpace(strings.Replace(part, "Effect:", "", -1))

		stat.code = "EFFECT"
		stat.effect = strings.ToUpper(part)
		stat.value = sql.NullFloat64{Float64: 0, Valid: false}

		parts := regexp.MustCompile("([A-Za-z0-9]+=\"(.*?)\")").FindAllString(part, -1)
		for _, part := range parts {
			part = strings.Replace(part, "\"", "", -1)
			if stringutil.CaseInsensitiveIndexOf(part, "href") > -1 {
				part = strings.Replace(part, "href=", "", -1)
				e.uri = part
			} else if stringutil.CaseInsensitiveIndexOf(part, "title") > -1 {
				part = strings.Replace(part, "title=", "", -1)
				e.name = part
			}
		}

		e.restriction = strings.TrimSpace(regexp.MustCompile("((<a)(.*?)(</a>))").ReplaceAllString(part, ""))

		i.effects = append(i.effects, e)
		return
	} else {
		fmt.Println("Unkown stat: ", part)
	}

	if stat.code != "" {
		i.statistics = append(i.statistics, stat)
	} else {
		LogInDebugMode("Nil stat code for: ", stat)
	}
}

// Saves the item to our SQL database
func (i *Item) Save() {
	query := "UPDATE items SET name = ?, displayName = ?, imageSrc = ? WHERE name = ? AND displayName = ?"
	_, err := DB.Query(query, i.name, i.displayName, i.imageSrc, i.name, i.displayName)
	if err != nil {
		fmt.Println(err.Error())
	}
	query = "SELECT id FROM items WHERE name = ? OR displayName = ?"
	rows, err := DB.Query(query, i.name, i.name)
	if err == nil {
		if rows != nil {
			var id int64

			defer rows.Close()
			for rows.Next() {
				err := rows.Scan(&id)
				if err != nil {
					fmt.Println("Scan error: ", err)
				}
				LogInDebugMode("Got id: ", fmt.Sprint(id))
				i.saveStats(id)
				i.saveEffects(id)
			}
		}
	} else {
		fmt.Println("Failed to insert stats for this item: ", err.Error())
	}
}

func (i *Item) saveEffects(id int64) {
	fmt.Println("Saving effects for item: ", id)
	for _, effect := range i.effects {
		if effect.name != "" && effect.uri != "" {
			query := "SELECT id " +
				"FROM effects " +
				"WHERE name = ?"

			rows, _ := DB.Query(query, effect.name)
			if rows != nil {
				var effectId int64

				exists := false
				for rows.Next() {
					exists = true
					err := rows.Scan(&effectId)
					if err != nil {
						fmt.Println("Scan error: ", err)
					}
					LogInDebugMode("Got effect id: ", fmt.Sprint(effectId))
				}
				if !exists {
					query := "INSERT IGNORE INTO effects" +
						"(name, uri)" +
						"VALUES (?, ?)"

					newEffectId, err := DB.Insert(query, effect.name, effect.uri)
					if err != nil {
						fmt.Println(err.Error())
					} else if newEffectId > 0 {
						query := "INSERT INTO item_effects " +
							"(item_id, effect_id, restriction) " +
							"VALUES (?, ?, ?)"

						itemEffectId, err := DB.Insert(query, id, newEffectId, effect.restriction)
						if err != nil {
							fmt.Println(err.Error())
						} else if itemEffectId > 0 {
							fmt.Println("Saved effect: " + effect.name + " for item: " + i.name)
						}
					}
				} else {
					// Migrate to the Effect model so we dont repeat it
					query := "INSERT INTO item_effects " +
						"(item_id, effect_id, restriction) " +
						"VALUES (?, ?, ?)"

					itemEffectId, err := DB.Insert(query, id, effectId, effect.restriction)
					if err != nil {
						fmt.Println(err.Error())
					} else if itemEffectId > 0 {
						fmt.Println("Saved effect: " + effect.name + " for item: " + i.name)
					}
				}

				rows.Close()
			} else {
				fmt.Println("No rows for effect: ", effect.name)
			}
		} else {
			fmt.Println("Invalid effect")
		}
	}

}

func (i *Item) saveStats(id int64) {
	var parameters []interface{}
	query := "INSERT INTO statistics" +
		"(item_id, code, value, effect)" +
		"VALUES "

	for _, statistic := range i.statistics {
		query += "(?, ?, ?, ?),"
		parameters = append(parameters, id, statistic.code, statistic.value, statistic.effect)
	}
	query = query[0:len(query)-1]

	LogInDebugMode("Inserting new statistics with row id: ", int64(id))
	_, err := DB.Insert(query, parameters...)
	if err != nil {
		LogInDebugMode("Darn, we couldn't create this statistic")
	}
}