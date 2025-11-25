package xdr

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

func (a ClaimAtom) OfferId() Int64 {
	switch a.Type {
	case ClaimAtomTypeClaimAtomTypeV0:
		return a.V0.OfferId
	case ClaimAtomTypeClaimAtomTypeOrderBook:
		return a.OrderBook.OfferId
	case ClaimAtomTypeClaimAtomTypeLiquidityPool:
		panic(errors.New("liquidity pools don't have offers"))
	default:
		panic(fmt.Errorf("Unknown ClaimAtom type: %v", a.Type))
	}
}

func (a ClaimAtom) SellerId() AccountId {
	switch a.Type {
	case ClaimAtomTypeClaimAtomTypeV0:
		return AccountId{
			Type:    PublicKeyTypePublicKeyTypeEd25519,
			Ed25519: &a.V0.SellerEd25519,
		}
	case ClaimAtomTypeClaimAtomTypeOrderBook:
		return a.OrderBook.SellerId
	case ClaimAtomTypeClaimAtomTypeLiquidityPool:
		panic(errors.New("liquidity pools don't have a seller"))
	default:
		panic(fmt.Errorf("Unknown ClaimAtom type: %v", a.Type))
	}
}

func (a ClaimAtom) AssetBought() Asset {
	switch a.Type {
	case ClaimAtomTypeClaimAtomTypeV0:
		return a.V0.AssetBought
	case ClaimAtomTypeClaimAtomTypeOrderBook:
		return a.OrderBook.AssetBought
	case ClaimAtomTypeClaimAtomTypeLiquidityPool:
		return a.LiquidityPool.AssetBought
	default:
		panic(fmt.Errorf("Unknown ClaimAtom type: %v", a.Type))
	}
}

func (a ClaimAtom) AmountBought() Int64 {
	switch a.Type {
	case ClaimAtomTypeClaimAtomTypeV0:
		return a.V0.AmountBought
	case ClaimAtomTypeClaimAtomTypeOrderBook:
		return a.OrderBook.AmountBought
	case ClaimAtomTypeClaimAtomTypeLiquidityPool:
		return a.LiquidityPool.AmountBought
	default:
		panic(fmt.Errorf("Unknown ClaimAtom type: %v", a.Type))
	}
}

func (a ClaimAtom) AssetSold() Asset {
	switch a.Type {
	case ClaimAtomTypeClaimAtomTypeV0:
		return a.V0.AssetSold
	case ClaimAtomTypeClaimAtomTypeOrderBook:
		return a.OrderBook.AssetSold
	case ClaimAtomTypeClaimAtomTypeLiquidityPool:
		return a.LiquidityPool.AssetSold
	default:
		panic(fmt.Errorf("Unknown ClaimAtom type: %v", a.Type))
	}
}

func (a ClaimAtom) AmountSold() Int64 {
	switch a.Type {
	case ClaimAtomTypeClaimAtomTypeV0:
		return a.V0.AmountSold
	case ClaimAtomTypeClaimAtomTypeOrderBook:
		return a.OrderBook.AmountSold
	case ClaimAtomTypeClaimAtomTypeLiquidityPool:
		return a.LiquidityPool.AmountSold
	default:
		panic(fmt.Errorf("Unknown ClaimAtom type: %v", a.Type))
	}
}
