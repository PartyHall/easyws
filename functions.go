package easyws

import "net/http"

type IsConnectionAllowedChecker func(socketType string, r *http.Request) bool
