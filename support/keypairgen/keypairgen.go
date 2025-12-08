package keypairgen

import "github.com/stellar/go-stellar-sdk/keypair"

// Generator generates new keys with the underlying source. The underlying
// source defaults to the RandomSource if not specified.
type Generator struct {
	Source Source
}

func (g *Generator) getSource() Source {
	if g == nil || g.Source == nil {
		return RandomSource{}
	}
	return g.Source
}

// Generate returns a new key using the underlying source.
func (g *Generator) Generate() (*keypair.Full, error) {
	return g.getSource().Generate()
}

// Source provides keys.
type Source interface {
	Generate() (*keypair.Full, error)
}

// RandomSource provides new keys that are randomly generated using the
// keypair.Random function.
type RandomSource struct{}

// Generated returns a new key using keypair.Random.
func (RandomSource) Generate() (*keypair.Full, error) {
	return keypair.Random()
}
