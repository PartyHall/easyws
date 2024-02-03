package easyws

import "github.com/labstack/echo/v4"

type IsConnectionAllowedChecker func(socketType string, c *echo.Context) bool
type OnJoinEvent func(socketType string, s *Socket)
