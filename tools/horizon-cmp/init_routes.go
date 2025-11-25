package main

import (
	cmp "github.com/stellar/go-stellar-sdk/tools/horizon-cmp/internal"
)

var routes = cmp.Routes{
	List: []*cmp.Route{
		// The order of the list is important because some regexps can match
		// a wider route.
		cmp.MakeRoute(`/accounts/*/effects`),
		cmp.MakeRoute(`/accounts/*/payments`),
		cmp.MakeRoute(`/accounts/*/operations`),
		cmp.MakeRoute(`/accounts/*/trades`),
		cmp.MakeRoute(`/accounts/*/transactions`),
		cmp.MakeRoute(`/accounts/*/offers`),
		cmp.MakeRoute(`/accounts/*/data/*`),
		cmp.MakeRoute(`/accounts/*`),
		cmp.MakeRoute(`/accounts`),

		cmp.MakeRoute(`/ledgers/*/transactions`),
		cmp.MakeRoute(`/ledgers/*/operations`),
		cmp.MakeRoute(`/ledgers/*/payments`),
		cmp.MakeRoute(`/ledgers/*/effects`),
		cmp.MakeRoute(`/ledgers/*`),
		cmp.MakeRoute(`/ledgers`),

		cmp.MakeRoute(`/operations/*/effects`),
		cmp.MakeRoute(`/operations/*`),
		cmp.MakeRoute(`/operations`),

		cmp.MakeRoute(`/transactions/*/effects`),
		cmp.MakeRoute(`/transactions/*/operations`),
		cmp.MakeRoute(`/transactions/*/payments`),
		cmp.MakeRoute(`/transactions/*`),
		cmp.MakeRoute(`/transactions`),

		cmp.MakeRoute(`/offers/*/trades`),
		cmp.MakeRoute(`/offers/*`),
		cmp.MakeRoute(`/offers`),

		cmp.MakeRoute(`/payments`),
		cmp.MakeRoute(`/effects`),
		cmp.MakeRoute(`/trades`),
		cmp.MakeRoute(`/trade_aggregations`),
		cmp.MakeRoute(`/order_book`),
		cmp.MakeRoute(`/assets`),
		cmp.MakeRoute(`/fee_stats`),

		cmp.MakeRoute(`/paths/strict-receive`),
		cmp.MakeRoute(`/paths/strict-send`),
		cmp.MakeRoute(`/paths`),

		cmp.MakeRoute(`/`),
	},
}
