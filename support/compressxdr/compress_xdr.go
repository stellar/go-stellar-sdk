package compressxdr

import (
	"io"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func NewXDREncoder(compressor Compressor, xdrPayload interface{}) XDREncoder {
	return XDREncoder{Compressor: compressor, XdrPayload: xdrPayload}
}

func NewXDRDecoder(compressor Compressor, xdrPayload interface{}) XDRDecoder {
	return XDRDecoder{Compressor: compressor, XdrPayload: xdrPayload}

}

// XDREncoder combines compression with XDR encoding
type XDREncoder struct {
	Compressor Compressor
	XdrPayload interface{}
}

// WriteTo writes the XDR compressed encoded data
func (e XDREncoder) WriteTo(w io.Writer) (int64, error) {
	zw, err := e.Compressor.NewWriter(w)
	if err != nil {
		return 0, err
	}
	defer zw.Close()

	n, err := xdr.Marshal(zw, e.XdrPayload)
	return int64(n), err
}

// XDRDecoder combines decompression with XDR decoding
type XDRDecoder struct {
	Compressor Compressor
	XdrPayload interface{}
}

// ReadFrom reads XDR compressed encoded data
func (d XDRDecoder) ReadFrom(r io.Reader) (int64, error) {
	zr, err := d.Compressor.NewReader(r)
	if err != nil {
		return 0, err
	}
	defer zr.Close()

	n, err := xdr.Unmarshal(zr, d.XdrPayload)
	return int64(n), err
}
