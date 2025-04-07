package database

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

var UserCache = expirable.NewLRU[string, User](512, nil, time.Hour*24)
