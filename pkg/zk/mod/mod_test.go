package zkmod

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/cronokirby/safenum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taurusgroup/cmp-ecdsa/pkg/hash"
	"github.com/taurusgroup/cmp-ecdsa/pkg/math/sample"
	"github.com/taurusgroup/cmp-ecdsa/pkg/zk"
)

func TestMod(t *testing.T) {
	p, q := zk.ProverPaillierSecret.P(), zk.ProverPaillierSecret.Q()
	sk := zk.ProverPaillierSecret
	public := Public{N: sk.PublicKey.N()}
	proof := NewProof(hash.New(), public, Private{
		P:   p,
		Q:   q,
		Phi: sk.Phi(),
	})
	out, err := proof.Marshal()
	require.NoError(t, err, "failed to marshal proof")
	proof2 := &Proof{}
	require.NoError(t, proof2.Unmarshal(out), "failed to unmarshal proof")
	out2, err := proof2.Marshal()
	require.NoError(t, err, "failed to marshal 2nd proof")
	proof3 := &Proof{}
	require.NoError(t, proof3.Unmarshal(out2), "failed to unmarshal 2nd proof")

	assert.True(t, proof3.Verify(hash.New(), public))

	proof.W = big.NewInt(0)
	for idx := range *proof.X {
		(*proof.X)[idx] = big.NewInt(0)
	}

	assert.False(t, proof.Verify(hash.New(), public), "proof should have failed")
}

func Test_set4thRoot(t *testing.T) {
	var pInt, qInt int64 = 311, 331
	p := big.NewInt(311)
	q := big.NewInt(331)
	n := big.NewInt(pInt * qInt)
	phi := big.NewInt((pInt - 1) * (qInt - 1))
	y := big.NewInt(502)
	w := sample.QNR(rand.Reader, n)

	phiNat := new(safenum.Nat).SetBig(phi, phi.BitLen())
	nMod := safenum.ModulusFromNat(new(safenum.Nat).SetBig(n, n.BitLen()))

	a, b, x := makeQuadraticResidue(y, w, n, p, q)

	xNat := new(safenum.Nat).SetBig(x, x.BitLen())

	root := fourthRoot(xNat, phiNat, nMod)

	if b {
		y.Mul(y, w)
		y.Mod(y, n)
	}
	if a {
		y.Neg(y)
		y.Mod(y, n)
	}

	assert.NotEqual(t, root, big.NewInt(1), "root cannot be 1")
	root.Exp(root, new(safenum.Nat).SetUint64(4), nMod)
	assert.Equal(t, root.Big(), y, "root^4 should be equal to y")
}
