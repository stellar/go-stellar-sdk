package historyarchive

import (
	"io"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/storage"
)

// FailingMockArchiveBackend is a mocking backend that will fail only when you
// try to read but otherwise behave like MockArchiveBackend.
type FailingMockArchiveBackend struct {
	files map[string][]byte
}

func (b *FailingMockArchiveBackend) Exists(pth string) (bool, error) {
	_, ok := b.files[pth]
	return ok, nil
}

func (b *FailingMockArchiveBackend) Size(pth string) (int64, error) {
	f, ok := b.files[pth]
	sz := int64(0)
	if ok {
		sz = int64(len(f))
	}
	return sz, nil
}

func (b *FailingMockArchiveBackend) GetFile(pth string) (io.ReadCloser, error) {
	data, ok := b.files[pth]
	if !ok {
		return nil, errors.New("file does not exist")
	}

	fr := FakeReader{}
	fr.data = make([]byte, len(data))
	copy(fr.data[:], data[:])
	return &fr, nil
}

func (b *FailingMockArchiveBackend) PutFile(pth string, in io.ReadCloser) error {
	buf, e := io.ReadAll(in)
	if e != nil {
		return e
	}
	b.files[pth] = buf
	return nil
}

func (b *FailingMockArchiveBackend) ListFiles(pth string) (chan string, chan error) {
	return nil, nil
}

func (b *FailingMockArchiveBackend) CanListFiles() bool {
	return false
}

func (b *FailingMockArchiveBackend) Close() error {
	b.files = make(map[string][]byte)
	return nil
}

func makeFailingMockBackend() storage.Storage {
	b := new(FailingMockArchiveBackend)
	b.Close()
	return b
}

type FakeReader struct {
	data []byte
}

func (fr *FakeReader) Read(b []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (fr *FakeReader) Close() error {
	return nil
}

var _ io.ReadCloser = &FakeReader{}
