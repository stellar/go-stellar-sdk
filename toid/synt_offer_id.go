package toid

type OfferIDType uint64

const (
	CoreOfferIDType OfferIDType = 0
	TOIDType        OfferIDType = 1

	mask uint64 = 0xC000000000000000
)

// Taken from https://github.com/stellar/go-stellar-sdk/tree/master/services/horizon/internal/db2/history

// EncodeOfferId creates synthetic offer ids to be used by trade resources
//
// This is required because stellar-core does not allocate offer ids for immediately filled offers,
// while clients expect them for aggregated views.
//
// The encoded value is of type int64 for sql compatibility. The 2nd bit is used to differentiate between stellar-core
// offer ids and operation ids, which are toids.
//
// Due to the 2nd bit being used, the largest possible toid is:
// 0011111111111111111111111111111100000000000000000001000000000001
// \          ledger              /\    transaction   /\    op    /
//
//	= 1073741823
//	  with avg. 5 sec close time will reach in ~170 years
func EncodeOfferId(id uint64, typ OfferIDType) int64 {
	// First ensure the bits we're going to change are 0s
	if id&mask != 0 {
		panic("Value too big to encode")
	}
	return int64(id | uint64(typ)<<62)
}

// DecodeOfferID performs the reverse operation of EncodeOfferID
func DecodeOfferID(encodedId int64) (uint64, OfferIDType) {
	if encodedId < 0 {
		panic("Negative offer ids can not be decoded")
	}
	return uint64(encodedId<<2) >> 2, OfferIDType(encodedId >> 62)
}
