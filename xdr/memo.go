package xdr

func MemoText(text string) Memo {
	m, _ := NewMemo(MemoTypeMemoText, text)
	return m
}

func MemoID(id uint64) Memo {
	m, _ := NewMemo(MemoTypeMemoId, Uint64(id))
	return m
}

func MemoHash(hash Hash) Memo {
	m, _ := NewMemo(MemoTypeMemoHash, hash)
	return m
}

func MemoRetHash(hash Hash) Memo {
	m, _ := NewMemo(MemoTypeMemoReturn, hash)
	return m
}
