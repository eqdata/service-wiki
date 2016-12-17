package main

// Just in case Controllers need to conform to some contract later lets generalise
// them as their own type, this enables us to build up a map of Controller's
type Controller interface {}

// Instantiate all controllers here so that we can bind them to our routes
var IC = new(ItemController)