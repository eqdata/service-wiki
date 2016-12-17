package main

import "database/sql"

/*
 |-------------------------------------------------------------------------
 | Type: Statistic
 |--------------------------------------------------------------------------
 |
 | Represents a statistic
 |
 | @member seller (string): The name of the stat (HP, Mana etc.)
 | @member value (int32): The value of the stat, this isn't a uint as we can
 | have negative stats, for example a fungi has -10 AGI or an AoN has -100HP
 | @member effect (string): some items may have non int based values, in which
 | case they have an effect property, this is nullable
 |
 */

type Statistic struct {
	code string
	value sql.NullFloat64
	effect string
}
