package common

import (
	"errors"

	"github.com/golang/protobuf/proto"
)

//go:generate gencode go -schema=structs.schema -package=verifier

type SignAlgorithm uint8

const (
	Secp256k1 SignAlgorithm = iota
)

type SignMode bool

const (
	SavePubkey SignMode = true
	NilPubkey  SignMode = false
)

type Signature struct {
	Algorithm SignAlgorithm

	Sig    []byte
	Pubkey []byte
}

func Sign(algo SignAlgorithm, info, privkey []byte, smode SignMode) Signature {
	s := Signature{Pubkey: nil}
	s.Algorithm = algo
	switch algo {
	case Secp256k1:
		if smode {
			s.Pubkey = CalcPubkeyInSecp256k1(privkey)
		}
		s.Sig = SignInSecp256k1(info, privkey)
		return s
	}
	return s
}

func VerifySignature(info []byte, s Signature) bool {
	switch s.Algorithm {
	case Secp256k1:
		return VerifySignInSecp256k1(info, s.Pubkey, s.Sig)
	}
	return false
}

func (s *Signature) SetPubkey(pubkey []byte) {
	s.Pubkey = pubkey
}

func (s *Signature) Encode() ([]byte, error) {
	sr := &SignatureRaw{
		Algorithm: int32(s.Algorithm),
		Sig:       s.Sig,
		PubKey:    s.Pubkey,
	}
	b, err := proto.Marshal(sr)
	if err != nil {
		return nil, errors.New("fail to encode signature")
	}
	return b, nil
}

func (s *Signature) Decode(b []byte) error {
	sr := &SignatureRaw{}
	err := proto.Unmarshal(b, sr)
	if err != nil {
		return err
	}
	s.Algorithm = SignAlgorithm(sr.Algorithm)
	s.Sig = sr.Sig
	s.Pubkey = sr.PubKey
	return err
}

func (s *Signature) Hash() []byte {
	b, _ := s.Encode()
	return Sha3(b)
}
