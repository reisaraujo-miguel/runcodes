package services

import "time"

// nowFunc is the clock used for visibility checks; a variable so tests can
// control it.
var nowFunc = time.Now
