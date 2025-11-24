// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/stellar/go-stellar-sdk/support/storage"
)

type MockArchiveBackend struct {
	mutex sync.Mutex
	files map[string][]byte
}

func (b *MockArchiveBackend) Exists(pth string) (bool, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	_, ok := b.files[pth]
	return ok, nil
}

func (b *MockArchiveBackend) Size(pth string) (int64, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	f, ok := b.files[pth]
	sz := int64(0)
	if ok {
		sz = int64(len(f))
	}
	return sz, nil
}

func (b *MockArchiveBackend) GetFile(pth string) (io.ReadCloser, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	var buf []byte
	buf, ok := b.files[pth]
	if !ok {
		return nil, errors.New("no such file: " + pth)
	}
	return ioutil.NopCloser(bytes.NewReader(buf)), nil
}

func (b *MockArchiveBackend) PutFile(pth string, in io.ReadCloser) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	buf, e := ioutil.ReadAll(in)
	if e != nil {
		return e
	}
	b.files[pth] = buf
	return nil
}

func (b *MockArchiveBackend) ListFiles(pth string) (chan string, chan error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	ch := make(chan string)
	errs := make(chan error)
	files := make([]string, 0, len(b.files))
	for k := range b.files {
		files = append(files, k)
	}
	go func() {
		for _, f := range files {
			if strings.HasPrefix(f, pth) {
				ch <- f
			}
		}
		close(ch)
		close(errs)
	}()
	return ch, errs
}

func (b *MockArchiveBackend) CanListFiles() bool {
	return true
}

func (b *MockArchiveBackend) Close() error {
	return nil
}

func makeMockBackend() storage.Storage {
	b := new(MockArchiveBackend)
	b.files = make(map[string][]byte)
	return b
}
