package main

import (
	"strings"
	"fmt"
)

// MIGRATE THIS TO stringutil eventually
// Given a string, generate a snake case string, if we have URLFriendly enabled then we add _ to make a URI string
func TitleCase(name string, urlFriendly bool) string {

	uriParts := strings.Split(name, " ")
	LogInDebugMode("STRING PARTS ARE: ", uriParts)

	uriString := ""
	for _, part := range uriParts {
		compare := strings.ToLower(part)
		if(compare == "the" || compare == "of" || compare == "or" || compare == "and" || compare == "a" || compare == "an" || compare == "on" || compare == "to") {
			part = strings.ToLower(part)
		} else {
			part = strings.Title(part)
		}
		if urlFriendly {
			uriString += part + "_"
		} else {
			uriString += part + " "
		}
	}
	uriString = strings.Replace(uriString, "'S", "'s", -1)
	uriString = uriString[0:len(uriString)-1]
	return uriString
}

// Replaces fmt.Println and is used for logging debug messages
func LogInDebugMode(message string, args ...interface{}) {
	if DEBUG {
		fmt.Println(message, args)
	}
}
