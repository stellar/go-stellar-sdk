package xdr

import "slices"

// ToAccountId returns the account ID of the muxed account, the view
// counterpart of MuxedAccount.ToAccountId. Unlike most view accessors, the
// result holds exactly the AccountId's XDR, since an AccountId is a fixed 36
// bytes. For KEY_TYPE_ED25519 it shares v's bytes, and for
// KEY_TYPE_MUXED_ED25519 it is built in a new buffer.
func (v MuxedAccountView) ToAccountId() (AccountIdView, error) {
	return Try(func() AccountIdView {
		if v.MustType() == CryptoKeyTypeKeyTypeMuxedEd25519 {
			// PUBLIC_KEY_TYPE_ED25519 is 0.
			return slices.Concat([]byte{0, 0, 0, 0}, v.MustMed25519().MustEd25519().MustRaw())
		}
		// MustEd25519 rejects any other key type. KEY_TYPE_ED25519 and
		// PUBLIC_KEY_TYPE_ED25519 are both 0, so the account's XDR is the
		// AccountId's.
		_ = v.MustEd25519()
		id := AccountIdView(v).MustRaw()
		return id[:len(id):len(id)]
	})
}

// MustToAccountId is ToAccountId for use inside Try.
func (v MuxedAccountView) MustToAccountId() AccountIdView { return must(v.ToAccountId()) }
