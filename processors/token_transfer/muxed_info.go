package token_transfer

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func NewMuxedInfoFromMemo(m xdr.Memo) *MuxedInfo {
	protoMemo := &MuxedInfo{}

	switch m.Type {
	case xdr.MemoTypeMemoNone:
		return nil
	case xdr.MemoTypeMemoId:
		id := uint64(*m.Id)
		return NewMuxedInfoFromId(id)
	case xdr.MemoTypeMemoText:
		protoMemo.Content = &MuxedInfo_Text{
			Text: *m.Text,
		}
	case xdr.MemoTypeMemoHash:
		hashSlice := make([]byte, 32)
		copy(hashSlice, (*m.Hash)[:])
		protoMemo.Content = &MuxedInfo_Hash{
			Hash: hashSlice,
		}
	case xdr.MemoTypeMemoReturn:
		hashSlice := make([]byte, 32)
		copy(hashSlice, (*m.RetHash)[:])
		protoMemo.Content = &MuxedInfo_Hash{
			Hash: hashSlice,
		}
	default:
		panic(fmt.Errorf("unknown memo type: %v", m.Type))
	}

	return protoMemo
}

func NewMuxedInfoFromId(id uint64) *MuxedInfo {
	return &MuxedInfo{
		Content: &MuxedInfo_Id{
			Id: id,
		},
	}
}

func NewMuxedInfoFromText(text string) *MuxedInfo {
	return &MuxedInfo{
		Content: &MuxedInfo_Text{
			Text: text,
		},
	}
}
