package privacy

import (
	"crypto/subtle"
	"fmt"
	C25519 "github.com/deroproject/derosuite/crypto"
	"testing"
)

func TestScalar_Mul(t *testing.T) {

	for i:=0; i< 100; i++ {
		sk := GeneratePrivateKey(RandBytes(10))
		skScalar := new(Scalar).FromBytes(SliceToArray(sk))
		fmt.Println(skScalar.ScalarValid())
		pk := GeneratePublicKey(sk)
		pkPoint, err := new(Point).FromBytes(SliceToArray(pk))
		fmt.Println(err, pkPoint)
		a := RandomScalar()
		b := RandomScalar()
		c := RandomScalar()
		res := new(Scalar).Mul(a, b)
		res = res.Mul(res, c)

		var resPrime C25519.Key
		C25519.ScMul(&resPrime, &a.key, &b.key)
		C25519.ScMul(&resPrime, &resPrime, &c.key)
		tmp, _ := resPrime.MarshalText()
		ok := subtle.ConstantTimeCompare(res.MarshalText(), tmp) == 1
		if !ok {
			t.Fatalf("expected Scalar Mul correct !")
		}
	}

}

func TestScalar_Add(t *testing.T) {
	for i:=0; i< 100; i++ {
		a := RandomScalar()
		b := RandomScalar()
		c := RandomScalar()

		res := new(Scalar).Add(a, b)
		res = res.Add(res, c)
		res = res.Add(res,a)

		var resPrime C25519.Key
		C25519.ScAdd(&resPrime, &a.key, &b.key)
		C25519.ScAdd(&resPrime, &resPrime, &c.key)
		C25519.ScAdd(&resPrime,&resPrime, &a.key)

		tmp, _ := resPrime.MarshalText()
		ok := subtle.ConstantTimeCompare(res.MarshalText(), tmp) == 1
		if !ok {
			t.Fatalf("expected Scalar Mul correct !")
		}
	}
}

func TestScalar_Sub(t *testing.T) {
	a := new(Scalar).SetUint64(1)
	b := new(Scalar).SetUint64(2)
	c := new(Scalar).Sub(a,b)
	d := new(Scalar).Add(c,b)
	fmt.Println(a.key)
	fmt.Println(b.key)
	fmt.Println(c.key)
	fmt.Println(d.key)

	for i:=0; i< 100; i++ {
		a := RandomScalar()
		b := RandomScalar()
		c := RandomScalar()

		res := new(Scalar).Sub(a, b)
		res = res.Sub(res, c)

		var resPrime C25519.Key
		C25519.ScSub(&resPrime, &a.key, &b.key)
		C25519.ScSub(&resPrime, &resPrime, &c.key)
		tmp, _ := resPrime.MarshalText()
		ok := subtle.ConstantTimeCompare(res.MarshalText(), tmp) == 1
		if !ok {
			t.Fatalf("expected Scalar Mul correct !")
		}
	}
}
func TestScalar_Exp(t *testing.T) {
	for i:=0; i< 1; i++ {
		a := RandomScalar()
		b := uint64(15)

		res := new(Scalar).Exp(a, b)
		resPrime := new(Scalar).Mul(a,a)
		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)

		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)

		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)
		resPrime.Mul(resPrime, a)

		fmt.Println(resPrime)
		fmt.Println(res.key)
	}
}

func TestScalar_Invert(t *testing.T) {
	for i:=0; i< 100; i++ {
		a := RandomScalar()
		inv_a := new(Scalar).Invert(a)

		res := new(Scalar).Mul(a, inv_a)
		ok := res.IsOne()
		if !ok {
			t.Fatalf("expected Scalar Invert correct !")
		}
	}

	b := new(Scalar).SetUint64(1)
	bInverse := b.Invert(b)
	fmt.Printf("bInverse %v\n", bInverse)
}