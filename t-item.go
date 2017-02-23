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
	id int64
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
func (i *Item) FetchData(syncSave bool) {
	fmt.Println("Fetching data for item: ", i.name)
	i.displayName = TitleCase(i.name, true)

	if(i.fetchDataFromSQL()) {
		fmt.Println("Exists in SQL")
	} else {
		i.fetchDataFromWiki(syncSave)
		fmt.Println("Fetched from Wiki")
	}
}

// Data didn't exist on our server, so we hit the wiki here
func (i *Item) fetchDataFromWiki(syncSave bool) {

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

	if !stringutil.CaseInsenstiveContains(i.name, "spell:") && !stringutil.CaseInsenstiveContains(i.displayName, "spell:") {
		i.extractItemDataFromHttpResponse(string(body), syncSave)
	}
}

// Check our cache first to see if the item exists - this will eventually return something
// other than a bool, it will return a parsed Item struct from a deserialised JSON object
// sent back from the mongo store
func (i *Item) fetchDataFromSQL() bool {
	var (
		id int64
		name string
		displayName string
		statCode interface{}
		statValue interface{}
	)

	query := "SELECT items.id, name, displayName, code AS statCode, value AS statValue " +
		"FROM items " +
		"LEFT JOIN statistics " +
		"ON items.id = statistics.item_id " +
		"WHERE name = ? " +
		"OR displayName = ?"

	rows, _ := DB.Query(query, i.name, i.name)
	if rows != nil {
		hasStat := false
		for rows.Next() {
			err := rows.Scan(&id, &name, &displayName, &statCode, &statValue)
			if err != nil {
				fmt.Println("Scan error: ", err)
			}
			if statCode == nil && statValue == nil {
				fmt.Println("No stat exists for: ", displayName)
			} else {
				hasStat = true
			}
			if id > 0 {
				i.id = id
			}
			//fmt.Println("Row is: ", fmt.Sprint(id), name, displayName, fmt.Sprint(statCode), fmt.Sprint(statValue))
		}
		if err := rows.Err(); err != nil {
			fmt.Println("ROW ERROR: ", err.Error())
		}
		DB.CloseRows(rows)
		return hasStat
	} else {
		fmt.Println("No record found for item: ", i.name)
		return false
	}
}

// Extracts data from body
func (i *Item) extractItemDataFromHttpResponse(body string, syncSave bool) {
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

			fmt.Println("Matching against: ", part)
			reg := regexp.MustCompile(`([A-Za-z:]+[ ]{0,}([+|-]?[ ]{0,}[0-9.]+|[A-Z ]+))`)
			matches := reg.FindAllStringSubmatch(part, -1)
			if len(matches) > 0 && !stringutil.CaseInsenstiveContains(part, "effect:") {
				for _, match := range matches {
					i.assignStatistic(strings.TrimSpace(match[0]))
				}
			} else {
				// Race, class etc can be auto handled
				i.assignStatistic(strings.TrimSpace(part))
			}
		}

		i.Save()
	} else {
		// Check if its a spell page
		reg := regexp.MustCompile("(?i)(magician|necromancer|paladin|warrior|druid|enchanter|cleric|shadowknight|monk|shaman|wizard|bard|rogue|ranger)")
		classMatches := reg.FindStringSubmatch(body)
		reg = regexp.MustCompile("(?i)(level[ \n]+[0-9]+)") // account for any poor formatting
		levelMatches := reg.FindStringSubmatch(body)

		if len(classMatches) > 0 && len(levelMatches) > 0 {
			srcMatches := regexp.MustCompile("(?i)(/images/)((.*?)+ ?\")").FindStringSubmatch(body)
			if len(srcMatches) > 0 {
				i.imageSrc = strings.TrimSpace(srcMatches[0])
			}

			class := classMatches[0]
			level := strings.TrimSpace(regexp.MustCompile("[0-9]+").FindStringSubmatch(levelMatches[0])[0])

			var stats []Statistic
			var stat Statistic
			stat.code = "CLASS"
			stat.effect = class

			stats = append(stats, stat)

			stat.code = "LEVEL"
			lvl, err := strconv.ParseFloat(level, 64)

			if err == nil {
				stat.value = sql.NullFloat64{Float64: float64(lvl), Valid: true}
				stat.effect = ""

				stats = append(stats, stat)

				i.name = "Spell: " + i.name
				i.statistics = stats

				fmt.Println("Saving item: ", i)

				query := "SELECT id FROM items WHERE displayName = ? OR name = ?"
				rows, _ := DB.Query(query, i.name, i.name)
				if rows != nil {
					var spellId int64

					exists := false
					for rows.Next() {
						exists = true
						err := rows.Scan(&spellId)
						if err != nil {
							fmt.Println("Scan error: ", err)
						}
						LogInDebugMode("Got effect id: ", fmt.Sprint(spellId))
					}
					if err := rows.Err(); err != nil {
						fmt.Println("ROW ERROR: ", err.Error())
					}
					DB.CloseRows(rows)
					if exists && spellId > 0 {
						i.saveStats(spellId)
					}
				}
			} else {
				fmt.Println("Conversion error: ", err.Error())
			}
		}
	}
}

func (i *Item) assignStatistic(part string) {
	if strings.TrimSpace(part) == "" {
		return
	}

	var stat Statistic

	LogInDebugMode("Assigning part: ", part)
	if stringutil.CaseInsenstiveContains(part, "size capacity:") {
		parts := strings.Split(part, ":")
		stat.code = "size capacity"
		stat.effect = parts[1]
	} else if stringutil.CaseInsenstiveContains(part, "nodrop", "quest item", "lore item", "magic item", "temporary", "no drop", "no rent", "no trade", "norent", "notrade", "expendable") {
		stat.code = "AFFINITY"
		stat.effect = strings.ToUpper(part)
		stat.value = sql.NullFloat64{Float64: 0, Valid: false}
	} else if stringutil.CaseInsenstiveContains(part, "slot:", "class:", "race:", "size:", "skill:") {
		parts := strings.Split(part, ":")
		stat.code = strings.ToUpper(strings.TrimSpace(parts[0]))
		stat.effect = strings.ToUpper(strings.TrimSpace(parts[1]))
		stat.value = sql.NullFloat64{Float64: 0, Valid: false}
	} else if stringutil.CaseInsenstiveContains(part, "sv fire:", "sv cold:", "sv poison:", "sv magic:", "sv disease:", "dmg:", "ac:", "hp:", "dex:", "agi:", "sta:", "str:", "mana:", "cha:", "atk:", "wis:", "int:", "endr:", "wt:", "atk delay:", "haste:", "instrument:", "instruments:", "range:", "charges:", "weight reduction:", "capacity:") {
		parts := strings.Split(part, ":")

		isPositiveNumber := true
		if stringutil.CaseInsensitiveIndexOf(parts[1], "+") > -1 {
			parts[1] = strings.TrimSpace(strings.Replace(parts[1], "+", "", -1))
			isPositiveNumber = true
		} else if stringutil.CaseInsensitiveIndexOf(parts[1], "-") > -1 {
			parts[1] = strings.TrimSpace(strings.Replace(parts[1], "-", "", -1))
			isPositiveNumber = false
		} else if stringutil.CaseInsensitiveIndexOf(parts[1], "%") > -1 {
			parts[1] = strings.TrimSpace(strings.Replace(parts[1], "%", "", -1))
		}

		stat.code = strings.ToUpper(strings.TrimSpace(parts[0]))
		val, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

		if err != nil {
			fmt.Println("Stat error for item " + i.name + ": ", err)
		} else {
			stat.value = sql.NullFloat64{Float64:val, Valid: true}
			if !isPositiveNumber {
				stat.value = sql.NullFloat64{Float64:(val * -1.0), Valid: true}
			}
		}
	} else if stringutil.CaseInsenstiveContains(part, "<a href=", "effect:", "casting time:", "combat", "at level") {
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

func (i *Item) Save() {
	query := "UPDATE items SET imageSrc = ? WHERE name = ? OR displayName = ?"
	_, err := DB.Query(query, i.imageSrc, i.name, i.name)
	if err == nil {
		i.saveEffects(i.id)
		i.saveStats(i.id)
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
				if err := rows.Err(); err != nil {
					fmt.Println("ROW ERROR: ", err.Error())
				}
				DB.CloseRows(rows)
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

				DB.CloseRows(rows)
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