#!/bin/sh
{
	grep -ohE 'Code: *"[a-z_]+"' internal/server/*.go | grep -oE '"[a-z_]+"' | tr -d '"'
	grep -oE ', "[a-z_]+"\}' internal/server/json.go | grep -oE '"[a-z_]+"' | tr -d '"'
	grep -ohE 'refuse(Holding)?\("[a-z_]+"' internal/themehost/*.go internal/mediahost/*.go |
		grep -oE '"[a-z_]+"' | tr -d '"'
	grep -ohE 'Refuse\([A-Za-z.]+, "[a-z_]+"' internal/content/*.go |
		grep -oE ', "[a-z_]+"' | grep -oE '"[a-z_]+"' | tr -d '"'
} | sort -u
